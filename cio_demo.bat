@echo off
setlocal
title BAP Zero-Trust Platform - CIO Demo Launcher
color 0F

echo ===============================================================================
echo            BOUNDED AUTHORITY PLANE (BAP) - ONE-CLICK CIO DEMO
echo ===============================================================================
echo.
echo [*] Preparing Executive Demonstration Environment...
echo.

:: 0. Initialize fresh, pristine audit trail
type nul > ltd-audit.jsonl

:: 1. Ensure bapcontrolplane is running on port 8080
powershell -NoProfile -Command "if (Get-NetTCPConnection -LocalPort 8080 -State Listen -ErrorAction SilentlyContinue) { exit 0 } else { exit 1 }"
if %errorlevel% equ 0 (
    echo [+] BAP Control Plane is already active on http://localhost:8080
) else (
    echo [*] Starting BAP Control Plane daemon on port 8080...
    powershell -NoProfile -Command "Start-Process -FilePath '.\bap-controlplane\bapcontrolplane.exe' -ArgumentList '-port 8080 -ttl 30 -trust-domain bap.internal' -WindowStyle Hidden"
    ping -n 3 127.0.0.1 >nul
    echo [+] Control Plane started successfully!
)

:: 2. Launch Activity Inspector in default browser (in live mode)
echo [*] Opening BAP Activity Inspector & Live Fleet Radar in browser...
start http://localhost:8080/inspector?mode=live

:: 3. Launch Interactive Demonstration Console in a styled side-by-side terminal
echo [*] Opening Interactive CIO Presentation Console...
start "BAP Zero-Trust - CIO Presentation Console" cmd.exe /c demo_cio_interactive.bat

echo.
echo ===============================================================================
echo   SUCCESS! All demo windows are now open:
echo.
echo   1. Browser Window:  BAP Activity Inspector & Live Radar (Live Mode)
echo   2. Terminal Window: Interactive Step-by-Step CIO Presentation Console
echo.
echo   Tip for the presentation:
echo   Position the browser on the left half of your screen and the terminal on the
echo   right half to deliver a stunning dual-screen visual experience to your CIO.
echo ===============================================================================
echo.
timeout /t 5 >nul
endlocal
