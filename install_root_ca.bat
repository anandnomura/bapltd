@echo off
setlocal
cd /d "%~dp0"
echo ===============================================================================
echo     BAP ROOT CA - WINDOWS CERTIFICATE STORE REGISTRATION
echo ===============================================================================
echo [*] Adding BAP Internal Root CA to CurrentUser\Root store...
echo [*] Note: If Windows displays a Security Warning dialog, click 'Yes' to trust.
echo.
if not exist "bap-root-ca.crt" (
    echo [*] Generating BAP Root CA first...
    powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\ensure_certs.ps1" 
)

certutil.exe -addstore -user Root "%~dp0bap-root-ca.crt"
if %ERRORLEVEL% equ 0 (
    echo.
    echo [+] SUCCESS: BAP Root CA installed into CurrentUser\Root store!
    echo [+] Chrome, Edge, and Windows now trust https://localhost:8443 and https://amn010731.nomura.com:8443
) else (
    echo.
    echo [-] Installation was cancelled or failed with exit code %ERRORLEVEL%.
)
echo.
pause

