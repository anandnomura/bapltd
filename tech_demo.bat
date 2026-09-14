@echo off
setlocal
cd /d "%~dp0"
title BAP Zero-Trust Platform - Technical Architecture Demo Launcher
color 0F

echo ===============================================================================
echo            BOUNDED AUTHORITY PLANE (BAP) - TECHNICAL ARCHITECTURE DEMO
echo ===============================================================================
echo.
echo [*] Preparing Technical Diligence Demonstration Environment...
echo.

:: 0. Initialize fresh, pristine audit trail and reset stale sessions
type nul > ltd-audit.jsonl
curl.exe -s --max-time 2 --connect-timeout 2 -X POST "http://localhost:8080/api/v1/sessions/reset" >nul 2>&1

:: 1. Ensure bapcontrolplane is running on port 8080
powershell -NoProfile -Command "if (Get-NetTCPConnection -LocalPort 8080 -State Listen -ErrorAction SilentlyContinue) { exit 0 } else { exit 1 }"
if %errorlevel% equ 0 (
    echo [+] BAP Control Plane is already active on http://localhost:8080
    curl.exe -s --max-time 2 --connect-timeout 2 -X POST "http://localhost:8080/api/v1/sessions/reset" >nul 2>&1
) else (
    echo [*] Starting BAP Control Plane daemon on port 8080...
    start "BAP Control Plane (8080)" /min "%~dp0bap-controlplane\bapcontrolplane.exe" -port 8080 -ttl 30 -trust-domain bap.internal
    ping -n 3 127.0.0.1 >nul
    echo [+] Control Plane started successfully!
)

:: 2. Ensure bapgateway is running on port 9090 (Gateway PEP for Rogue Agent Defense)
powershell -NoProfile -Command "if (Get-NetTCPConnection -LocalPort 9090 -State Listen -ErrorAction SilentlyContinue) { exit 0 } else { exit 1 }"
if %errorlevel% equ 0 (
    echo [+] BAP Gateway PEP is already active on http://localhost:9090
) else (
    echo [*] Starting BAP Gateway PEP daemon on port 9090...
    start "BAP Gateway PEP (9090)" /min "%~dp0bapgateway.exe" -port 9090
    ping -n 2 127.0.0.1 >nul
    echo [+] Gateway PEP started successfully!
)

:: 3. Launch Activity Inspector in default browser (in live mode)
echo [*] Opening BAP Activity Inspector & Live Fleet Radar in browser...
start http://localhost:8080/inspector?mode=live

:: 4. Launch Interactive Demonstration Console in a styled side-by-side terminal
echo [*] Opening Interactive Technical Diligence Presentation Console...
start "BAP Zero-Trust - Technical Presentation Console" cmd.exe /k demo_tech_interactive.bat

echo.
echo ===============================================================================
echo   SUCCESS! All demo windows are now open:
echo.
echo   1. Browser Window:  BAP Activity Inspector & Live Radar (Live Mode)
echo   2. Terminal Window: Interactive Step-by-Step Technical Console
echo.
echo   Tip for the presentation:
echo   Position the browser on the left half of your screen and the terminal on the
echo   right half to deliver a stunning dual-screen visual experience to your CIO.
echo ===============================================================================
echo.
timeout /t 5 >nul
endlocal
