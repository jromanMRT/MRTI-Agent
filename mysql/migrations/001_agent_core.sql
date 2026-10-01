-- 001: almacenamiento de Agent Core en MySQL (Fase 1 de la migración del
-- servidor de Go a Node). Mismas tablas y columnas que el SQLite de
-- cmd/mrti-monitor/store.go, para que la copia de la Fase 2 sea 1:1:
-- último estado por agente, último dato por módulo, alertas y cola de
-- comandos. Tiempos en segundos Unix (BIGINT) igual que en Go.
-- Idempotente: CREATE TABLE IF NOT EXISTS.
CREATE TABLE IF NOT EXISTS agents (
  id VARCHAR(64) NOT NULL,
  hostname VARCHAR(255) NULL,
  name VARCHAR(255) NULL,
  os VARCHAR(64) NULL,
  arch VARCHAR(64) NULL,
  version VARCHAR(64) NULL,
  tags TEXT NULL,
  first_seen BIGINT NULL,
  last_seen BIGINT NULL,
  last_heartbeat BIGINT NULL,
  self_cpu DOUBLE NULL,
  self_mem DOUBLE NULL,
  sequence BIGINT UNSIGNED NOT NULL DEFAULT 0,
  archived TINYINT(1) NOT NULL DEFAULT 0,
  PRIMARY KEY (id),
  KEY idx_agents_hostname (hostname)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE IF NOT EXISTS latest (
  agent_id VARCHAR(64) NOT NULL,
  sequence BIGINT UNSIGNED NULL,
  ts BIGINT NULL,
  envelope LONGTEXT NULL,
  PRIMARY KEY (agent_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE IF NOT EXISTS module_data (
  agent_id VARCHAR(64) NOT NULL,
  module VARCHAR(64) NOT NULL,
  ts BIGINT NULL,
  data LONGTEXT NULL,
  PRIMARY KEY (agent_id, module)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE IF NOT EXISTS alerts (
  id BIGINT NOT NULL AUTO_INCREMENT,
  agent_id VARCHAR(64) NULL,
  hostname VARCHAR(255) NULL,
  rule VARCHAR(255) NULL,
  severity VARCHAR(32) NULL,
  resource VARCHAR(512) NULL,
  message TEXT NULL,
  value DOUBLE NULL,
  threshold DOUBLE NULL,
  ts BIGINT NULL,
  PRIMARY KEY (id),
  KEY idx_alerts_ts (ts),
  KEY idx_alerts_agent (agent_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;

CREATE TABLE IF NOT EXISTS commands (
  id BIGINT NOT NULL AUTO_INCREMENT,
  agent_id VARCHAR(64) NULL,
  cmd_id VARCHAR(128) NULL,
  type VARCHAR(64) NULL,
  payload LONGTEXT NULL,
  created BIGINT NULL,
  delivered TINYINT(1) NOT NULL DEFAULT 0,
  result LONGTEXT NULL,
  result_ok TINYINT(1) NULL,
  result_ts BIGINT NULL,
  PRIMARY KEY (id),
  KEY idx_cmd_agent (agent_id, delivered)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_bin;
