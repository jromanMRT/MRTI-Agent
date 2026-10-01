// pm2: Agent Core en Node (Fase 1). El servidor de Go (mrti-monitor, systemd)
// sigue siendo el que reciben los agentes; este proceso corre en paralelo
// solo en 127.0.0.1:3007, sin ruta en nginx.
module.exports = {
  apps: [
    {
      name: 'mrti-agent-core-api',
      cwd: './server',
      script: 'src/index.js',
      instances: 1,
      exec_mode: 'fork',
      max_memory_restart: '300M',
      env: {
        NODE_ENV: 'production',
      },
    },
  ],
};
