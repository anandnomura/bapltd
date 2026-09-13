@echo off
setlocal

:: ==============================================================================
:: BAP - Option 1 Standalone Demo Runner (Windows)
:: Launches Envoy on Podman/Docker (if not already running) and executes demo_envoy.py
:: ==============================================================================

cd /d "%~dp0"

echo ===============================================================================
echo   BAP OPTION 1: STANDALONE ENVOY PROXY ON PODMAN / DOCKER DEMO
echo ===============================================================================

:: 1. Check if bapcontrolplane is running on port 8080
powershell -NoProfile -Command "try { $r = Invoke-WebRequest -Uri 'http://localhost:8080/api/v1/health' -TimeoutSec 2 -UseBasicParsing; exit 0 } catch { exit 1 }" >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo [*] Starting bapcontrolplane.exe in background...
    start /b "" "..\bapcontrolplane.exe" -port 8080
    timeout /t 2 /nobreak >nul 2>&1
)

:: 2. Check if Envoy is listening on port 10000
powershell -NoProfile -Command "try { $r = Invoke-WebRequest -Uri 'http://localhost:10000/api/v1/health' -TimeoutSec 1 -UseBasicParsing; exit 0 } catch { exit 1 }" >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo [*] Envoy is not running on port 10000. Launching via Podman/Docker...
    call run_envoy_podman.bat
    if %ERRORLEVEL% neq 0 (
        echo [!] Failed to start Envoy container. Aborting demo.
        pause
        exit /b 1
    )
    timeout /t 2 /nobreak >nul 2>&1
)

:: 3. Run demo_envoy.py
python demo_envoy.py

pause

