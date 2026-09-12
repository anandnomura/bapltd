@echo off
setlocal

echo ===============================================================================
echo   Starting BAP Control Plane & Opening Activity Inspector Dashboard
echo ===============================================================================

:: 1. Check if bapcontrolplane is already running on port 8080
netstat -ano | findstr :8080 >nul 2>&1
if %errorlevel% equ 0 (
    echo [*] bapcontrolplane is already running on http://localhost:8080.
) else (
    echo [*] Launching bapcontrolplane daemon on port 8080...
    if exist "bap-controlplane\bapcontrolplane.exe" (
        start "" /B "bap-controlplane\bapcontrolplane.exe" -port 8080 -ttl 30 -trust-domain bap.internal
    ) else (
        echo [!] Compiling bapcontrolplane.exe...
        cd bap-controlplane && go build -o bapcontrolplane.exe ./cmd/server && cd ..
        start "" /B "bap-controlplane\bapcontrolplane.exe" -port 8080 -ttl 30 -trust-domain bap.internal
    )
    timeout /t 2 /nobreak >nul
)

echo [*] Opening Inspector Dashboard in your default browser...
start http://localhost:8080/inspector

echo.
echo ===============================================================================
echo   Inspector is LIVE at: http://localhost:8080/inspector
echo   Press any key to exit this launcher (bapcontrolplane will continue running).
echo ===============================================================================
pause >nul
endlocal

