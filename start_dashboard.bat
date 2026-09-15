@echo off
setlocal EnableExtensions EnableDelayedExpansion

set "BAP_ROOT=%~dp0"
cd /d "%BAP_ROOT%"

if not defined CP_PORT set "CP_PORT=8443"
if not defined DASHBOARD_PORT set "DASHBOARD_PORT=8444"
if not defined BAP_DB_PATH set "BAP_DB_PATH=%BAP_ROOT%bap-controlplane.db"
if not defined BAP_CA_CERT set "BAP_CA_CERT=%BAP_ROOT%bap-root-ca.crt"
if not defined BAP_TLS_CERT set "BAP_TLS_CERT=%BAP_ROOT%controlplane-cert.pem"
if not defined BAP_TLS_KEY set "BAP_TLS_KEY=%BAP_ROOT%controlplane-key.pem"

call :resolve_binaries
if not exist "!CP_BIN!" goto :build_required
if not exist "!DASHBOARD_BIN!" goto :build_required
goto :binaries_ready

:build_required
if not exist "%BAP_ROOT%build_binaries.bat" (
    echo [-] Required binaries are missing and build_binaries.bat is unavailable.
    exit /b 1
)
echo [*] Required binaries are missing. Running build_binaries.bat...
call "%BAP_ROOT%build_binaries.bat"
if errorlevel 1 exit /b 1
set "CP_BIN="
set "DASHBOARD_BIN="
call :resolve_binaries

:binaries_ready
if not exist "!CP_BIN!" (
    echo [-] Control-plane binary was not found after the build: !CP_BIN!
    exit /b 1
)
if not exist "!DASHBOARD_BIN!" (
    echo [-] Dashboard binary was not found after the build: !DASHBOARD_BIN!
    exit /b 1
)
for %%F in ("!BAP_CA_CERT!" "!BAP_TLS_CERT!" "!BAP_TLS_KEY!") do (
    if not exist "%%~fF" (
        echo [-] Required TLS file is missing: %%~fF
        exit /b 1
    )
)

if not defined CP_URL set "CP_URL=https://localhost:!CP_PORT!"
if not defined DASHBOARD_URL set "DASHBOARD_URL=https://localhost:!DASHBOARD_PORT!"

echo ====================================================================================================
echo   BAP CONTROL PLANE + STANDALONE DASHBOARD
echo ====================================================================================================
echo   Control plane : !CP_BIN!
echo   Control port  : !CP_PORT!
echo   Control URL   : !CP_URL!
echo   Dashboard     : !DASHBOARD_BIN!
echo   Dashboard port: !DASHBOARD_PORT!
echo   Database      : !BAP_DB_PATH!
echo.

echo [*] Restarting verified BAP listeners. Persisted state will be preserved...
call :stop_bap_listener !DASHBOARD_PORT! bapdashboard.exe
if errorlevel 1 exit /b 1
call :stop_bap_listener !CP_PORT! bapcontrolplane.exe
if errorlevel 1 exit /b 1

rem Force the control plane to generate a fresh token for this launch.
set "BAP_ADMIN_TOKEN="
echo [*] Starting HTTPS control plane...
start "BAP Control Plane !CP_PORT!" /min "!CP_BIN!" -port !CP_PORT! -https -tls-cert "!BAP_TLS_CERT!" -tls-key "!BAP_TLS_KEY!" -db "!BAP_DB_PATH!"
call :wait_for_url "!CP_URL!/api/v1/health" 30
if errorlevel 1 (
    echo [-] Control plane did not become healthy at !CP_URL!.
    exit /b 1
)

set "REMOTE_ADMIN_ARG="
if /i "!BAP_ALLOW_REMOTE_ADMIN!"=="1" set "REMOTE_ADMIN_ARG=-allow-remote-admin"
echo [+] Control plane is healthy. A fresh admin token is stored in:
echo     %BAP_ROOT%.bap-admin-token
echo [*] Starting standalone HTTPS dashboard...
start "BAP Dashboard !DASHBOARD_PORT!" /min "!DASHBOARD_BIN!" -port !DASHBOARD_PORT! -control-plane "!CP_URL!" -ca-cert "!BAP_CA_CERT!" -tls-cert "!BAP_TLS_CERT!" -tls-key "!BAP_TLS_KEY!" !REMOTE_ADMIN_ARG! %*
call :wait_for_url "!DASHBOARD_URL!/dashboard-health" 30
if errorlevel 1 (
    echo [-] Dashboard did not become healthy at !DASHBOARD_URL!.
    exit /b 1
)

echo [+] Dashboard is ready: !DASHBOARD_URL!/dashboard/
if /i not "!BAP_NO_BROWSER!"=="1" start "" "!DASHBOARD_URL!/dashboard/"
exit /b 0

:resolve_binaries
if not defined CP_BIN (
    if exist "%BAP_ROOT%dist\windows-amd64\controlplane\bapcontrolplane.exe" set "CP_BIN=%BAP_ROOT%dist\windows-amd64\controlplane\bapcontrolplane.exe"
    if not defined CP_BIN if exist "%BAP_ROOT%dist\windows-amd64\bapcontrolplane.exe" set "CP_BIN=%BAP_ROOT%dist\windows-amd64\bapcontrolplane.exe"
    if not defined CP_BIN if exist "%BAP_ROOT%..\controlplane\bapcontrolplane.exe" set "CP_BIN=%BAP_ROOT%..\controlplane\bapcontrolplane.exe"
    if not defined CP_BIN set "CP_BIN=%BAP_ROOT%bapcontrolplane.exe"
)
if not defined DASHBOARD_BIN (
    if exist "%BAP_ROOT%dist\windows-amd64\dashboard\bapdashboard.exe" set "DASHBOARD_BIN=%BAP_ROOT%dist\windows-amd64\dashboard\bapdashboard.exe"
    if not defined DASHBOARD_BIN if exist "%BAP_ROOT%dist\windows-amd64\bapdashboard.exe" set "DASHBOARD_BIN=%BAP_ROOT%dist\windows-amd64\bapdashboard.exe"
    if not defined DASHBOARD_BIN set "DASHBOARD_BIN=%BAP_ROOT%bapdashboard.exe"
)
exit /b 0

:stop_bap_listener
set "STOP_PORT=%~1"
set "EXPECTED_EXE=%~2"
powershell -NoProfile -ExecutionPolicy Bypass -Command ^
  "$port = [int]$env:STOP_PORT; $expected = $env:EXPECTED_EXE; $connections = @(Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue); $ids = @(); foreach ($connection in $connections) { if ($ids -notcontains $connection.OwningProcess) { $ids += $connection.OwningProcess } }; foreach ($id in $ids) { $process = Get-CimInstance Win32_Process -Filter ('ProcessId = ' + $id); if ($null -eq $process -or $process.Name -ine $expected) { Write-Host ('[-] Port ' + $port + ' is owned by an unrelated process: ' + $process.Name + ' (PID ' + $id + ')'); exit 20 } }; foreach ($id in $ids) { Stop-Process -Id $id -Force -ErrorAction Stop }; for ($attempt = 0; $attempt -lt 50; $attempt++) { if (-not (Get-NetTCPConnection -State Listen -LocalPort $port -ErrorAction SilentlyContinue)) { exit 0 }; Start-Sleep -Milliseconds 100 }; exit 21"
if errorlevel 21 (
    echo [-] Timed out waiting for port !STOP_PORT! to be released.
    exit /b 1
)
if errorlevel 20 exit /b 1
exit /b 0

:wait_for_url
set "WAIT_URL=%~1"
set "WAIT_ATTEMPTS=%~2"
for /l %%I in (1,1,!WAIT_ATTEMPTS!) do (
    curl.exe --silent --show-error --fail --ssl-no-revoke --cacert "!BAP_CA_CERT!" --connect-timeout 1 --max-time 2 "!WAIT_URL!" >nul 2>&1
    if not errorlevel 1 exit /b 0
    ping 127.0.0.1 -n 2 >nul
)
exit /b 1
