MRTI Agent — Windows package (amd64)
====================================

Contents
  mrti-agent.exe        the monitoring agent
  mrti-core.exe         (optional) the reference Core server + API + dashboard
  config.yaml           agent configuration — EDIT BEFORE INSTALLING
  plugins\ping.exe      example gRPC plugin (TCP latency probe)
  install-windows.ps1   installs the agent as a Windows service
  install-core-windows.ps1 installs Core as a Windows service

------------------------------------------------------------------------
QUICK START (agent)
------------------------------------------------------------------------
1) Edit config.yaml BEFORE installing:
     - server.url : the address of your MRTI Core (e.g. http://192.168.1.5:8477)
     - api_key    : must match the Core's -api-key
2) Open PowerShell AS ADMINISTRATOR in this folder and run:
     Set-ExecutionPolicy -Scope Process Bypass
     .\install-windows.ps1
3) Check it:  Get-Service mrti-agent
   Logs:      "%ProgramFiles%\MRTI Agent\logs\"

The service uses Automatic startup, runs in the background without a desktop
window, and is configured to restart after a failure. You do not need to sign
in to Windows for it to run.

Run in the foreground for a quick test (no install):
     .\mrti-agent.exe -foreground -config .\config.yaml
List modules:
     .\mrti-agent.exe -list-modules
Uninstall:
     & "$env:ProgramFiles\MRTI Agent\mrti-agent.exe" -service uninstall -config "$env:ProgramFiles\MRTI Agent\config.yaml"

------------------------------------------------------------------------
OPTIONAL: run the Core server (dashboard + API) on this or another machine
------------------------------------------------------------------------
Persistent installation (recommended):
1) Open PowerShell AS ADMINISTRATOR in this folder.
2) Choose an API key and run:
     .\install-core-windows.ps1 -ApiKey "replace-with-a-secure-key"
3) Check it:
     Get-Service mrti-core

Core will start automatically with Windows, run without a CMD window and
restart after a failure. Its database and log are stored in:
     "%ProgramData%\MRTI Core\"

Foreground mode is only for a quick test:
     .\mrti-core.exe -addr :8477 -db core.db -api-key demo-api-key
Then browse:
     http://localhost:8477/                     (dashboard)
     http://localhost:8477/api/v1/agents        (JSON API)
     http://localhost:8477/metrics              (Prometheus)

If Agent and Core are on the same PC, use the same API key in config.yaml and
set server.url to http://127.0.0.1:8477 before installing the Agent.

NOTE: these binaries are not code-signed. Windows SmartScreen may warn on first
run — choose "More info" > "Run anyway", or sign them with your certificate.
