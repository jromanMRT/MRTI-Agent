// Fase 2: copia el SQLite del servidor de Go (run/core.db) a MySQL
// (mrti_agent_core) y lo verifica.
//
// 1. Toma una instantánea consistente con el respaldo en línea de SQLite en
//    modo solo lectura: el servicio mrti-monitor sigue escribiendo sin
//    bloquearse ni enterarse.
// 2. Inserta o actualiza cada fila por su llave, conservando los ids de
//    alertas y comandos. Repetirlo no duplica nada; se vuelve a correr justo
//    antes del corte (Fase 4) para traer lo último.
// 3. Verifica conteo y huella SHA-256 por tabla (filas canónicas ordenadas
//    por llave) entre la instantánea y MySQL.
// 4. Con --compare-go levanta una copia temporal del binario de Go sobre la
//    misma instantánea y compara todas las consultas de solo lectura contra
//    este servidor leyendo MySQL.
//
// Las filas que existan en MySQL y no en la instantánea solo se reportan; se
// borran únicamente con --prune (válido mientras Node no reciba reportes
// propios, es decir, antes del corte).
//
// Uso (desde server/):
//   node scripts/import-sqlite.mjs [--source run/core.db] [--dry-run] [--prune] [--compare-go]
import { execFileSync, spawn } from 'node:child_process';
import { createHash } from 'node:crypto';
import { mkdtempSync, rmSync } from 'node:fs';
import http from 'node:http';
import { tmpdir } from 'node:os';
import path from 'node:path';
import dotenv from 'dotenv';

dotenv.config({ path: process.env.DOTENV_CONFIG_PATH || '.env' });
const { pool } = await import('../src/db.js');

const args = process.argv.slice(2);
const flag = (name) => args.includes(name);
const option = (name, fallback) => { const i = args.indexOf(name); return i >= 0 ? args[i + 1] : fallback; };
const SOURCE = option('--source', '/var/www/mrt/MRTI/run/core.db');
const GO_BINARY = option('--go-binary', '/var/www/mrt/MRTI/bin/mrti-monitor');
const DRY_RUN = flag('--dry-run');

// Columnas por tabla en el orden de store.go; los BLOB se leen como texto.
const TABLES = {
  agents: { key: ['id'], columns: ['id', 'hostname', 'name', 'os', 'arch', 'version', 'tags', 'first_seen', 'last_seen', 'last_heartbeat', 'self_cpu', 'self_mem', 'sequence', 'archived'] },
  latest: { key: ['agent_id'], columns: ['agent_id', 'sequence', 'ts', 'envelope'], blobs: ['envelope'] },
  module_data: { key: ['agent_id', 'module'], columns: ['agent_id', 'module', 'ts', 'data'], blobs: ['data'] },
  alerts: { key: ['id'], columns: ['id', 'agent_id', 'hostname', 'rule', 'severity', 'resource', 'message', 'value', 'threshold', 'ts'] },
  commands: { key: ['id'], columns: ['id', 'agent_id', 'cmd_id', 'type', 'payload', 'created', 'delivered', 'result', 'result_ok', 'result_ts'], blobs: ['payload', 'result'] },
};

const workdir = mkdtempSync(path.join(tmpdir(), 'mrti-agent-import-'));
const snapshot = path.join(workdir, 'snapshot.db');

function sqliteRows(table) {
  const { columns, blobs = [], key } = TABLES[table];
  const select = columns.map((column) => (blobs.includes(column) ? `CAST(${column} AS TEXT) AS ${column}` : column)).join(', ');
  const out = execFileSync('sqlite3', ['-readonly', '-json', snapshot, `SELECT ${select} FROM ${table} ORDER BY ${key.join(', ')}`], { maxBuffer: 1 << 30 }).toString().trim();
  return out ? JSON.parse(out) : [];
}

// Forma canónica para la huella: mismo tratamiento de NULL y números en
// ambos lados (SQLite y MySQL devuelven DOUBLE/INTEGER como número).
function canonical(table, row) {
  return JSON.stringify(TABLES[table].columns.map((column) => {
    const value = row[column];
    if (value === null || value === undefined) return null;
    if (typeof value === 'bigint') return Number(value);
    if (typeof value === 'number') return value;
    if (column === 'archived' || column === 'delivered' || column === 'result_ok') return Number(value);
    return String(value);
  }));
}
const keyOf = (table, row) => TABLES[table].key.map((column) => String(row[column])).join('\u0000');
const digest = (table, rows) => createHash('sha256').update(rows.map((row) => canonical(table, row)).join('\n')).digest('hex');

async function mysqlRows(table) {
  const { columns, key } = TABLES[table];
  const [rows] = await pool.query(`SELECT ${columns.join(', ')} FROM ${table} ORDER BY ${key.join(', ')}`);
  return rows;
}

const report = { source: SOURCE, dryRun: DRY_RUN, tables: {} };
let failed = false;
try {
  // 1. Instantánea consistente (respaldo en línea, solo lectura sobre el origen).
  execFileSync('sqlite3', ['-readonly', `file:${SOURCE}?mode=ro`, `.backup '${snapshot.replaceAll("'", "''")}'`]);
  const source = Object.fromEntries(Object.keys(TABLES).map((table) => [table, sqliteRows(table)]));

  // 2. Copia en una sola transacción.
  if (!DRY_RUN) {
    const connection = await pool.getConnection();
    try {
      await connection.beginTransaction();
      for (const [table, { columns, key }] of Object.entries(TABLES)) {
        const updates = columns.filter((column) => !key.includes(column)).map((column) => `${column} = incoming.${column}`).join(', ');
        for (let i = 0; i < source[table].length; i += 200) {
          const batch = source[table].slice(i, i + 200);
          await connection.query(`INSERT INTO ${table} (${columns.join(', ')}) VALUES ${batch.map(() => `(${columns.map(() => '?').join(', ')})`).join(', ')}
            AS incoming ON DUPLICATE KEY UPDATE ${updates}`, batch.flatMap((row) => columns.map((column) => row[column] ?? null)));
        }
      }
      if (flag('--prune')) {
        for (const table of Object.keys(TABLES)) {
          const wanted = new Set(source[table].map((row) => keyOf(table, row)));
          const extras = (await mysqlRows(table)).filter((row) => !wanted.has(keyOf(table, row)));
          for (const row of extras) {
            const { key } = TABLES[table];
            await connection.query(`DELETE FROM ${table} WHERE ${key.map((column) => `${column} = ?`).join(' AND ')}`, key.map((column) => row[column]));
          }
          report.tables[table] = { ...(report.tables[table] || {}), pruned: extras.length };
        }
      }
      await connection.commit();
    } catch (error) {
      await connection.rollback().catch(() => {});
      throw error;
    } finally { connection.release(); }
  }

  // 3. Conteos y huellas.
  for (const table of Object.keys(TABLES)) {
    const target = await mysqlRows(table);
    const sourceKeys = new Set(source[table].map((row) => keyOf(table, row)));
    const targetByKey = new Map(target.map((row) => [keyOf(table, row), row]));
    const matching = target.filter((row) => sourceKeys.has(keyOf(table, row)));
    const missing = source[table].filter((row) => !targetByKey.has(keyOf(table, row))).length;
    const different = source[table].filter((row) => targetByKey.has(keyOf(table, row)) && canonical(table, row) !== canonical(table, targetByKey.get(keyOf(table, row)))).length;
    const entry = {
      ...(report.tables[table] || {}),
      sqlite: source[table].length, mysql: target.length, extrasInMysql: target.length - matching.length, missing, different,
      sha256: digest(table, source[table]) === digest(table, matching) ? 'igual' : 'DISTINTA',
    };
    report.tables[table] = entry;
    if (!DRY_RUN && (missing || different || entry.sha256 !== 'igual')) failed = true;
  }

  // 4. Mismas consultas contra Go (sobre la instantánea) y Node (sobre MySQL).
  if (flag('--compare-go') && !DRY_RUN) report.apiComparison = await compareWithGo(source);
  if (report.apiComparison && report.apiComparison.failed.length) failed = true;
} finally {
  await pool.end();
  rmSync(workdir, { recursive: true, force: true });
}

console.log(JSON.stringify(report, null, 2));
console.log(failed ? '\n✗ La copia NO coincide con el origen.' : DRY_RUN ? '\nSimulación: no se escribió nada.' : '\n✓ La copia coincide con el origen.');
process.exitCode = failed ? 1 : 0;

async function compareWithGo(source) {
  const { createApp } = await import('../src/app.js');
  const { createStore } = await import('../src/store.js');
  const fakeCore = http.createServer((_req, res) => { res.statusCode = 204; res.end(); });
  await new Promise((resolve) => fakeCore.listen(0, '127.0.0.1', resolve));
  const corePort = fakeCore.address().port;
  const goPort = 20000 + Math.floor(Math.random() * 20000);
  // Go necesita poder escribir su propio SQLite: trabaja sobre otra copia.
  const goDb = path.join(workdir, 'go.db');
  execFileSync('sqlite3', [snapshot, `.backup '${goDb.replaceAll("'", "''")}'`]);
  const go = spawn(GO_BINARY, ['-addr', `127.0.0.1:${goPort}`, '-db', goDb, '-api-key', 'compare-only', '-foreground', '-downloads-dir', workdir], {
    env: { ...process.env, MRTI_AUTH_MODULE_URL: `http://127.0.0.1:${corePort}/` }, stdio: 'ignore',
  });
  const allow = (_req, _res, next) => next();
  const node = http.createServer(createApp({ store: createStore(pool), apiKey: '', portalAccess: allow }));
  await new Promise((resolve) => node.listen(0, '127.0.0.1', resolve));
  const nodePort = node.address().port;
  try {
    for (let i = 0; i < 50; i += 1) {
      try { if ((await fetch(`http://127.0.0.1:${goPort}/healthz`)).ok) break; } catch { /* arrancando */ }
      await new Promise((resolve) => setTimeout(resolve, 100));
    }
    // Solo lecturas (GET de la cola de comandos no: marca entregados).
    const paths = ['/api/v1/agents', `/api/v1/alerts?limit=100000`, '/api/v1/export', '/metrics'];
    for (const agent of source.agents) {
      paths.push(`/api/v1/agents/${agent.id}`);
      for (const row of source.module_data.filter((item) => item.agent_id === agent.id)) paths.push(`/api/v1/agents/${agent.id}/modules/${row.module}`);
    }
    // Diferencia conocida: store.go ignora el error de rows.Scan, así que en
    // una fila con NULL (p. ej. un agente que nunca mandó heartbeat) Go pone
    // en cero esa columna y todas las siguientes. Node devuelve los valores
    // reales. Esas filas se apartan de la comparación estricta y se reportan.
    const scanned = {
      agents: ['hostname', 'name', 'os', 'arch', 'version', 'first_seen', 'last_seen', 'last_heartbeat', 'self_cpu', 'self_mem', 'sequence', 'archived'],
      alerts: ['agent_id', 'hostname', 'rule', 'severity', 'resource', 'message', 'value', 'threshold', 'ts'],
    };
    const nullAgents = new Set(source.agents.filter((row) => scanned.agents.some((column) => row[column] === null)).map((row) => row.id));
    const nullAlerts = new Set(source.alerts.filter((row) => scanned.alerts.some((column) => row[column] === null)).map((row) => row.id));
    const withoutKnown = (url, text) => {
      if (!nullAgents.size && !nullAlerts.size) return text;
      if (url === '/metrics') return text.split('\n').filter((line) => ![...nullAgents].some((id) => line.includes(`agent="${id}"`))).join('\n');
      let body; try { body = JSON.parse(text); } catch { return text; }
      if (url === '/api/v1/agents') body = (body || []).filter((agent) => !nullAgents.has(agent.id));
      else if (url.startsWith('/api/v1/alerts')) body = (body || []).filter((alert) => !nullAlerts.has(alert.id));
      else if (url === '/api/v1/export') {
        body.agents = body.agents.filter((item) => !nullAgents.has(item.agent.id));
        body.alerts = (body.alerts || []).filter((alert) => !nullAlerts.has(alert.id));
        body.agent_count = '<n>';
      }
      return JSON.stringify(body);
    };
    const sorted = (value) => Array.isArray(value) ? value.map(sorted) : value && typeof value === 'object'
      ? Object.fromEntries(Object.entries(value).sort(([a], [b]) => a.localeCompare(b)).map(([k, v]) => [k, k === 'exported_at' ? '<t>' : sorted(v)])) : value;
    const same = (a, b) => { try { return JSON.stringify(sorted(JSON.parse(a))) === JSON.stringify(sorted(JSON.parse(b))); } catch { return a === b; } };
    const results = [];
    for (const url of paths) {
      const [g, n] = await Promise.all([goPort, nodePort].map(async (port) => { const r = await fetch(`http://127.0.0.1:${port}${url}`); return { status: r.status, text: await r.text() }; }));
      results.push({ url, ok: g.status === n.status && same(withoutKnown(url, g.text), withoutKnown(url, n.text)), go: g.status, node: n.status, bytes: g.text.length });
    }
    return { knownGoNullDifferences: { agents: [...nullAgents], alerts: [...nullAlerts] }, cases: results.length, identical: results.filter((r) => r.ok).length, failed: results.filter((r) => !r.ok).map((r) => `${r.url} (Go ${r.go} / Node ${r.node})`) };
  } finally {
    go.kill(); node.close(); fakeCore.close();
  }
}
