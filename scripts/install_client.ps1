# BAP Client One-Click Developer Installer
param(
    [string]$TargetDir = (Join-Path $env:USERPROFILE "bin"),
    [switch]$Force = $false
)

$ErrorActionPreference = 'Stop'
$repoRoot = (Get-Item -Path $PSScriptRoot).Parent.FullName

Write-Host "===============================================================================" -ForegroundColor Cyan
Write-Host "      BAP CLIENT - ONE-CLICK DEVELOPER ONBOARDING INSTALLER                    " -ForegroundColor Cyan
Write-Host "===============================================================================" -ForegroundColor Cyan
Write-Host "[*] Target Installation Directory: $TargetDir"

if (-not (Test-Path -LiteralPath $TargetDir)) {
    New-Item -ItemType Directory -Path $TargetDir -Force | Out-Null
    Write-Host "[+] Created directory: $TargetDir" -ForegroundColor Green
}

$sourceDir = Join-Path $repoRoot "dist\windows-amd64\claude-client"
if (-not (Test-Path -LiteralPath $sourceDir)) {
    $sourceDir = $repoRoot
}

$clientFiles = @(
    "bapedge.exe",
    "interceptor.exe",
    "run_claude_bap.bat",
    "run_claude_bap.ps1",
    "bap-config.json",
    "policy.cedar",
    "schema.json"
)

$installedCount = 0
foreach ($f in $clientFiles) {
    $src = Join-Path $sourceDir $f
    if (-not (Test-Path -LiteralPath $src)) {
        $src = Join-Path $repoRoot $f
    }

    if (Test-Path -LiteralPath $src) {
        $dst = Join-Path $TargetDir $f
        Copy-Item -LiteralPath $src -Destination $dst -Force
        $size = (Get-Item -LiteralPath $dst).Length
        Write-Host ("  [+] Installed: {0,-22} ({1:N0} bytes)" -f $f, $size) -ForegroundColor Green
        $installedCount++
    } else {
        Write-Warning "Source file not found: $f"
    }
}

Write-Host "`n[*] Configuring Environment PATH..."
$userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
if ([string]::IsNullOrWhiteSpace($userPath)) {
    $userPath = ""
}

$pathParts = $userPath.Split(';') | ForEach-Object { $_.Trim().TrimEnd('\') } | Where-Object { -not [string]::IsNullOrWhiteSpace($_) }
$targetNormalized = $TargetDir.Trim().TrimEnd('\')

if ($pathParts -contains $targetNormalized) {
    Write-Host "[+] User PATH already contains: $TargetDir" -ForegroundColor Green
} else {
    $newPath = if ([string]::IsNullOrWhiteSpace($userPath)) { $TargetDir } else { "$userPath;$TargetDir" }
    [Environment]::SetEnvironmentVariable("PATH", $newPath, "User")
    Write-Host "[+] Added to User PATH: $TargetDir" -ForegroundColor Green
    Write-Host "    (Changes will apply to all new terminal sessions)" -ForegroundColor Gray
}

# Update current session PATH so it can be used immediately
if ($env:PATH -notmatch [regex]::Escape($targetNormalized)) {
    $env:PATH = "$env:PATH;$TargetDir"
}

Write-Host "`n===============================================================================" -ForegroundColor Cyan
Write-Host "  INSTALLATION SUCCESSFUL! ($installedCount files deployed)                   " -ForegroundColor Green
Write-Host "===============================================================================" -ForegroundColor Cyan
Write-Host "You can now run Claude Code governed by BAP from ANY directory:"
Write-Host "  > run_claude_bap" -ForegroundColor Yellow
Write-Host "  > run_claude_bap --version" -ForegroundColor Yellow
Write-Host "===============================================================================" -ForegroundColor Cyan
