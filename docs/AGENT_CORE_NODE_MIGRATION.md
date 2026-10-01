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
| 2 | Copia de datos SQLite (`run/core.db`) → MySQL, idempotente y verificada por conteos | Completa (2026-10-01); se repite justo antes del corte |
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

### Rollback (Fase 1)

El servidor Node no recibe tráfico: `pm2 delete mrti-agent-core-api && pm2
save` lo retira sin efecto en agentes ni usuarios. `mrti-monitor` (Go) no se
modificó. La base `mrti_agent_core` puede conservarse o eliminarse (no la usa
nadie más).

## Fase 2

**Herramienta:** `npm run import:sqlite -- [--dry-run] [--prune] [--compare-go] [--source …]`
(`server/scripts/import-sqlite.mjs`).

1. Instantánea consistente de `run/core.db` con el respaldo en línea de
   SQLite (`.backup`) en modo **solo lectura**: `mrti-monitor` sigue
   recibiendo reportes sin bloquearse.
2. Copia en una transacción con `INSERT … ON DUPLICATE KEY UPDATE` por llave,
   conservando los ids de `alerts` y `commands`. Repetirla no duplica nada.
3. Verificación por tabla: conteo, filas faltantes, filas distintas y huella
   SHA-256 de las filas canónicas ordenadas por llave.
4. `--compare-go`: copia temporal del binario de producción sobre la misma
   instantánea y comparación de todas las consultas de solo lectura (lista,
   reporte y cada módulo de cada agente, alertas, exportación y `/metrics`)
   contra Node leyendo MySQL.
5. Las filas que sólo están en MySQL se reportan; `--prune` las borra (sólo
   tiene sentido antes del corte, mientras Node no recibe reportes).

**Diferencia conocida de Go:** `store.go` ignora el error de `rows.Scan`. En
una fila con `NULL` (por ejemplo, un agente que mandó reportes pero nunca un
heartbeat) Go pone en cero esa columna y todas las siguientes (`sequence`,
`archived`…); Node devuelve los valores reales. La comparación aparta esas
filas y las lista en `knownGoNullDifferences`. Hoy no hay ninguna.

### Evidencia

- Base de pruebas: copia del SQLite real y repetición sin cambios; fuente
  sintética con comandos con resultado, agente sin heartbeat, alertas con
  `NULL` y una fila sobrante en MySQL: huellas iguales, `--prune` borró la
  sobrante y 21/21 consultas idénticas (2 filas en diferencias conocidas).
- **Producción (`mrti_agent_core`, 2026-10-01):** simulacro sin escrituras;
  copia: agents 1/1, latest 1/1, module_data 15/15, alerts 19/19,
  commands 0/0, sin faltantes ni distintas, huellas iguales; **20/20
  consultas idénticas a Go**; repetición sin cambios. `mrti-monitor` siguió
  activo y recibiendo reportes durante la copia. Directorio temporal borrado.

### Pendiente hasta el corte

MySQL queda como foto del momento de la copia: Node todavía no recibe
reportes. En la Fase 4 se repite la copia (con `--prune --compare-go`) justo
antes de mover a los agentes.

### Rollback (Fase 2)

No toca el SQLite ni el servicio de Go. Para deshacer: vaciar las tablas de
`mrti_agent_core` (nadie más las usa) o volver a correr la copia.
