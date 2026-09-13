@echo off
setlocal

echo ===============================================================================
echo   Starting BAP Control Plane and Opening Activity Inspector Dashboard
echo ===============================================================================

:: 1. Check if bapcontrolplane is already running on port 8080
powershell -NoProfile -Command "if (Get-NetTCPConnection -LocalPort 8080 -State Listen -ErrorAction SilentlyContinue) { exit 0 } else { exit 1 }"
if %errorlevel% equ 0 (
    echo [*] bapcontrolplane is already running on http://localhost:8080.
) else (
    echo [*] Launching bapcontrolplane daemon on port 8080...
    powershell -NoProfile -Command "Start-Process -FilePath '.\bap-controlplane\bapcontrolplane.exe' -ArgumentList '-port 8080 -ttl 30 -trust-domain bap.internal' -WindowStyle Hidden"
    ping -n 3 127.0.0.1 >nul
)

echo [*] Opening Inspector Dashboard in your default browser...
start http://localhost:8080/inspector

echo.
echo ===============================================================================
echo   Inspector is LIVE at: http://localhost:8080/inspector
echo   To stop it anytime, run: stop_inspector.bat
echo ===============================================================================
endlocal
