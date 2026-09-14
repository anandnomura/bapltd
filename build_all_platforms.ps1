# Bounded Authority Plane (BAP) - Multi-Platform Binary Builder
# Builds static, zero-dependency Go binaries for Windows, Linux, and macOS (Intel + ARM64).

param(
    [string]$DistDir = "dist",
    [switch]$Archive
)

$ErrorActionPreference = "Stop"
$rootDir = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $rootDir

$env:CGO_ENABLED = "0"
$env:GOTOOLCHAIN = "local"

$platforms = @(
    @{ Name = "windows-amd64"; OS = "windows"; Arch = "amd64"; Ext = ".exe" },
    @{ Name = "linux-amd64";   OS = "linux";   Arch = "amd64"; Ext = "" },
    @{ Name = "linux-arm64";   OS = "linux";   Arch = "arm64"; Ext = "" },
    @{ Name = "darwin-amd64";  OS = "darwin";  Arch = "amd64"; Ext = "" },
    @{ Name = "darwin-arm64";  OS = "darwin";  Arch = "arm64"; Ext = "" }
)

$components = @(
    @{ Dir = "bap-controlplane"; Pkg = "./cmd/server";            Name = "bapcontrolplane" },
    @{ Dir = "bap-edge";         Pkg = ".";                       Name = "bapedge" },
    @{ Dir = "bap-gateway";      Pkg = ".";                       Name = "bapgateway" },
    @{ Dir = "cchook";           Pkg = "interceptor.go";          Name = "cchook-interceptor" },
    @{ Dir = "copilot";          Pkg = "copilot_interceptor.go";  Name = "copilot-interceptor" }
)

Write-Host "===============================================================================" -ForegroundColor Cyan
Write-Host "   BAP MULTI-PLATFORM BINARY COMPILER" -ForegroundColor Cyan
Write-Host "   Target Platforms: Windows, Linux (amd64, arm64), macOS (Intel, Apple Silicon)" -ForegroundColor Cyan
Write-Host "===============================================================================" -ForegroundColor Cyan

$distPath = Join-Path $rootDir $DistDir
if (!(Test-Path $distPath)) {
    New-Item -ItemType Directory -Path $distPath | Out-Null
}

$startTime = Get-Date

foreach ($p in $platforms) {
    $platformDir = Join-Path $distPath $p.Name
    if (!(Test-Path $platformDir)) {
        New-Item -ItemType Directory -Path $platformDir | Out-Null
    }

    Write-Host "`n>>> Compiling for $($p.Name) (OS: $($p.OS), ARCH: $($p.Arch))..." -ForegroundColor Yellow
    $env:GOOS = $p.OS
    $env:GOARCH = $p.Arch

    foreach ($c in $components) {
        $outFile = Join-Path $platformDir "$($c.Name)$($p.Ext)"
        $compDir = Join-Path $rootDir $c.Dir

        Push-Location $compDir
        try {
            # Build with optimizations and stripped symbols for minimal footprint
            go build -ldflags "-s -w" -o $outFile $c.Pkg
            if ($LASTEXITCODE -ne 0) {
                throw "Compilation failed for $($c.Name) on $($p.Name)"
            }
            $size = (Get-Item $outFile).Length / 1MB
            Write-Host ("  [+] {0,-20} -> {1:N2} MB" -f "$($c.Name)$($p.Ext)", $size) -ForegroundColor Green
        }
        finally {
            Pop-Location
        }
    }

    # Copy config and web assets for standalone deployment
    Copy-Item (Join-Path $rootDir "bap-config.json") (Join-Path $platformDir "bap-config.json") -Force
    if (Test-Path (Join-Path $rootDir "bap-controlplane\inspector.html")) {
        Copy-Item (Join-Path $rootDir "bap-controlplane\inspector.html") (Join-Path $platformDir "inspector.html") -Force
    }

    # If windows-amd64, also sync root binaries
    if ($p.Name -eq "windows-amd64") {
        Copy-Item (Join-Path $platformDir "bapcontrolplane.exe") (Join-Path $rootDir "bapcontrolplane.exe") -Force
        Copy-Item (Join-Path $platformDir "bapedge.exe") (Join-Path $rootDir "bapedge.exe") -Force
        Copy-Item (Join-Path $platformDir "bapedge.exe") (Join-Path $rootDir "bapmcp.exe") -Force
        Copy-Item (Join-Path $platformDir "bapedge.exe") (Join-Path $rootDir "ltd-agent.exe") -Force
        Copy-Item (Join-Path $platformDir "bapgateway.exe") (Join-Path $rootDir "bapgateway.exe") -Force
        Copy-Item (Join-Path $platformDir "cchook-interceptor.exe") (Join-Path $rootDir "cchook\interceptor.exe") -Force
        Copy-Item (Join-Path $platformDir "copilot-interceptor.exe") (Join-Path $rootDir "copilot\copilot_interceptor.exe") -Force
    }

    # Create tar.gz (or zip for windows) if requested or convenience archive
    if ($Archive) {
        $archiveName = "bap-$($p.Name)"
        if ($p.OS -eq "windows") {
            $zipPath = Join-Path $distPath "$archiveName.zip"
            if (Test-Path $zipPath) { Remove-Item $zipPath -Force }
            Compress-Archive -Path "$platformDir\*" -DestinationPath $zipPath
            Write-Host "  [+] Packaged archive: $archiveName.zip" -ForegroundColor Cyan
        } else {
            $tarPath = Join-Path $distPath "$archiveName.tar.gz"
            tar -czf $tarPath -C $platformDir .
            Write-Host "  [+] Packaged archive: $archiveName.tar.gz" -ForegroundColor Cyan
        }
    }
}

# Clean environment
Remove-Item Env:\GOOS -ErrorAction SilentlyContinue
Remove-Item Env:\GOARCH -ErrorAction SilentlyContinue

$elapsed = (Get-Date) - $startTime
Write-Host "`n===============================================================================" -ForegroundColor Cyan
Write-Host ("   ALL PLATFORMS BUILT SUCCESSFULLY IN {0:N1}s" -f $elapsed.TotalSeconds) -ForegroundColor Green
Write-Host "   Output Directory: $distPath" -ForegroundColor Cyan
Write-Host "===============================================================================" -ForegroundColor Cyan
