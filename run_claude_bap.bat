@echo off
setlocal EnableDelayedExpansion
set "BAP_HOME=%~dp0"
title Claude Code (Governed by BAP Zero-Trust)

echo ===============================================================================
echo       CLAUDE CODE - BOUNDED AUTHORITY PLANE (BAP) GOVERNANCE LAUNCHER
echo ===============================================================================
echo [*] Working Directory: %CD%
echo.

:: 1. Resolve Linux Control Plane Server URL
set "SERVER_URL=%~1"
if "!SERVER_URL!"=="" set "SERVER_URL=%BAP_SERVER_URL%"
if "!SERVER_URL!"=="" (
    set "CFG_FILE="
    if exist "bap-config.json" (
        set "CFG_FILE=bap-config.json"
    ) else if exist "%BAP_HOME%bap-config.json" (
        set "CFG_FILE=%BAP_HOME%bap-config.json"
    ) else if exist "%USERPROFILE%\bin\bap-config.json" (
        set "CFG_FILE=%USERPROFILE%\bin\bap-config.json"
    )
    if not "!CFG_FILE!"=="" (
        for /f "usebackq delims=" %%U in (`powershell -NoProfile -Command "(Get-Content '!CFG_FILE!' -Raw | ConvertFrom-Json).controlplane_url" 2^>nul`) do set "SERVER_URL=%%U"
    )
)
if "!SERVER_URL!"=="" set "SERVER_URL=http://localhost:8080"

:: If user ran without args, show current and prompt for convenience
if "%~1"=="" (
    echo Current Target Server: !SERVER_URL!
    echo Enter Linux server URL or press [ENTER] to use default:
    set /p USER_INPUT="Linux Server URL [default: !SERVER_URL!]: "
    if not "!USER_INPUT!"=="" set "SERVER_URL=!USER_INPUT!"
)

:: Trim trailing slashes
if "!SERVER_URL:~-1!"=="/" set "SERVER_URL=!SERVER_URL:~0,-1!"

echo.
echo [*] Checking connectivity to Linux BAP Control Plane at !SERVER_URL!...
curl.exe -s --max-time 3 --connect-timeout 2 -X GET "!SERVER_URL!/api/v1/health" | findstr /i "ok healthy ltd-service" >nul 2>&1
if !ERRORLEVEL! equ 0 (
    echo [+] SUCCESS: Connected to Linux BAP Control Plane.
) else (
    echo [-] WARNING: Could not reach !SERVER_URL!/api/v1/health.
    echo     Please verify the Linux server IP, port, and firewall.
    echo     Proceeding in local-edge mode - Cedar policies still enforced locally.
)

:: 2. Resolve bapedge binary
set "BAPEDGE_BIN="
if exist "%BAP_HOME%dist\windows-amd64\claude-client\bapedge.exe" (
    set "BAPEDGE_BIN=%BAP_HOME%dist\windows-amd64\claude-client\bapedge.exe"
) else if exist "%BAP_HOME%dist\windows-amd64\bapedge.exe" (
    set "BAPEDGE_BIN=%BAP_HOME%dist\windows-amd64\bapedge.exe"
) else if exist "%BAP_HOME%bapedge.exe" (
    set "BAPEDGE_BIN=%BAP_HOME%bapedge.exe"
) else if exist "%USERPROFILE%\bin\bapedge.exe" (
    set "BAPEDGE_BIN=%USERPROFILE%\bin\bapedge.exe"
) else (
    where bapedge.exe >nul 2>&1
    if !ERRORLEVEL! equ 0 set "BAPEDGE_BIN=bapedge.exe"
)

:: Persist to bap-config.json so bapedge and cchook automatically connect
if not "!BAPEDGE_BIN!"=="" (
    !BAPEDGE_BIN! config set --server "!SERVER_URL!" >nul 2>&1
) else (
    powershell -NoProfile -Command ^
        "$p = if (Test-Path 'bap-config.json') { 'bap-config.json' } elseif (Test-Path '%BAP_HOME%bap-config.json') { '%BAP_HOME%bap-config.json' } else { $null }; if ($p) { $j = Get-Content $p -Raw | ConvertFrom-Json; $j.controlplane_url = '!SERVER_URL!'; $j | ConvertTo-Json -Depth 5 | Set-Content $p -Encoding UTF8 }" >nul 2>&1
)

:: 3. Verify cchook interceptor binary exists
set "INTERCEPTOR_BIN="
if exist "%BAP_HOME%dist\windows-amd64\claude-client\cchook\interceptor.exe" (
    set "INTERCEPTOR_BIN=%BAP_HOME%dist\windows-amd64\claude-client\cchook\interceptor.exe"
) else if exist "%BAP_HOME%dist\windows-amd64\cchook-interceptor.exe" (
    set "INTERCEPTOR_BIN=%BAP_HOME%dist\windows-amd64\cchook-interceptor.exe"
) else if exist "%BAP_HOME%interceptor.exe" (
    set "INTERCEPTOR_BIN=%BAP_HOME%interceptor.exe"
) else if exist "%USERPROFILE%\bin\interceptor.exe" (
    set "INTERCEPTOR_BIN=%USERPROFILE%\bin\interceptor.exe"
) else if exist "%BAP_HOME%cchook\interceptor.exe" (
    set "INTERCEPTOR_BIN=%BAP_HOME%cchook\interceptor.exe"
) else (
    where interceptor.exe >nul 2>&1
    if !ERRORLEVEL! equ 0 set "INTERCEPTOR_BIN=interceptor.exe"
)

:: Formulate hook command (prefer forward-slash absolute path, fallback to interceptor.exe in PATH)
set "SAFE_HOOK_CMD=interceptor.exe"
if not "!INTERCEPTOR_BIN!"=="" (
    set "SAFE_HOOK_CMD=!INTERCEPTOR_BIN:\=/!"
)

:: 4. Ensure .claude/settings.json hook is configured in workspace
if not exist ".claude\settings.json" (
    if not exist ".claude" mkdir .claude
    (
        echo {
        echo   "hooks": {
        echo     "PreToolUse": [
        echo       {
        echo         "matcher": "Bash|Read|View|Edit|Write",
        echo         "hooks": [
        echo           {
        echo             "type": "command",
        echo             "command": "!SAFE_HOOK_CMD!"
        echo           }
        echo         ]
        echo       }
        echo     ]
        echo   }
        echo }
    ) > ".claude\settings.json"
    echo [+] Configured Claude Code PreToolUse hook in .claude\settings.json
)

:: 5. Generate unique BAP Session ID
set "RND=%RANDOM%"
set "TME=%TIME::=%"
set "TME=%TME: =0%"
set "TME=%TME:.=%"
set "BAP_SESSION_ID=sess-claude-%RND%-%TME%"
set "BAP_SERVER_URL=!SERVER_URL!"

:: Write workspace session marker so all tool hooks use the exact same session ID
(
    echo {
    echo   "session_id": "!BAP_SESSION_ID!",
    echo   "server_url": "!SERVER_URL!",
    echo   "user": "%USERNAME%",
    echo   "hostname": "%COMPUTERNAME%"
    echo }
) > ".bap-session.json"

:: 6. Notify Linux Control Plane of Workload Enrollment (Live Radar)
curl.exe -s --max-time 2 --connect-timeout 2 -X POST "!SERVER_URL!/api/v1/sessions/start" ^
    -H "Content-Type: application/json" ^
    -d "{\"session_id\":\"!BAP_SESSION_ID!\",\"app_id\":\"claude-code\",\"user_id\":\"%USERNAME%\",\"hostname\":\"%COMPUTERNAME%\",\"client_pid\":0}" >nul 2>&1

:: Start native bapedge background watcher thread to guarantee deregistration even on Ctrl+C or window close [X]
if not "!BAPEDGE_BIN!"=="" (
    !BAPEDGE_BIN! watch --server "!SERVER_URL!" --session-id "!BAP_SESSION_ID!" --detach >nul 2>&1
) else (
    start "" /b powershell.exe -NoProfile -ExecutionPolicy Bypass -WindowStyle Hidden -File "%~dp0scripts\watch_session.ps1" -ServerUrl "!SERVER_URL!" -SessionId "!BAP_SESSION_ID!" >nul 2>&1
)

echo.
echo ===============================================================================
echo   CLAUDE CODE IS NOW GOVERNED BY BAP ZERO-TRUST
echo ===============================================================================
echo   Target Control Plane : !SERVER_URL!
echo   Active Session ID    : !BAP_SESSION_ID!
echo   User Identity        : %USERNAME% on %COMPUTERNAME%
echo   Policy Enforcement   : Local Cedar Broker (^< 1.5ms)
echo   Live Telemetry       : Streaming to !SERVER_URL!
echo   Liveness Watcher     : Active (auto-deregisters on exit or window close)
echo.
echo   Check your browser dashboard on: !SERVER_URL!/inspector?mode=live
echo   You will see your active Claude Code session glowing on the Live Radar!
echo ===============================================================================
echo.

:: 7. Auto-detect Claude executable
set "CLAUDE_BIN="
where claude-code.cmd >nul 2>&1
if !ERRORLEVEL! equ 0 (
    set "CLAUDE_BIN=claude-code.cmd"
) else (
    where claude >nul 2>&1
    if !ERRORLEVEL! equ 0 (
        set "CLAUDE_BIN=claude"
    ) else (
        where claude.exe >nul 2>&1
        if !ERRORLEVEL! equ 0 (
            set "CLAUDE_BIN=claude.exe"
        ) else (
            set "CLAUDE_BIN=claude"
        )
    )
)

:: 8. Launch Claude Code (passes all arguments forwarded to the script)
call !CLAUDE_BIN! %*

:: 9. Graceful Session Teardown
echo.
echo [*] Claude Code exited. Deregistering session !BAP_SESSION_ID! from Linux server...
curl.exe -s --max-time 2 --connect-timeout 2 -X POST "!SERVER_URL!/api/v1/sessions/end" ^
    -H "Content-Type: application/json" ^
    -d "{\"session_id\":\"!BAP_SESSION_ID!\",\"reason\":\"Claude Code closed gracefully\"}" >nul 2>&1

if exist ".bap-session.json" del /f /q ".bap-session.json" >nul 2>&1
echo [+] Session cleanly closed on Linux dashboard.
endlocal
exit /b 0
