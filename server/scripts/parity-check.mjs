// Prueba de paridad Fase 1: manda el mismo tráfico al binario de Go
// (referencia) y a este servidor Node, y compara código HTTP y cuerpo de
// cada respuesta. Nada toca el servicio real:
// - Go corre como copia temporal en otro puerto, con un SQLite desechable.
// - Node usa la base indicada en DOTENV_CONFIG_PATH (por defecto .env.test),
//   cuyas tablas se vacían antes y después.
// - La sesión la responde un Core simulado (Bearer good → 204).
// - El reporte de muestra es el último real, leído del SQLite en vivo en
//   modo solo lectura (MRTI_PARITY_LIVE_DB).
//
// Uso: DOTENV_CONFIG_PATH=.env.test npm run parity
import { execFileSync, spawn } from 'node:child_process';
import { mkdtempSync, rmSync } from 'node:fs';
import http from 'node:http';
import { tmpdir } from 'node:os';
import path from 'node:path';
import { gzipSync } from 'node:zlib';
import dotenv from 'dotenv';

dotenv.config({ path: process.env.DOTENV_CONFIG_PATH || '.env.test' });
const { pool } = await import('../src/db.js');
const { createStore } = await import('../src/store.js');
const { createApp } = await import('../src/app.js');
const { createCoreAuth } = await import('../src/vendor/mrti-platform/core-auth.js');

const GO_BINARY = process.env.MRTI_PARITY_GO_BINARY || '/var/www/mrt/MRTI/bin/mrti-monitor';
const LIVE_DB = process.env.MRTI_PARITY_LIVE_DB || '/var/www/mrt/MRTI/run/core.db';
const KEY = 'parity-key';
const TABLES = ['commands', 'alerts', 'module_data', 'latest', 'agents'];

const listen = (server) => new Promise((resolve) => server.listen(0, '127.0.0.1', () => resolve(server.address().port)));
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

// Core simulado: el mismo contrato que /api/auth/module-access/:code.
let coreDown = false;
const fakeCore = http.createServer((req, res) => {
  if (coreDown) { res.statusCode = 502; return res.end(); }
  const auth = req.headers.authorization || '';
  res.statusCode = auth === 'Bearer good' ? 204 : auth === 'Bearer nomodule' ? 403 : 401;
  res.end();
});
const corePort = await listen(fakeCore);

// Último reporte real (solo lectura).
const liveEnvelope = execFileSync('sqlite3', ['-readonly', `file:${LIVE_DB}?mode=ro`, 'SELECT CAST(envelope AS TEXT) FROM latest ORDER BY ts DESC LIMIT 1'], { maxBuffer: 64 << 20 }).toString().trim();
if (!liveEnvelope) throw new Error('No hay reportes en el SQLite en vivo para usar como muestra');
const sample = JSON.parse(liveEnvelope);
const agentId = sample.agent_id;

// Go de referencia.
const workdir = mkdtempSync(path.join(tmpdir(), 'mrti-parity-'));
const goPort = 20000 + Math.floor(Math.random() * 20000);
const go = spawn(GO_BINARY, ['-addr', `127.0.0.1:${goPort}`, '-db', path.join(workdir, 'ref.db'), '-api-key', KEY, '-foreground', '-downloads-dir', workdir], {
  env: { ...process.env, MRTI_AUTH_MODULE_URL: `http://127.0.0.1:${corePort}/api/auth/module-access/agent-core` }, stdio: 'ignore',
});

// Node bajo prueba.
await pool.query('SET FOREIGN_KEY_CHECKS = 0');
for (const table of TABLES) await pool.query(`DELETE FROM ${table}`);
for (const table of ['alerts', 'commands']) await pool.query(`ALTER TABLE ${table} AUTO_INCREMENT = 1`);
const auth = createCoreAuth({ service: 'parity', moduleCode: 'agent-core', coreUrl: `http://127.0.0.1:${corePort}` });
const nodeServer = http.createServer(createApp({ store: createStore(pool), apiKey: KEY, portalAccess: auth.moduleAccessRequired }));
const nodePort = await listen(nodeServer);

for (let i = 0; i < 50; i += 1) {
  try { const r = await fetch(`http://127.0.0.1:${goPort}/healthz`); if (r.ok) break; } catch { /* arrancando */ }
  await sleep(100);
}

async function call(port, { method = 'GET', path: url, headers = {}, body }) {
  const response = await fetch(`http://127.0.0.1:${port}${url}`, { method, headers, body });
  const text = await response.text();
  return { status: response.status, text, type: (response.headers.get('content-type') || '').split(';')[0] };
}

// Normaliza lo que depende del reloj (segundos Unix) para comparar.
const TIME_FIELDS = new Set(['first_seen', 'last_seen', 'last_heartbeat', 'timestamp', 'exported_at']);
function normalize(value) {
  if (Array.isArray(value)) return value.map(normalize);
  if (value && typeof value === 'object') return Object.fromEntries(Object.entries(value).sort(([a], [b]) => a.localeCompare(b)).map(([k, v]) => [k, TIME_FIELDS.has(k) ? '<t>' : normalize(v)]));
  return value;
}
function sameBody(a, b, kind) {
  if (kind === 'text') return a.trim() === b.trim();
  try { return JSON.stringify(normalize(JSON.parse(a))) === JSON.stringify(normalize(JSON.parse(b))); } catch { return a.trim() === b.trim(); }
}

const results = [];
async function check(name, request, { kind = 'json', compareBody = true } = {}) {
  const [g, n] = [await call(goPort, request), await call(nodePort, request)];
  const statusOk = g.status === n.status;
  const bodyOk = !compareBody || sameBody(g.text, n.text, kind);
  results.push({ name, ok: statusOk && bodyOk, go: g.status, node: n.status, bytes: g.text.length });
  if (!(statusOk && bodyOk)) console.log(`✗ ${name}\n  Go   ${g.status} ${g.text.slice(0, 300)}\n  Node ${n.status} ${n.text.slice(0, 300)}`);
  return { g, n };
}

const good = { Authorization: 'Bearer good' };
const ingest = (kind, body, extra = {}) => ({ method: 'POST', path: '/api/v1/ingest', headers: { 'X-MRTI-API-Key': KEY, 'X-MRTI-Kind': kind, 'Content-Type': 'application/json', ...extra }, body });
const heartbeat = JSON.stringify({ schema: 'mrti.v1', agent_id: agentId, hostname: sample.hostname, version: sample.version, os: sample.os, arch: sample.arch, uptime_seconds: 3600, active_modules: ['cpu', 'ram'], self_cpu_percent: 0.42, self_mem_mb: 21.5, timestamp: sample.timestamp });

try {
  await check('healthz', { path: '/healthz' }, { kind: 'text' });
  await check('lista vacía', { path: '/api/v1/agents', headers: good });
  await check('ingest sin llave', { ...ingest('envelope', liveEnvelope), headers: { 'X-MRTI-Kind': 'envelope' } }, { kind: 'text' });
  await check('ingest llave incorrecta', ingest('envelope', liveEnvelope, { 'X-MRTI-API-Key': 'nope' }), { kind: 'text' });
  await check('heartbeat', ingest('heartbeat', heartbeat));
  await check('reporte real', ingest('envelope', liveEnvelope));
  await check('reporte real en gzip', ingest('envelope', gzipSync(liveEnvelope), { 'Content-Encoding': 'gzip' }));
  await check('JSON inválido se acepta y se ignora', ingest('envelope', '{no es json'));
  await check('gzip corrupto', ingest('envelope', 'no-gzip', { 'Content-Encoding': 'gzip' }), { compareBody: false });
  await check('lista de agentes', { path: '/api/v1/agents', headers: good });
  await check('reporte completo del agente', { path: `/api/v1/agents/${agentId}`, headers: good });
  await check('agente inexistente', { path: '/api/v1/agents/no-existe', headers: good }, { kind: 'text' });
  for (const module of sample.results.filter((r) => r.data !== undefined && r.data !== null).map((r) => r.module)) {
    await check(`módulo ${module}`, { path: `/api/v1/agents/${agentId}/modules/${module}`, headers: good });
  }
  await check('módulo inexistente', { path: `/api/v1/agents/${agentId}/modules/nada`, headers: good }, { kind: 'text' });
  const withAlerts = JSON.stringify({ ...sample, sequence: sample.sequence + 1, results: [
    { module: 'alerts', collected_at: sample.timestamp, duration_ms: 1, data: [
      { rule: 'disk_high', severity: 'warning', resource: '/', message: 'Disco <95%> & "lleno"', value: 91.5, threshold: 90 },
      { rule: 'cpu_high', severity: 'critical', resource: 'cpu', message: 'CPU alta', value: 99, threshold: 95 },
    ] },
    { module: 'ram', collected_at: sample.timestamp, duration_ms: 1, data: { used_percent: 0.00001234, swap_used_percent: 1234567.5 } },
    { module: 'temperature', collected_at: sample.timestamp, duration_ms: 1, error: 'sin sensores' },
  ] });
  await check('reporte con alertas', ingest('envelope', withAlerts));
  await check('heartbeat con valores grandes', ingest('heartbeat', JSON.stringify({ ...JSON.parse(heartbeat), self_cpu_percent: 1e-7, self_mem_mb: 1234567.25 })));
  await check('reporte tras alertas', { path: `/api/v1/agents/${agentId}`, headers: good });
  const extreme = await check('métricas con valores extremos', { path: '/metrics' }, { kind: 'text' });
  if (process.env.PARITY_VERBOSE) console.log(extreme.g.text.split('\n').filter((line) => /e[+-]\d\d$/.test(line)).map((line) => `Go: ${line.replace(/agent="[^"]+"/, 'agent="…"')}`).join('\n'));
  await check('alertas', { path: '/api/v1/alerts?limit=50', headers: good });
  await check('alertas límite 1', { path: '/api/v1/alerts?limit=1', headers: good });
  await check('alertas límite inválido', { path: '/api/v1/alerts?limit=-3', headers: good });
  await check('exportación', { path: '/api/v1/export', headers: good });
  const metrics = await check('métricas Prometheus', { path: '/metrics' }, { kind: 'text' });
  if (process.env.PARITY_VERBOSE) console.log(metrics.g.text.split('\n').filter((line) => line && !line.startsWith('#')).slice(0, 8).join('\n'));
  await check('sin comandos → 204', { path: `/api/v1/agents/${agentId}/commands` }, { compareBody: false });
  await check('comando sin tipo', { method: 'POST', path: `/api/v1/agents/${agentId}/commands`, headers: { ...good, 'Content-Type': 'application/json' }, body: '{"payload":{}}' }, { kind: 'text' });
  await check('encolar comando', { method: 'POST', path: `/api/v1/agents/${agentId}/commands`, headers: { ...good, 'Content-Type': 'application/json' }, body: JSON.stringify({ id: 'cmd-paridad-1', type: 'enable_module', payload: { module: 'snmp' } }) });
  await check('encolar comando sin payload', { method: 'POST', path: `/api/v1/agents/${agentId}/commands`, headers: { ...good, 'Content-Type': 'application/json' }, body: JSON.stringify({ id: 'cmd-paridad-2', type: 'ping' }) });
  await check('entregar comandos', { path: `/api/v1/agents/${agentId}/commands` });
  await check('ya entregados → 204', { path: `/api/v1/agents/${agentId}/commands` }, { compareBody: false });
  await check('resultado de comando', ingest('command_result', JSON.stringify({ command_id: 'cmd-paridad-1', agent_id: agentId, ok: true, output: 'listo', finished_at: sample.timestamp })));
  await check('archivar sin llave', { method: 'PATCH', path: `/api/v1/agents/${agentId}`, body: '{"archived":true}' });
  await check('archivar sin campo', { method: 'PATCH', path: `/api/v1/agents/${agentId}`, headers: { 'X-MRTI-API-Key': KEY }, body: '{}' });
  await check('archivar', { method: 'PATCH', path: `/api/v1/agents/${agentId}`, headers: { 'X-MRTI-API-Key': KEY }, body: '{"archived":true}' });
  await check('archivar inexistente', { method: 'PATCH', path: '/api/v1/agents/no-existe', headers: { 'X-MRTI-API-Key': KEY }, body: '{"archived":false}' });
  await check('lista tras archivar', { path: '/api/v1/agents', headers: good });
  await check('borrar agente en línea → 409', { method: 'DELETE', path: `/api/v1/agents/${agentId}`, headers: { 'X-MRTI-API-Key': KEY } });
  await check('borrar inexistente', { method: 'DELETE', path: '/api/v1/agents/no-existe', headers: { 'X-MRTI-API-Key': KEY } });
  // Sesión: mismos códigos que Go (los cuerpos difieren a propósito: Node usa los mensajes del kit).
  await check('sin sesión → 401', { path: '/api/v1/agents' }, { compareBody: false });
  await check('sesión inválida → 401', { path: '/api/v1/agents', headers: { Authorization: 'Bearer malo' } }, { compareBody: false });
  await check('sin acceso al módulo → 403', { path: '/api/v1/agents', headers: { Authorization: 'Bearer nomodule' } }, { compareBody: false });
  coreDown = true;
  await check('Core caído → 503', { path: '/api/v1/agents', headers: good }, { compareBody: false });
  coreDown = false;
} finally {
  go.kill();
  for (const table of TABLES) await pool.query(`DELETE FROM ${table}`);
  await pool.end();
  nodeServer.close(); fakeCore.close();
  rmSync(workdir, { recursive: true, force: true });
}

if (process.env.PARITY_VERBOSE) for (const r of results) console.log(`${r.ok ? '✓' : '✗'} Go ${r.go} · Node ${r.node} · ${String(r.bytes).padStart(7)} B  ${r.name}`);
const failed = results.filter((r) => !r.ok);
console.log(`\nParidad Go ↔ Node: ${results.length - failed.length}/${results.length} casos idénticos${failed.length ? ` — fallan: ${failed.map((r) => r.name).join(', ')}` : ''}`);
process.exitCode = failed.length ? 1 : 0;
