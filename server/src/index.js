import 'dotenv/config';
import { createApp } from './app.js';
import { moduleAccessRequired } from './auth.js';
import { pool } from './db.js';
import { createStore } from './store.js';
import { requestContext } from './vendor/mrti-platform/request-context.js';

const store = createStore(pool);
const app = createApp({
  store,
  apiKey: process.env.AGENT_API_KEY || '',
  portalAccess: moduleAccessRequired,
  healthCheck: () => pool.query('SELECT 1'),
  before: [requestContext({ service: 'mrti-agent-core-api' })],
});

if (!process.env.AGENT_API_KEY) console.warn('[mrti-agent-core-api] AGENT_API_KEY vacía: /api/v1/ingest responde 503 hasta configurarla.');
const port = Number(process.env.PORT || 3007);
const host = process.env.HOST || '127.0.0.1';
app.listen(port, host, () => console.log(`MRTI Agent Core API escuchando en http://${host}:${port}`));
