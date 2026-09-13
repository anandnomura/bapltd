<#
.SYNOPSIS
    Configures Central BAP endpoints (Control Plane & Gateway) for developer laptops.
.EXAMPLE
    .\configure_endpoints.ps1 -ControlPlane "https://bap-controlplane.corp.internal:8080"
    .\configure_endpoints.ps1 -Global
#>

param(
    [string]$ControlPlane,
    [string]$Gateway,
    [string]$Envoy,
    [switch]$Global
)

$targetFile = "bap-config.json"
if ($Global) {
    $homeDir = [System.Environment]::GetFolderPath('UserProfile')
    $bapDir = Join-Path $homeDir ".bap"
    if (!(Test-Path $bapDir)) { New-Item -ItemType Directory -Path $bapDir -Force | Out-Null }
    $targetFile = Join-Path $bapDir "config.json"
}

Write-Host "===============================================================================" -ForegroundColor Cyan
Write-Host "   BAP CENTRAL ENDPOINT CONFIGURATION (POWERSHELL)" -ForegroundColor Cyan
Write-Host "===============================================================================" -ForegroundColor Cyan

if (Test-Path ".\bapedge.exe") {
    .\bapedge.exe config show
}

if (-not $ControlPlane) {
    Write-Host "`nEnter the corporate BAP Control Plane URL (e.g. https://bap-controlplane.corp.internal:8080):" -ForegroundColor Yellow
    $ControlPlane = Read-Host "Control Plane URL"
}

if ([string]::IsNullOrWhiteSpace($ControlPlane)) {
    Write-Host "[-] No URL provided. Existing configuration preserved." -ForegroundColor Gray
    exit 0
}

$bapedge = ".\bapedge.exe"
if (Test-Path $bapedge) {
    $argsList = @("config", "set", "--server", $ControlPlane)
    if ($Gateway) { $argsList += @("--gateway", $Gateway) }
    if ($Envoy) { $argsList += @("--envoy", $Envoy) }
    if ($Global) { $argsList += @("--global") }
    & $bapedge @argsList
} else {
    $cfg = @{
        controlplane_url = $ControlPlane.TrimEnd('/')
        gateway_url      = if ($Gateway) { $Gateway.TrimEnd('/') } else { "http://localhost:9090" }
        envoy_url        = if ($Envoy) { $Envoy.TrimEnd('/') } else { "http://localhost:10000" }
        trust_domain     = "bap.internal"
        environment      = "production"
    }
    $cfg | ConvertTo-Json -Depth 5 | Set-Content -Path $targetFile -Encoding UTF8
    Write-Host "[+] Endpoints saved directly to $targetFile" -ForegroundColor Green
}

Write-Host "`nAll Claude Code, Copilot, and Python agents will now connect to $ControlPlane" -ForegroundColor Green

