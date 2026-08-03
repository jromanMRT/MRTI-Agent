MRTI Agent para Linux
=====================

1. Edita config.yaml y configura server.url y server.api_key.
2. Abre una terminal en esta carpeta.
3. Ejecuta:  sudo ./install.sh

El agente se instala como un servicio systemd, inicia automáticamente y se
reinicia si falla. No es necesario mantener una terminal abierta.

Estado:       systemctl status mrti-agent
Logs:         journalctl -u mrti-agent -f
Desinstalar:  sudo systemctl disable --now mrti-agent
