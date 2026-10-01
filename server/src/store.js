// Persistencia de Agent Core en MySQL: equivalente a cmd/mrti-monitor/store.go
// (último estado por agente, último dato por módulo, alertas recientes y cola
// de comandos), con las mismas reglas de negocio.
import { encodeCommandResult, encodeEnvelope, encodeTags } from './goCompat.js';

// Un agente está "en línea" si reportó en los últimos 120 s (onlineWindow).
export const ONLINE_WINDOW_SECONDS = 120;
const nowSeconds = () => Math.floor(Date.now() / 1000);

export class AgentOnlineError extends Error {
  constructor() { super('online agents cannot be deleted'); }
}

export function createStore(pool) {
  async function touchAgent(db, { id, hostname, name, os, arch, version, tags }, now) {
    // Igual que Go: un heartbeat (sin tags) deja tags en '' y el nombre solo
    // se reemplaza si el nuevo no viene vacío.
    await db.query(`INSERT INTO agents (id, hostname, name, os, arch, version, tags, first_seen, last_seen)
      VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?) AS incoming
      ON DUPLICATE KEY UPDATE hostname = incoming.hostname, name = COALESCE(NULLIF(incoming.name, ''), agents.name),
        os = incoming.os, arch = incoming.arch, version = incoming.version, tags = incoming.tags, last_seen = incoming.last_seen`,
    [id, hostname, name, os, arch, version, tags, now, now]);
  }

  return {
    async ingestHeartbeat(hb) {
      const now = nowSeconds();
      await touchAgent(pool, { id: hb.agent_id, hostname: hb.hostname, name: '', os: hb.os, arch: hb.arch, version: hb.version, tags: '' }, now);
      await pool.query('UPDATE agents SET last_heartbeat = ?, self_cpu = ?, self_mem = ? WHERE id = ?', [now, hb.self_cpu_percent, hb.self_mem_mb, hb.agent_id]);
    },

    async ingestEnvelope(env, unixOf) {
      const now = nowSeconds();
      const connection = await pool.getConnection();
      try {
        await connection.beginTransaction();
        await touchAgent(connection, { id: env.agent_id, hostname: env.hostname, name: '', os: env.os, arch: env.arch, version: env.version, tags: encodeTags(env.tags) }, now);
        await connection.query('UPDATE agents SET sequence = ? WHERE id = ?', [env.sequence, env.agent_id]);
        await connection.query(`INSERT INTO latest (agent_id, sequence, ts, envelope) VALUES (?, ?, ?, ?) AS incoming
          ON DUPLICATE KEY UPDATE sequence = incoming.sequence, ts = incoming.ts, envelope = incoming.envelope`,
        [env.agent_id, env.sequence, now, encodeEnvelope(env)]);
        for (const result of env.results) {
          if (result.data === null) continue;
          const data = JSON.stringify(result.data);
          await connection.query(`INSERT INTO module_data (agent_id, module, ts, data) VALUES (?, ?, ?, ?) AS incoming
            ON DUPLICATE KEY UPDATE ts = incoming.ts, data = incoming.data`,
          [env.agent_id, result.module, unixOf(result.collected_at), data]);
          if (result.module === 'alerts' && Array.isArray(result.data)) {
            for (const alert of result.data) {
              if (alert === null || typeof alert !== 'object') continue;
              await connection.query(`INSERT INTO alerts (agent_id, hostname, rule, severity, resource, message, value, threshold, ts)
                VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, [env.agent_id, env.hostname, alert.rule ?? '', alert.severity ?? '', alert.resource ?? '',
                alert.message ?? '', Number(alert.value) || 0, Number(alert.threshold) || 0, now]);
            }
          }
        }
        await connection.commit();
      } catch (error) {
        await connection.rollback().catch(() => {});
        throw error;
      } finally { connection.release(); }
    },

    async saveCommandResult(result) {
      await pool.query('UPDATE commands SET result = ?, result_ok = ?, result_ts = ? WHERE cmd_id = ? AND agent_id = ?',
        [encodeCommandResult(result), result.ok ? 1 : 0, nowSeconds(), result.command_id, result.agent_id]);
    },

    async enqueueCommand(agentId, command) {
      await pool.query('INSERT INTO commands (agent_id, cmd_id, type, payload, created) VALUES (?, ?, ?, ?, ?)',
        [agentId, command.id, command.type, command.payload === undefined ? null : JSON.stringify(command.payload), nowSeconds()]);
    },

    // Devuelve y marca entregados los comandos pendientes. A diferencia de Go,
    // en una transacción con bloqueo: dos lecturas simultáneas no entregan
    // el mismo comando dos veces.
    async takeCommands(agentId) {
      const connection = await pool.getConnection();
      try {
        await connection.beginTransaction();
        const [rows] = await connection.query('SELECT id, cmd_id, type, payload FROM commands WHERE agent_id = ? AND delivered = 0 ORDER BY id ASC FOR UPDATE', [agentId]);
        if (rows.length) await connection.query(`UPDATE commands SET delivered = 1 WHERE id IN (${rows.map(() => '?').join(',')})`, rows.map((row) => row.id));
        await connection.commit();
        // model.Command: issued_at no se guarda en la cola, así que Go lo
        // entrega con el valor cero de time.Time.
        return rows.map((row) => {
          const command = { id: row.cmd_id ?? '', type: row.type ?? '' };
          if (row.payload !== null && row.payload !== '') command.payload = JSON.parse(row.payload);
          command.issued_at = '0001-01-01T00:00:00Z';
          return command;
        });
      } catch (error) {
        await connection.rollback().catch(() => {});
        throw error;
      } finally { connection.release(); }
    },

    async listAgents() {
      const [rows] = await pool.query(`SELECT id, hostname, name, os, arch, version, tags, first_seen, last_seen, last_heartbeat,
        self_cpu, self_mem, sequence, archived FROM agents ORDER BY hostname`);
      const now = nowSeconds();
      return rows.map((row) => {
        const agent = { id: row.id, hostname: row.hostname ?? '' };
        if (row.name) agent.name = row.name;
        Object.assign(agent, { os: row.os ?? '', arch: row.arch ?? '', version: row.version ?? '' });
        if (row.tags && row.tags !== 'null') agent.tags = JSON.parse(row.tags);
        Object.assign(agent, {
          first_seen: Number(row.first_seen ?? 0), last_seen: Number(row.last_seen ?? 0), last_heartbeat: Number(row.last_heartbeat ?? 0),
          self_cpu_percent: Number(row.self_cpu ?? 0), self_mem_mb: Number(row.self_mem ?? 0), sequence: Number(row.sequence ?? 0),
          online: now - Number(row.last_seen ?? 0) <= ONLINE_WINDOW_SECONDS, archived: Boolean(row.archived),
        });
        return agent;
      });
    },

    async setAgentArchived(agentId, archived) {
      const [result] = await pool.query('UPDATE agents SET archived = ? WHERE id = ?', [archived ? 1 : 0, agentId]);
      return result.affectedRows > 0;
    },

    async deleteAgent(agentId) {
      const [[agent]] = await pool.query('SELECT last_seen FROM agents WHERE id = ?', [agentId]);
      if (!agent) return false;
      if (nowSeconds() - Number(agent.last_seen ?? 0) <= ONLINE_WINDOW_SECONDS) throw new AgentOnlineError();
      const connection = await pool.getConnection();
      try {
        await connection.beginTransaction();
        for (const table of ['latest', 'module_data', 'alerts', 'commands']) await connection.query(`DELETE FROM ${table} WHERE agent_id = ?`, [agentId]);
        await connection.query('DELETE FROM agents WHERE id = ?', [agentId]);
        await connection.commit();
        return true;
      } catch (error) {
        await connection.rollback().catch(() => {});
        throw error;
      } finally { connection.release(); }
    },

    async getEnvelope(agentId) {
      const [[row]] = await pool.query('SELECT envelope FROM latest WHERE agent_id = ?', [agentId]);
      return row ? row.envelope : null;
    },

    async getModule(agentId, module) {
      const [[row]] = await pool.query('SELECT data FROM module_data WHERE agent_id = ? AND module = ?', [agentId, module]);
      return row ? row.data : null;
    },

    async recentAlerts(limit) {
      const [rows] = await pool.query(`SELECT id, agent_id, hostname, rule, severity, resource, message, value, threshold, ts
        FROM alerts ORDER BY id DESC LIMIT ?`, [limit]);
      return rows.map((row) => ({
        id: Number(row.id), agent_id: row.agent_id ?? '', hostname: row.hostname ?? '', rule: row.rule ?? '', severity: row.severity ?? '',
        resource: row.resource ?? '', message: row.message ?? '', value: Number(row.value ?? 0), threshold: Number(row.threshold ?? 0), timestamp: Number(row.ts ?? 0),
      }));
    },

    async modulesFor(agentId) {
      const [rows] = await pool.query('SELECT module, data FROM module_data WHERE agent_id = ?', [agentId]);
      return Object.fromEntries(rows.map((row) => [row.module, JSON.parse(row.data)]));
    },
  };
}
