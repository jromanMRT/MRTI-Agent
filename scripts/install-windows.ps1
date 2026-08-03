<#
.SYNOPSIS
  Install the locally built MRTI Agent as a Windows service.
.EXAMPLE
  .\scripts\install-windows.ps1 -Config .\config.yaml
#>
param(
	[string]$Binary = "",
	[string]$Config = "",
	[string]$InstallDir = "$env:ProgramFiles\MRTI Agent"
)

$ErrorActionPreference = "Stop"
$RepoRoot = Split-Path $PSScriptRoot -Parent

if ([string]::IsNullOrWhiteSpace($Binary)) {
	$Binary = Join-Path $RepoRoot "dist\windows-amd64\mrti-agent.exe"
}
if ([string]::IsNullOrWhiteSpace($Config)) {
	$LocalConfig = Join-Path $RepoRoot "config.yaml"
	if (Test-Path -LiteralPath $LocalConfig -PathType Leaf) {
		$Config = $LocalConfig
	} else {
		$Config = Join-Path $RepoRoot "config.yaml.example"
	}
}

$Installer = Join-Path $RepoRoot "packaging\windows\install-windows.ps1"
& $Installer -Binary $Binary -Config $Config -InstallDir $InstallDir
