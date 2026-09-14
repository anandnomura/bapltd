@echo off
setlocal

echo ===============================================================================
echo   Starting BAP Control Plane and Opening Activity Inspector Dashboard
echo ===============================================================================

set "CP_URL=%BAP_SERVER_URL%"
if "%CP_URL%"=="" (
    if exist "bap-config.json" (
        for /f "usebackq delims=" %%U in (`powershell -NoProfile -Command "(Get-Content bap-config.json -Raw | ConvertFrom-Json).controlplane_url"`) do set "CP_URL=%%U"
    )
)
if "%CP_URL%"=="" set CP_URL=http://localhost:8080

:: Check if CP_URL points to localhost
set "IS_LOCAL=0"
echo %CP_URL% | findstr /i "localhost 127.0.0.1 ::1" >nul 2>&1
if %errorlevel% equ 0 set "IS_LOCAL=1"

if "%IS_LOCAL%"=="1" (
    :: Local mode: check or launch local daemon
    powershell -NoProfile -Command "if (Get-NetTCPConnection -LocalPort 8080 -State Listen -ErrorAction SilentlyContinue) { exit 0 } else { exit 1 }"
    if %errorlevel% equ 0 (
        echo [*] Local bapcontrolplane is already running on %CP_URL%.
    ) else (
        echo [*] Launching local bapcontrolplane daemon on port 8080...
        powershell -NoProfile -Command "Start-Process -FilePath '.\bap-controlplane\bapcontrolplane.exe' -ArgumentList '-port 8080 -ttl 30 -trust-domain bap.internal' -WindowStyle Hidden"
        ping -n 3 127.0.0.1 >nul
    )
) else (
    echo [*] Central Control Plane configured at: %CP_URL%
    echo [*] Verifying connection to remote control plane...
    curl.exe -s --max-time 3 --connect-timeout 2 -X GET "%CP_URL%/api/v1/health" >nul 2>&1
    if %errorlevel% equ 0 (
        echo [+] Remote Control Plane is reachable and healthy.
    ) else (
        echo [!] Warning: Could not reach remote Control Plane at %CP_URL%. Opening browser anyway.
    )
)

echo [*] Opening Inspector Dashboard in your default browser...
start %CP_URL%/inspector

echo.
echo ===============================================================================
echo   Inspector is LIVE at: %CP_URL%/inspector
if "%IS_LOCAL%"=="1" (
    echo   To stop local server anytime, run: stop_inspector.bat
)
echo ===============================================================================
endlocal
