@echo off
setlocal

echo ===============================================================================
echo   Stopping BAP Control Plane Daemon
echo ===============================================================================

taskkill /F /IM bapcontrolplane.exe >nul 2>&1
if %errorlevel% equ 0 (
    echo [+] Terminated bapcontrolplane.exe process.
)



powershell -NoProfile -Command "Get-NetTCPConnection -LocalPort 8080 -State Listen -ErrorAction SilentlyContinue | ForEach-Object { Stop-Process -Id $_.OwningProcess -Force -ErrorAction SilentlyContinue }" >nul 2>&1

powershell -NoProfile -Command "if (Get-NetTCPConnection -LocalPort 8080 -State Listen -ErrorAction SilentlyContinue) { exit 1 } else { exit 0 }"
if %errorlevel% equ 0 (
    echo [+] Port 8080 is now completely released.
    echo [+] BAP Control Plane and Inspector have been stopped.
) else (
    echo [!] Warning: A process is still listening on port 8080.
)

echo ===============================================================================
endlocal

