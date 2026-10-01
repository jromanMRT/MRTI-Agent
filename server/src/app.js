// API de Agent Core en Node: mismas rutas, códigos y cuerpos que
// cmd/mrti-monitor (ver docs/CORE-API.md). Fase 1 de la migración: corre en
// paralelo al servidor de Go sin recibir tráfico de agentes todavía.
import { timingSafeEqual } from 'node:crypto';
import { gunzipSync } from 'node:zlib';
import express from 'express';
import { decodeCommandResult, decodeEnvelope, decodeHeartbeat, goFormatG, goQuote, goUnixSeconds } from './goCompat.js';
import { AgentOnlineError } from './store.js';

const MAX_BODY_BYTES = 16 << 20;

function sameKey(provided, expected) {
  const a = Buffer.from(String(provided || ''));
  const b = Buffer.from(String(expected));
  return a.length === b.length && timingSafeEqual(a, b);
}

// http.Error de Go: texto plano con salto de línea final.
const goError = (res, status, text) => res.status(status).type('text/plain; charset=utf-8').set('X-Content-Type-Options', 'nosniff').send(`${text}\n`);
// Go serializa los slices nil como null: la lista vacía sale "null".
const nullIfEmpty = (list) => (list.length ? list : null);
const goJson = (res, status, value) => res.status(status).type('application/json').send(JSON.stringify(Array.isArray(value) && !value.length ? null : value));

function rawBody(req, res, next) {
  const chunks = [];
  let size = 0;
  req.on('data', (chunk) => {
    size += chunk.length;
    if (size > MAX_BODY_BYTES) { req.destroy(); return; }
    chunks.push(chunk);
  });
  req.on('end', () => {
    try {
      let body = Buffer.concat(chunks);
      if (req.headers['content-encoding'] === 'gzip') body = gunzipSync(body, { maxOutputLength: MAX_BODY_BYTES });
      req.rawBody = body.toString('utf8');
      next();
    } catch (error) { goError(res, 400, error.message); }
  });
  req.on('error', next);
}

export function metricsText(agents, modulesByAgent) {
  const lines = [];
  const help = (name, type, text) => lines.push(`# HELP ${name} ${text}`, `# TYPE ${name} ${type}`);
  help('mrti_agent_up', 'gauge', '1 if the agent is online (seen recently)');
  help('mrti_agent_self_mem_mb', 'gauge', 'Agent process RSS in MB');
  help('mrti_agent_self_cpu_percent', 'gauge', 'Agent process CPU percent');
  help('mrti_cpu_usage_percent', 'gauge', 'Host CPU usage percent');
  help('mrti_mem_used_percent', 'gauge', 'Host RAM used percent');
  help('mrti_swap_used_percent', 'gauge', 'Host swap used percent');
  help('mrti_disk_used_percent', 'gauge', 'Filesystem used percent');
  help('mrti_temperature_celsius', 'gauge', 'Hottest sensor temperature');
  help('mrti_ups_battery_percent', 'gauge', 'UPS battery charge percent');
  help('mrti_docker_containers_running', 'gauge', 'Running docker containers');
  help('mrti_services_failed', 'gauge', 'Failed services');
  const number = (value) => (typeof value === 'number' ? value : 0);
  const object = (value) => (value && typeof value === 'object' && !Array.isArray(value) ? value : null);
  for (const agent of agents) {
    const label = `agent=${goQuote(agent.id)},hostname=${goQuote(agent.hostname)},os=${goQuote(agent.os)}`;
    lines.push(`mrti_agent_up{${label}} ${agent.online ? 1 : 0}`);
    lines.push(`mrti_agent_self_mem_mb{${label}} ${goFormatG(agent.self_mem_mb)}`);
    lines.push(`mrti_agent_self_cpu_percent{${label}} ${goFormatG(agent.self_cpu_percent)}`);
    const modules = modulesByAgent[agent.id] || {};
    const cpu = object(modules.cpu);
    if (cpu) lines.push(`mrti_cpu_usage_percent{${label}} ${goFormatG(number(cpu.usage_percent))}`);
    const ram = object(modules.ram);
    if (ram) {
      lines.push(`mrti_mem_used_percent{${label}} ${goFormatG(number(ram.used_percent))}`);
      lines.push(`mrti_swap_used_percent{${label}} ${goFormatG(number(ram.swap_used_percent))}`);
    }
    const disk = object(modules.disk);
    if (disk) for (const partition of Array.isArray(disk.partitions) ? disk.partitions : []) {
      lines.push(`mrti_disk_used_percent{${label},mount=${goQuote(partition?.mountpoint ?? '')}} ${goFormatG(number(partition?.used_percent))}`);
    }
    const temperature = object(modules.temperature);
    if (temperature && number(temperature.max_celsius) > 0) lines.push(`mrti_temperature_celsius{${label}} ${goFormatG(temperature.max_celsius)}`);
    const ups = object(modules.ups);
    if (ups) for (const item of Array.isArray(ups.upses) ? ups.upses : []) {
      lines.push(`mrti_ups_battery_percent{${label},ups=${goQuote(item?.name ?? '')}} ${goFormatG(number(item?.battery_charge_percent))}`);
    }
    const docker = object(modules.docker);
    if (docker) lines.push(`mrti_docker_containers_running{${label}} ${Math.trunc(number(docker.running))}`);
    const services = object(modules.services);
    if (services) lines.push(`mrti_services_failed{${label}} ${Math.trunc(number(services.failed))}`);
  }
  return `${lines.join('\n')}\n`;
}

// store: ver store.js. apiKey: X-MRTI-API-Key que presentan los agentes al
// reportar (y operaciones de archivo/borrado, igual que en Go). portalAccess:
// middleware del kit que valida la sesión de Core para el módulo agent-core.
export function createApp({ store, apiKey, portalAccess, healthCheck = async () => {}, before = [] }) {
  const app = express();
  app.disable('x-powered-by');
  for (const middleware of before) app.use(middleware);

  // Fallar cerrado: sin llave configurada este servidor no acepta reportes.
  // (El de Go aceptaba cualquiera si la llave venía vacía.)
  const agentKeyRequired = (req, res, next) => {
    if (!apiKey) return goError(res, 503, 'agent ingest not configured');
    if (!sameKey(req.get('X-MRTI-API-Key'), apiKey)) return goError(res, 401, 'unauthorized');
    next();
  };
  const operatorKeyRequired = (req, res, next) => {
    if (!apiKey || !sameKey(req.get('X-MRTI-API-Key'), apiKey)) return goJson(res, 401, { error: 'unauthorized' });
    next();
  };
  const wrap = (handler) => (req, res, next) => Promise.resolve(handler(req, res, next)).catch(next);

  app.get('/healthz', (_req, res) => res.type('text/plain; charset=utf-8').send('ok'));
  app.get('/api/health', wrap(async (_req, res) => {
    try { await healthCheck(); res.json({ ok: true, database: 'mysql' }); }
    catch { res.status(500).json({ ok: false, error: 'Sin conexión a MySQL' }); }
  }));

  // --- Hacia los agentes ---
  app.post('/api/v1/ingest', agentKeyRequired, rawBody, wrap(async (req, res) => {
    const kind = req.get('X-MRTI-Kind');
    if (kind === 'heartbeat') {
      const hb = decodeHeartbeat(req.rawBody);
      if (hb) await store.ingestHeartbeat(hb);
    } else if (kind === 'command_result') {
      const result = decodeCommandResult(req.rawBody);
      if (result) await store.saveCommandResult(result);
    } else {
      const env = decodeEnvelope(req.rawBody);
      if (env) await store.ingestEnvelope(env, goUnixSeconds);
    }
    res.type('application/json').send('{"ok":true}');
  }));

  // Igual que en Go, la cola de comandos no exige llave: el agente sólo
  // recibe los suyos por su id.
  app.get('/api/v1/agents/:id/commands', wrap(async (req, res) => {
    const commands = await store.takeCommands(req.params.id);
    if (!commands.length) return res.status(204).end();
    goJson(res, 200, commands);
  }));

  // --- Operadores (sesión de Core con acceso a agent-core) ---
  app.get('/api/v1/agents', portalAccess, wrap(async (_req, res) => goJson(res, 200, await store.listAgents())));
  app.get('/api/v1/agents/:id', portalAccess, wrap(async (req, res) => {
    const envelope = await store.getEnvelope(req.params.id);
    if (envelope === null) return goError(res, 404, '{"error":"agent not found"}');
    res.type('application/json').send(envelope);
  }));
  app.patch('/api/v1/agents/:id', operatorKeyRequired, rawBody, wrap(async (req, res) => {
    let archived;
    try { archived = JSON.parse(req.rawBody)?.archived; } catch { archived = undefined; }
    if (typeof archived !== 'boolean') return goJson(res, 400, { error: 'body must include archived' });
    if (!await store.setAgentArchived(req.params.id, archived)) return goJson(res, 404, { error: 'agent not found' });
    goJson(res, 200, { ok: true, archived });
  }));
  app.delete('/api/v1/agents/:id', operatorKeyRequired, wrap(async (req, res) => {
    try {
      if (!await store.deleteAgent(req.params.id)) return goJson(res, 404, { error: 'agent not found' });
      res.status(204).end();
    } catch (error) {
      if (error instanceof AgentOnlineError) return goJson(res, 409, { error: 'disconnect the agent before deleting it' });
      throw error;
    }
  }));
  app.get('/api/v1/agents/:id/modules/:module', portalAccess, wrap(async (req, res) => {
    const data = await store.getModule(req.params.id, req.params.module);
    if (data === null) return goError(res, 404, '{"error":"no data for that agent/module"}');
    res.type('application/json').send(data);
  }));
  app.post('/api/v1/agents/:id/commands', portalAccess, rawBody, wrap(async (req, res) => {
    let command;
    try { command = JSON.parse(req.rawBody); } catch { command = null; }
    if (!command || typeof command !== 'object' || typeof command.type !== 'string' || !command.type) return goError(res, 400, '{"error":"body must be a command with a type"}');
    // Go: "cmd-" + UnixNano en base 36 cuando el operador no manda id.
    const unixNano = BigInt(Date.now()) * 1000000n + (process.hrtime.bigint() % 1000000n);
    const id = typeof command.id === 'string' && command.id ? command.id : `cmd-${unixNano.toString(36)}`;
    await store.enqueueCommand(req.params.id, { id, type: command.type, payload: 'payload' in command ? command.payload : undefined });
    goJson(res, 202, { queued: id });
  }));
  app.get('/api/v1/alerts', portalAccess, wrap(async (req, res) => {
    const requested = Number.parseInt(req.query.limit, 10);
    goJson(res, 200, await store.recentAlerts(Number.isInteger(requested) && requested > 0 ? Math.min(requested, 10000) : 100));
  }));
  app.get('/api/v1/export', portalAccess, wrap(async (_req, res) => {
    const agents = await store.listAgents();
    const fleet = [];
    for (const agent of agents) fleet.push({ agent, modules: await store.modulesFor(agent.id) });
    res.type('application/json').set('Content-Disposition', 'attachment; filename=mrti-export.json')
      .send(JSON.stringify({ exported_at: new Date().toISOString().replace(/\.\d{3}Z$/, 'Z'), agent_count: agents.length, agents: fleet, alerts: nullIfEmpty(await store.recentAlerts(200)) }));
  }));
  app.get('/metrics', wrap(async (_req, res) => {
    const agents = await store.listAgents();
    const modules = {};
    for (const agent of agents) modules[agent.id] = await store.modulesFor(agent.id);
    res.type('text/plain; charset=utf-8').send(metricsText(agents, modules));
  }));

  app.use((err, _req, res, _next) => {
    console.error('[mrti-agent-core-api]', err);
    goJson(res, 500, { error: 'internal error' });
  });
  return app;
}
