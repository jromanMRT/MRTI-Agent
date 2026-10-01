import mysql from 'mysql2/promise';

export const pool = mysql.createPool({
  host: process.env.MYSQL_HOST || 'localhost',
  port: Number(process.env.MYSQL_PORT || 3306),
  user: process.env.MYSQL_USER || 'root',
  password: process.env.MYSQL_PASSWORD || '',
  database: process.env.MYSQL_DATABASE || 'mrti_agent_core',
  waitForConnections: true,
  connectionLimit: 10,
  // BIGINT (sequence, tiempos) como número: los valores caben en 2^53.
  supportBigNumbers: true,
  bigNumberStrings: false,
});
