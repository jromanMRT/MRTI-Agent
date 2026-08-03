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
	[string]$Binary = "",
	[string]$Config = "",
	[string]$InstallDir = "$env:ProgramFiles\MRTI Agent"
)

$ErrorActionPreference = "Stop"

$principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
    throw "This installer must run as Administrator."
}
if ([string]::IsNullOrWhiteSpace($Binary)) {
	$Binary = Join-Path $PSScriptRoot "mrti-agent.exe"
}
if ([string]::IsNullOrWhiteSpace($Config)) {
	$Config = Join-Path $PSScriptRoot "config.yaml"
}
$Binary = [IO.Path]::GetFullPath($Binary)
$Config = [IO.Path]::GetFullPath($Config)
$InstallDir = [IO.Path]::GetFullPath($InstallDir)

if (-not (Test-Path -LiteralPath $Binary -PathType Leaf)) {
	throw "mrti-agent.exe not found at '$Binary'."
}
if (-not (Test-Path -LiteralPath $Config -PathType Leaf) -and
	-not (Test-Path -LiteralPath (Join-Path $InstallDir "config.yaml") -PathType Leaf)) {
	throw "config.yaml not found at '$Config'. Edit it with the Core URL and API key before installing."
}

Write-Host "==> Installing MRTI Agent to $InstallDir"
New-Item -ItemType Directory -Force -Path $InstallDir, "$InstallDir\logs", "$InstallDir\cache", "$InstallDir\plugins" | Out-Null
Copy-Item -LiteralPath $Binary "$InstallDir\mrti-agent.exe" -Force
$PingPlugin = Join-Path $PSScriptRoot "plugins\ping.exe"
if (Test-Path -LiteralPath $PingPlugin -PathType Leaf) {
	Copy-Item -LiteralPath $PingPlugin "$InstallDir\plugins\ping.exe" -Force
}

if (-not (Test-Path "$InstallDir\config.yaml")) {
	Copy-Item -LiteralPath $Config "$InstallDir\config.yaml" -Force
	Write-Host "==> Installed config at $InstallDir\config.yaml"
}

$exe = "$InstallDir\mrti-agent.exe"
& $exe -service install -config "$InstallDir\config.yaml"
if ($LASTEXITCODE -ne 0) { throw "Could not install the MRTI Agent service." }
& $exe -service start   -config "$InstallDir\config.yaml"
if ($LASTEXITCODE -ne 0) { throw "The MRTI Agent service was installed but could not be started." }

# Explicitly configure recovery as a second line of defence. The binary also
# requests these settings when it registers itself with Windows.
& sc.exe failure "mrti-agent" reset= 86400 actions= restart/5000/restart/15000/restart/60000 | Out-Null
if ($LASTEXITCODE -ne 0) { throw "Could not configure service recovery." }
& sc.exe failureflag "mrti-agent" 1 | Out-Null
if ($LASTEXITCODE -ne 0) { throw "Could not enable service recovery actions." }

Write-Host "==> Done. Manage with:  Get-Service mrti-agent"
Write-Host "    Logs: $InstallDir\logs\"
Write-Host "    Startup: Automatic; no desktop window; recovery: restart on failure"
Write-Host "    Uninstall:  & '$exe' -service uninstall -config '$InstallDir\config.yaml'"
