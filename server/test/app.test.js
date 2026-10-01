import test from 'node:test';
import assert from 'node:assert/strict';
import http from 'node:http';
import { createApp, metricsText } from '../src/app.js';

function fakeStore() {
  const calls = [];
  return {
    calls,
    ingestEnvelope: async (env) => calls.push(['envelope', env.agent_id]),
    ingestHeartbeat: async (hb) => calls.push(['heartbeat', hb.agent_id]),
    saveCommandResult: async () => {},
    takeCommands: async () => [],
    listAgents: async () => [],
    recentAlerts: async () => [],
  };
}

async function serve(options) {
  const server = http.createServer(createApp(options));
  await new Promise((resolve) => server.listen(0, '127.0.0.1', resolve));
  const base = `http://127.0.0.1:${server.address().port}`;
  return { base, close: () => server.close() };
}

const allow = (_req, _res, next) => next();

test('sin AGENT_API_KEY no acepta reportes (falla cerrado)', async () => {
  const store = fakeStore();
  const { base, close } = await serve({ store, apiKey: '', portalAccess: allow });
  try {
    const response = await fetch(`${base}/api/v1/ingest`, { method: 'POST', headers: { 'X-MRTI-API-Key': '' }, body: '{}' });
    assert.equal(response.status, 503);
    assert.equal(store.calls.length, 0);
  } finally { close(); }
});

test('con llave correcta guarda el reporte; con incorrecta responde 401', async () => {
  const store = fakeStore();
  const { base, close } = await serve({ store, apiKey: 'k', portalAccess: allow });
  try {
    const body = JSON.stringify({ agent_id: 'a1', timestamp: '2026-10-01T08:00:00Z', results: [] });
    assert.equal((await fetch(`${base}/api/v1/ingest`, { method: 'POST', headers: { 'X-MRTI-API-Key': 'x' }, body })).status, 401);
    const ok = await fetch(`${base}/api/v1/ingest`, { method: 'POST', headers: { 'X-MRTI-API-Key': 'k' }, body });
    assert.equal(ok.status, 200);
    assert.equal(await ok.text(), '{"ok":true}');
    assert.deepEqual(store.calls, [['envelope', 'a1']]);
  } finally { close(); }
});

test('las rutas de operador pasan por la validación de sesión', async () => {
  const deny = (_req, res) => res.status(401).json({ error: 'No autenticado' });
  const { base, close } = await serve({ store: fakeStore(), apiKey: 'k', portalAccess: deny });
  try {
    for (const path of ['/api/v1/agents', '/api/v1/agents/a/modules/cpu', '/api/v1/alerts', '/api/v1/export']) {
      assert.equal((await fetch(base + path)).status, 401, path);
    }
    // La cola de comandos y las métricas no piden sesión, igual que en Go.
    assert.equal((await fetch(`${base}/api/v1/agents/a/commands`)).status, 204);
    assert.equal((await fetch(`${base}/metrics`)).status, 200);
  } finally { close(); }
});

test('lista vacía se responde como null, igual que Go', async () => {
  const { base, close } = await serve({ store: fakeStore(), apiKey: 'k', portalAccess: allow });
  try { assert.equal(await (await fetch(`${base}/api/v1/agents`)).text(), 'null'); } finally { close(); }
});

test('métricas: líneas por módulo con formato de Go', () => {
  const text = metricsText([{ id: 'a', hostname: 'pc "1"', os: 'windows', online: true, self_mem_mb: 1234567, self_cpu_percent: 0.5 }], {
    a: { disk: { partitions: [{ mountpoint: 'C:\\', used_percent: 50 }] }, temperature: { max_celsius: 0 }, docker: { running: 3 } },
  });
  assert.match(text, /mrti_agent_up\{agent="a",hostname="pc \\"1\\"",os="windows"\} 1/);
  assert.match(text, /mrti_agent_self_mem_mb\{.*\} 1\.234567e\+06/);
  assert.match(text, /mount="C:\\\\"\} 50/);
  assert.doesNotMatch(text, /mrti_temperature_celsius\{/);
  assert.match(text, /mrti_docker_containers_running\{.*\} 3/);
});
