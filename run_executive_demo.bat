@echo off
setlocal
cd /d "%~dp0"
title BAP Executive Command Center Demonstration

echo ===============================================================================
echo   BAP ZERO-TRUST: EXECUTIVE COMMAND CENTER LIVE FLEET DEMONSTRATION
echo ===============================================================================
echo.
echo [*] Checking Control Plane availability at https://localhost:8443/api/v1/health...
python -c "import urllib.request, ssl, sys; ctx=ssl._create_unverified_context(); (sys.exit(0) if urllib.request.urlopen('https://localhost:8443/api/v1/health', context=ctx, timeout=2).getcode()==200 else sys.exit(1))" 2>nul

if errorlevel 1 (
    echo [*] Control plane is not running. Starting background supervisor...
    start "BAP Control Plane Supervisor" /min cmd /c "start_controlplane_supervisor.bat"
    echo [*] Waiting 3 seconds for control plane startup...
    timeout /t 3 /nobreak >nul
)

echo [*] Opening CIO Agent Command Center in default browser...
start https://localhost:8443/dashboard/

echo [*] Launching deterministic 3-agent executive fleet scenario...
echo.
python demo_executive.py %*

if errorlevel 1 (
    echo.
    echo [-] Executive demo exited with error code %ERRORLEVEL%.
)

echo.
pause

