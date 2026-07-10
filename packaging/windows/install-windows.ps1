<#
.SYNOPSIS
  Install the MRTI Agent as a Windows service (run from this folder).
.DESCRIPTION
  Copies the agent + ping plugin + config into Program Files and registers the
  Windows service. Run in an elevated (Administrator) PowerShell:

      Set-ExecutionPolicy -Scope Process Bypass
      .\install-windows.ps1

  Edit config.yaml first (server.url, api_key) so the agent points at your Core.
#>
param(
    [string]$Binary = ".\mrti-agent.exe",
    [string]$InstallDir = "$env:ProgramFiles\MRTI Agent"
)

$ErrorActionPreference = "Stop"

$principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw "This installer must run as Administrator."
}
if (-not (Test-Path $Binary)) {
    throw "mrti-agent.exe not found next to this script."
}

Write-Host "==> Installing MRTI Agent to $InstallDir"
New-Item -ItemType Directory -Force -Path $InstallDir, "$InstallDir\logs", "$InstallDir\cache", "$InstallDir\plugins" | Out-Null
Copy-Item $Binary "$InstallDir\mrti-agent.exe" -Force
if (Test-Path ".\plugins\ping.exe") { Copy-Item ".\plugins\ping.exe" "$InstallDir\plugins\ping.exe" -Force }

if (-not (Test-Path "$InstallDir\config.yaml")) {
    Copy-Item ".\config.yaml" "$InstallDir\config.yaml" -Force
    Write-Host "==> Installed config at $InstallDir\config.yaml — EDIT server.url and api_key."
}

$exe = "$InstallDir\mrti-agent.exe"
& $exe -service install -config "$InstallDir\config.yaml"
& $exe -service start   -config "$InstallDir\config.yaml"

Write-Host "==> Done. Manage with:  Get-Service mrti-agent"
Write-Host "    Logs: $InstallDir\logs\"
Write-Host "    Uninstall:  & '$exe' -service uninstall -config '$InstallDir\config.yaml'"
