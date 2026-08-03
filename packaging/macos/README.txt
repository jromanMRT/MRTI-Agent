MRTI Agent para macOS
=====================

1. Edita config.yaml y configura server.url y server.api_key.
2. Abre Terminal en esta carpeta.
3. Ejecuta:  sudo ./install.sh

El agente se instala como un servicio launchd del sistema, inicia al encender
la Mac y funciona en segundo plano aunque ningún usuario haya iniciado sesión.

Estado:       launchctl print system/com.mrti.agent
Desinstalar:  sudo launchctl bootout system/com.mrti.agent
