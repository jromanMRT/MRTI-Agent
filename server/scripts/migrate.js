// Aplica las migraciones pendientes de mysql/migrations en orden.
// Uso: npm run migrate (desde server/) — mismo patrón que los demás módulos.
import 'dotenv/config';
import mysql from 'mysql2/promise';
import { readdir, readFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const MIGRATIONS_DIR = path.resolve(
  path.dirname(fileURLToPath(import.meta.url)),
  '../../mysql/migrations'
);

const conn = await mysql.createConnection({
  host: process.env.MYSQL_HOST || 'localhost',
  port: Number(process.env.MYSQL_PORT || 3306),
  user: process.env.MYSQL_USER || 'root',
  password: process.env.MYSQL_PASSWORD || '',
  database: process.env.MYSQL_DATABASE || 'mrti_agent_core',
  multipleStatements: true,
});

await conn.query(`CREATE TABLE IF NOT EXISTS schema_migrations (
  name VARCHAR(255) NOT NULL PRIMARY KEY,
  applied_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
)`);

const [applied] = await conn.query('SELECT name FROM schema_migrations');
const done = new Set(applied.map((r) => r.name));

const files = (await readdir(MIGRATIONS_DIR)).filter((f) => f.endsWith('.sql')).sort();
for (const file of files) {
  if (done.has(file)) continue;
  const sql = await readFile(path.join(MIGRATIONS_DIR, file), 'utf8');
  console.log(`Aplicando ${file}...`);
  await conn.query(sql);
  await conn.query('INSERT INTO schema_migrations (name) VALUES (?)', [file]);
}

console.log('Migraciones al día.');
await conn.end();
