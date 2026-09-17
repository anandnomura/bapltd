@echo off
setlocal
cd /d "%~dp0"
title BAP Executive Command Center Demonstration

echo ===============================================================================
echo   BAP ZERO-TRUST: EXECUTIVE COMMAND CENTER LIVE FLEET DEMONSTRATION
echo ===============================================================================
echo.

if exist ".bap-controlplane-supervisor.pid" (
    echo [*] Stopping the legacy control-plane supervisor before refresh...
    powershell -NoProfile -ExecutionPolicy Bypass -Command ^
      "$pidFile = Join-Path (Get-Location) '.bap-controlplane-supervisor.pid'; $supervisorId = [int](Get-Content $pidFile -ErrorAction SilentlyContinue); if ($supervisorId -gt 0) { $process = Get-CimInstance Win32_Process -Filter ('ProcessId = ' + $supervisorId) -ErrorAction SilentlyContinue; if ($process -and $process.CommandLine -match 'supervise_controlplane\.ps1') { Stop-Process -Id $supervisorId -Force -ErrorAction SilentlyContinue } }; Remove-Item -LiteralPath $pidFile -Force -ErrorAction SilentlyContinue"
    ping 127.0.0.1 -n 2 >nul
)

echo [*] Checking whether the embedded cockpit binary is current...
powershell -NoProfile -ExecutionPolicy Bypass -Command ^
  "$root = (Get-Location).Path; $sources = @(Get-ChildItem (Join-Path $root 'dashboard\src') -File -Recurse) + @(Get-ChildItem (Join-Path $root 'bap-controlplane\internal\api') -Filter '*.go' -File -Recurse); $newest = ($sources | Sort-Object LastWriteTimeUtc -Descending | Select-Object -First 1).LastWriteTimeUtc; $bins = @((Join-Path $root 'bapcontrolplane.exe'), (Join-Path $root 'bapdashboard.exe'), (Join-Path $root 'dist\windows-amd64\controlplane\bapcontrolplane.exe'), (Join-Path $root 'dist\windows-amd64\dashboard\bapdashboard.exe')) | Where-Object { Test-Path $_ }; if ($bins.Count -lt 2 -or @($bins | Where-Object { (Get-Item $_).LastWriteTimeUtc -lt $newest }).Count -gt 0) { exit 10 }"
if errorlevel 10 (
    echo [*] Dashboard or control-plane sources are newer than the binaries. Rebuilding...
    call build_binaries.bat
    if errorlevel 1 exit /b 1
)

echo [*] Restarting the control plane and dashboard with the current cockpit...
set "BAP_DEMO_NO_BROWSER=%BAP_NO_BROWSER%"
set "BAP_NO_BROWSER=1"
set "BAP_DEMO_MODE=1"
set "CP_BIN=%~dp0bapcontrolplane.exe"
set "DASHBOARD_BIN=%~dp0bapdashboard.exe"
call start_dashboard.bat
if errorlevel 1 exit /b 1
set "BAP_NO_BROWSER=%BAP_DEMO_NO_BROWSER%"

echo [*] Opening CIO Agent Command Center in default browser...
if /i not "%BAP_DEMO_NO_BROWSER%"=="1" start https://localhost:8443/dashboard/

echo [*] Launching deterministic 3-agent executive fleet scenario...
echo [*] For the 25-agent view, click Start Demo in the cockpit command bar.
echo.
python demo_executive.py %*

if errorlevel 1 (
    echo.
    echo [-] Executive demo exited with error code %ERRORLEVEL%.
)

echo.
pause
