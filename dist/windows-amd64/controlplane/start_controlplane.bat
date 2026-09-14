@echo off
cd /d "%~dp0"
echo ===============================================================================
echo   BAP CONTROL PLANE - CENTRAL SECURITY GOVERNANCE SERVER
echo ===============================================================================
echo [*] Starting BAP Control Plane on port 8080...
echo [*] Open browser dashboard: http://localhost:8080/inspector?mode=live
echo.
bapcontrolplane.exe %*
