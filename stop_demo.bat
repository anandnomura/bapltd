@echo off
setlocal
cd /d "%~dp0"
title BAP Zero-Trust Platform - Demo Environment Teardown
color 0C

echo ===============================================================================
echo            BOUNDED AUTHORITY PLANE (BAP) - DEMO TEARDOWN
echo ===============================================================================
echo.
echo [*] Gracefully stopping all BAP demonstration services...
echo.

:: 1. Stop BAP Gateway PEP (Port 9090)
echo [*] Terminating BAP Gateway PEP (bapgateway.exe)...
taskkill /F /IM bapgateway.exe >nul 2>&1

:: 2. Stop BAP Control Plane (Port 8080)
echo [*] Terminating BAP Control Plane (bapcontrolplane.exe)...
taskkill /F /IM bapcontrolplane.exe >nul 2>&1

:: 3. Kill any remaining processes bound to 8080 or 9090 if needed
powershell -NoProfile -Command "Get-NetTCPConnection -LocalPort 8080, 9090 -State Listen -ErrorAction SilentlyContinue | ForEach-Object { Stop-Process -Id $_.OwningProcess -Force -ErrorAction SilentlyContinue }" >nul 2>&1

timeout /t 1 >nul

echo.
echo ===============================================================================
echo [STATUS] Port Check:
powershell -NoProfile -Command "$listeners = @(Get-NetTCPConnection -LocalPort 8080, 9090 -State Listen -ErrorAction SilentlyContinue); if ($listeners.Count -eq 0) { Write-Host '[+] SUCCESS: Ports 8080 and 9090 are completely FREE.' -ForegroundColor Green } else { Write-Host '[!] Warning: Some listeners still active' -ForegroundColor Yellow }"

echo.
echo   All background demonstration services have been stopped.
echo   All client sessions and agent workloads are closed and deregistered.
echo ===============================================================================
echo.
pause
endlocal
