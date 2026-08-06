<#
.SYNOPSIS
  Install MRTI Monitor as a persistent Windows service.
.DESCRIPTION
  Run from an elevated PowerShell. The service starts automatically with
  Windows, runs without a console window and restarts after failures.
#>
param(
	[string]$Binary = "",
	[string]$InstallDir = "$env:ProgramFiles\MRTI Monitor",
	[string]$DataDir = "$env:ProgramData\MRTI Monitor",
	[string]$Addr = ":8477",
	[Parameter(Mandatory = $true)]
	[ValidateNotNullOrEmpty()]
	[string]$ApiKey
)

$ErrorActionPreference = "Stop"

$principal = New-Object Security.Principal.WindowsPrincipal([Security.Principal.WindowsIdentity]::GetCurrent())
if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
	throw "This installer must run as Administrator."
}
if ([string]::IsNullOrWhiteSpace($Binary)) {
	$Binary = Join-Path $PSScriptRoot "mrti-monitor.exe"
}
$Binary = [IO.Path]::GetFullPath($Binary)
$InstallDir = [IO.Path]::GetFullPath($InstallDir)
$DataDir = [IO.Path]::GetFullPath($DataDir)

if (-not (Test-Path -LiteralPath $Binary -PathType Leaf)) {
	throw "mrti-monitor.exe not found at '$Binary'."
}

Write-Host "==> Installing MRTI Monitor to $InstallDir"
New-Item -ItemType Directory -Force -Path $InstallDir, $DataDir, "$DataDir\downloads" | Out-Null
Copy-Item -LiteralPath $Binary "$InstallDir\mrti-monitor.exe" -Force

$exe = "$InstallDir\mrti-monitor.exe"
$db = "$DataDir\core.db"
$downloads = "$DataDir\downloads"
& $exe -service install -addr $Addr -db $db -api-key $ApiKey -downloads-dir $downloads
if ($LASTEXITCODE -ne 0) { throw "Could not install the MRTI Monitor service." }
& $exe -service start -addr $Addr -db $db -api-key $ApiKey -downloads-dir $downloads
if ($LASTEXITCODE -ne 0) { throw "MRTI Monitor was installed but could not be started." }

& sc.exe failure "mrti-monitor" reset= 86400 actions= restart/5000/restart/15000/restart/60000 | Out-Null
if ($LASTEXITCODE -ne 0) { throw "Could not configure service recovery." }
& sc.exe failureflag "mrti-monitor" 1 | Out-Null
if ($LASTEXITCODE -ne 0) { throw "Could not enable service recovery actions." }

Write-Host "==> Done. Dashboard: http://localhost$Addr/"
Write-Host "    Status: Get-Service mrti-monitor"
Write-Host "    Log: $DataDir\mrti-monitor.log"
Write-Host "    Startup: Automatic; no desktop window; recovery: restart on failure"
Write-Host "    Uninstall: & '$exe' -service uninstall -addr '$Addr' -db '$db' -api-key '$ApiKey' -downloads-dir '$downloads'"
