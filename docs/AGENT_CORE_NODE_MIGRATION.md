# Agent Core: servidor de Go → Node

Fecha de inicio: 2026-10-01.

## Objetivo

Que el servidor y panel de Agent Core usen el mismo stack que los demás
módulos (Node + Express, MySQL, sesión de Core por el kit MRTI-Platform, pm2,
nginx en el mismo origen y, en la Fase 3, frontend React con releases).

**Fuera de alcance:** el agente que se instala en cada equipo
(`cmd/mrti-agent`) sigue en Go: es un binario único sin dependencias que
corre como servicio de Windows/Linux. Su contrato con el servidor no cambia,
así que ningún equipo se reinstala.

## Fases

| Fase | Qué | Estado |
|---|---|---|
| 1 | Servidor Node con el mismo contrato que `cmd/mrti-monitor`, en paralelo y sin tráfico de agentes | Completa (2026-10-01) |
| 2 | Copia de datos SQLite (`run/core.db`) → MySQL, idempotente y verificada por conteos | Pendiente |
| 3 | Panel React igual a Activos/RH bajo `/agent-core/` (releases) | Pendiente |
| 4 | Corte: nginx `/agent-core/` y los agentes (`server.url`) al servidor Node, con la misma `AGENT_API_KEY`; Go queda de respaldo | Pendiente |
| 5 | Retiro de `mrti-monitor` tras el periodo de compatibilidad | Pendiente, requiere autorización |

## Fase 1

**Dónde:** `server/` (Express, `type: module`), `mysql/migrations/`,
`ecosystem.config.cjs` (pm2 `mrti-agent-core-api`, `127.0.0.1:3007`).

**Contrato:** mismas rutas, códigos y cuerpos que `docs/CORE-API.md`:
`POST /api/v1/ingest` (gzip, `X-MRTI-Kind`, `X-MRTI-API-Key`),
`GET /api/v1/agents/{id}/commands` (204 sin comandos), API de operador
(`agents`, `modules`, `commands`, `alerts`, `export`), `PATCH`/`DELETE` con
llave de operador, `/metrics` (Prometheus, formato `%g`/`%q` de Go) y
`/healthz`. Además `/api/health` como el resto de la plataforma.

**Base:** `mrti_agent_core`, mismas tablas y columnas que el SQLite de Go
(`agents`, `latest`, `module_data`, `alerts`, `commands`) para que la Fase 2
sea una copia 1:1. Migración `001_agent_core.sql` idempotente.

**Diferencias intencionales con Go:**

- Sin `AGENT_API_KEY` el servidor **no acepta** reportes (503). Go aceptaba
  cualquiera si la llave venía vacía.
- La entrega de comandos es transaccional (`SELECT … FOR UPDATE`): dos
  consultas simultáneas no entregan el mismo comando dos veces.
- Los mensajes de error de sesión son los del kit (mismos códigos
  401/403/503, distinto texto).

Se conservan a propósito comportamientos de Go de los que dependen los
consumidores: lista vacía como `null`, JSON inválido aceptado e ignorado,
heartbeats que dejan `tags` vacías, `issued_at` en valor cero al entregar
comandos.

### Evidencia

- `npm test`: 11/11 (formatos de Go, llave obligatoria, sesión en rutas de
  operador, métricas).
- `npm run parity` (`scripts/parity-check.mjs`): levanta una **copia
  temporal** del binario en producción (`/var/www/mrt/MRTI/bin/mrti-monitor`)
  con SQLite desechable y Core simulado, toma el último reporte real de
  `run/core.db` en modo **solo lectura** y manda el mismo tráfico a Go y a
  Node: **54/54 casos idénticos** (código y cuerpo). Incluye el reporte real
  (184 KB, 15 módulos) en claro y gzip, cada módulo, alertas con `<`, `&` y
  comillas, `/metrics` con valores en notación exponencial (`1.23456725e+06`,
  `1e-07`), cola de comandos, archivar/borrar y los códigos de sesión
  401/401/403/503. No deja datos: vacía las tablas de prueba y borra el
  directorio temporal.
- `npm run migrate` aplicado dos veces sobre la base de pruebas: la segunda
  no cambia nada.
- `npm audit --omit=dev`: 0 vulnerabilidades.
- Despliegue: base `mrti_agent_core` creada por jroman; migración aplicada dos
  veces sin cambios; pm2 `mrti-agent-core-api` online y guardado; `/api/health`
  200, rutas de operador sin sesión 401, sesión inválida (Core real) 401,
  ingest sin llave 503, puerto no expuesto a la red; `health.mjs` saludable.

### Rollback

El servidor Node no recibe tráfico: `pm2 delete mrti-agent-core-api && pm2
save` lo retira sin efecto en agentes ni usuarios. `mrti-monitor` (Go) no se
modificó. La base `mrti_agent_core` puede conservarse o eliminarse (no la usa
nadie más).
