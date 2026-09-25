#!/usr/bin/env bash
# Cambia /agent-core en nginx de una redirección cruzada de puerto (302 a
# :8477) a un proxy en el mismo origen que el resto de la plataforma, ahora
# que el binario de mrti-monitor entiende el prefijo /agent-core (ver
# rootHandler en cmd/mrti-monitor/main.go). El puerto :8477 en sí no cambia
# ni deja de servir sin prefijo: los agentes de telemetría que le hablan
# directo a ese puerto no se ven afectados.
#
# Requiere sudo. Hace respaldo y revierte automáticamente si nginx rechaza
# la configuración nueva, igual que los demás scripts de activación del
# proyecto.
#
# Uso:
#   sudo ./deploy/activate-agent-core-proxy.sh
set -euo pipefail

CONF=/etc/nginx/sites-available/it-infra
BACKUP="${CONF}.bak-agent-core-proxy-$(date +%Y%m%d-%H%M%S)"

if [ "$(id -u)" -ne 0 ]; then
  echo "Este script necesita sudo (edita nginx y recarga el servicio)." >&2
  exit 1
fi

if ! grep -q 'return 302 http://\$host:8477/;' "$CONF"; then
  echo "No se encontró el bloque de redirección esperado en $CONF." >&2
  echo "¿Ya se aplicó este cambio, o el archivo cambió de forma inesperada? Revísalo a mano." >&2
  exit 1
fi

cp "$CONF" "$BACKUP"
echo "Respaldo guardado en $BACKUP"

python3 - "$CONF" <<'PYEOF'
import sys

path = sys.argv[1]
with open(path, "r", encoding="utf-8") as fh:
    content = fh.read()

old = """    # El Core de agentes conserva su puerto porque sus enlaces y endpoints
    # internos están publicados desde la raíz del servicio Go.
    location = /agent-core {
        return 302 http://$host:8477/;
    }

    location = /agent-core/ {
        return 302 http://$host:8477/;
    }
"""

new = """    # El Core de agentes vive en el mismo origen que el resto de la
    # plataforma: mrti-monitor entiende el prefijo /agent-core (rootHandler
    # en MRTI-Agent/cmd/mrti-monitor/main.go) y sigue escuchando sin cambios
    # en :8477 para los agentes de telemetría que le hablan directo a ese
    # puerto (nunca pasan por aquí).
    location = /agent-core {
        return 301 /agent-core/;
    }

    location ^~ /agent-core/ {
        proxy_pass http://127.0.0.1:8477;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
"""

if old not in content:
    print("El bloque exacto a reemplazar no coincide; no se tocó el archivo.", file=sys.stderr)
    sys.exit(1)

with open(path, "w", encoding="utf-8") as fh:
    fh.write(content.replace(old, new, 1))
PYEOF

if ! nginx -t; then
  echo "nginx -t falló; restaurando el respaldo." >&2
  cp "$BACKUP" "$CONF"
  exit 1
fi

systemctl reload nginx
echo "Listo. /agent-core/ ahora es un proxy al puerto 8477 en el mismo origen."
echo "Respaldo conservado en $BACKUP por si hace falta revertir."
