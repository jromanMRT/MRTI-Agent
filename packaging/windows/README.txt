MRTI Agent — Windows package (amd64)
====================================

Contents
  mrti-agent.exe        the monitoring agent
  mrti-core.exe         (optional) the reference Core server + API + dashboard
  config.yaml           agent configuration — EDIT BEFORE INSTALLING
  plugins\ping.exe      example gRPC plugin (TCP latency probe)
  install-windows.ps1   installs the agent as a Windows service

------------------------------------------------------------------------
QUICK START (agent)
------------------------------------------------------------------------
1) Edit config.yaml:
     - server.url : the address of your MRTI Core (e.g. http://192.168.1.5:8477)
     - api_key    : must match the Core's -api-key
2) Open PowerShell AS ADMINISTRATOR in this folder and run:
     Set-ExecutionPolicy -Scope Process Bypass
     .\install-windows.ps1
3) Check it:  Get-Service mrti-agent
   Logs:      "%ProgramFiles%\MRTI Agent\logs\"

Run in the foreground for a quick test (no install):
     .\mrti-agent.exe -foreground -config .\config.yaml
List modules:
     .\mrti-agent.exe -list-modules
Uninstall:
     & "$env:ProgramFiles\MRTI Agent\mrti-agent.exe" -service uninstall -config "$env:ProgramFiles\MRTI Agent\config.yaml"

------------------------------------------------------------------------
OPTIONAL: run the Core server (dashboard + API) on this or another machine
------------------------------------------------------------------------
     .\mrti-core.exe -addr :8477 -db core.db -api-key demo-api-key
Then browse:
     http://localhost:8477/                     (dashboard)
     http://localhost:8477/api/v1/agents        (JSON API)
     http://localhost:8477/metrics              (Prometheus)

NOTE: these binaries are not code-signed. Windows SmartScreen may warn on first
run — choose "More info" > "Run anyway", or sign them with your certificate.
