@echo off
setlocal
echo ===============================================================================
echo   Simulating Live Agent Connection to BAP Control Plane ^& Inspector
echo ===============================================================================

set "SERVER_URL=%BAP_SERVER_URL%"
if "%SERVER_URL%"=="" (
    if exist "bap-config.json" (
        for /f "usebackq delims=" %%U in (`powershell -NoProfile -Command "(Get-Content bap-config.json -Raw | ConvertFrom-Json).controlplane_url"`) do set "SERVER_URL=%%U"
    )
)
if "%SERVER_URL%"=="" set SERVER_URL=http://localhost:8080

set RND=%RANDOM%
set SESS_ID=sess-claude-live-%RND%
set AGENT_PID=%RANDOM%

echo [*] Target Control Plane: %SERVER_URL%
echo [*] 1. Enrolling live agent workload with BAP Control Plane...
curl.exe -s -X POST "%SERVER_URL%/api/v1/sessions/start" ^
    -H "Content-Type: application/json" ^
    -d "{\"session_id\":\"%SESS_ID%\",\"app_id\":\"claude-code\",\"client_pid\":%AGENT_PID%,\"hostname\":\"developer-laptop\"}" >nul

echo [+] Connected! Session: %SESS_ID% (PID: %AGENT_PID%)
echo [*] Watch the Inspector UI (%SERVER_URL%/inspector) - you should see:
echo     - Green "1 ACTIVE AGENT" beacon on Live Workload Radar
echo     - Floating toast: "NEW WORKLOAD ENROLLED (LIVE)"
echo     - "Claude Code [PID: %AGENT_PID%]" glowing radar chip
echo.
ping -n 3 127.0.0.1 >nul

echo [*] 2. Agent invoking permitted tool call: 'git status'...
bapedge.exe exec --source claude-code --session-id %SESS_ID% --server %SERVER_URL% --raw git status
echo.
ping -n 3 127.0.0.1 >nul

echo [*] 3. Agent attempting forbidden credential exfiltration: 'cat .env'...
bapedge.exe exec --source claude-code --session-id %SESS_ID% --server %SERVER_URL% --raw cat .env
echo.
ping -n 3 127.0.0.1 >nul

echo [*] 4. Agent invoking permitted runtime check: 'python --version'...
bapedge.exe exec --source claude-code --session-id %SESS_ID% --server %SERVER_URL% --raw python --version
echo.
ping -n 3 127.0.0.1 >nul

echo [*] 5. Agent terminating session gracefully...
curl.exe -s -X POST "%SERVER_URL%/api/v1/sessions/end" ^
    -H "Content-Type: application/json" ^
    -d "{\"session_id\":\"%SESS_ID%\",\"reason\":\"normal shutdown\"}" >nul

echo.
echo [+] Session %SESS_ID% closed. Inspector will show toast and transition to CLOSED.
echo ===============================================================================
endlocal
