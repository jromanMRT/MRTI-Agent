<#
.SYNOPSIS
  Install the MRTI Agent as a Windows service.
.DESCRIPTION
  Copies the agent binary and config into Program Files and registers the
  service using the agent's own -service install action. Run in an elevated
  (Administrator) PowerShell.
.EXAMPLE
  .\install-windows.ps1 -Binary .\dist\windows-amd64\mrti-agent.exe
#>
param(
    [string]$Binary = ".\dist\windows-amd64\mrti-agent.exe",
    [string]$InstallDir = "$env:ProgramFiles\MRTI Agent"
)

$ErrorActionPreference = "Stop"

# Require elevation.
$principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw "This installer must run as Administrator."
}

if (-not (Test-Path $Binary)) {
    throw "Agent binary not found at '$Binary'. Build it first: make build-windows"
}

Write-Host "==> Installing MRTI Agent to $InstallDir"
New-Item -ItemType Directory -Force -Path $InstallDir, "$InstallDir\logs", "$InstallDir\cache", "$InstallDir\plugins" | Out-Null
Copy-Item $Binary "$InstallDir\mrti-agent.exe" -Force

if (-not (Test-Path "$InstallDir\config.yaml")) {
    Copy-Item ".\config.yaml.example" "$InstallDir\config.yaml" -Force
    Write-Host "==> Installed default config at $InstallDir\config.yaml — EDIT server.url and credentials."
}

# Register + start the service via the agent's built-in service control.
$exe = "$InstallDir\mrti-agent.exe"
& $exe -service install -config "$InstallDir\config.yaml"
& $exe -service start   -config "$InstallDir\config.yaml"

Write-Host "==> Done. Manage with: Get-Service mrti-agent"
Write-Host "Logs: $InstallDir\logs\"
