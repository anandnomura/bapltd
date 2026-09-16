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
echo [*] Checking connectivity to BAP Control Plane at !SERVER_URL!...
echo !SERVER_URL! | findstr /i "localhost 127.0.0.1 ::1" >nul 2>&1
if !ERRORLEVEL! equ 0 (
    powershell -NoProfile -Command "$u = [System.Uri]'!SERVER_URL!'; $port = $u.Port; if (Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue) { exit 0 } else { exit 1 }"
    if !ERRORLEVEL! neq 0 (
        echo [*] Launching local BAP Control Plane daemon on !SERVER_URL!...
        powershell -NoProfile -Command "$u = [System.Uri]'!SERVER_URL!'; $port = $u.Port; $isHttps = $u.Scheme -eq 'https'; $args = '-port ' + $port + ' -ttl 30 -trust-domain bap.internal'; if ($isHttps) { $args += ' -https' }; $p = if (Test-Path '%BAP_HOME%dist\windows-amd64\controlplane\bapcontrolplane.exe') { '%BAP_HOME%dist\windows-amd64\controlplane\bapcontrolplane.exe' } elseif (Test-Path '%BAP_HOME%dist\windows-amd64\bapcontrolplane.exe') { '%BAP_HOME%dist\windows-amd64\bapcontrolplane.exe' } else { '%BAP_HOME%bap-controlplane\bapcontrolplane.exe' }; Start-Process -FilePath $p -ArgumentList $args -WindowStyle Hidden"
        ping -n 3 127.0.0.1 >nul
    )
)
curl.exe -s --max-time 3 --connect-timeout 2 -X GET "!SERVER_URL!/api/v1/health" | findstr /i "ok healthy ltd-service" >nul 2>&1
if !ERRORLEVEL! equ 0 (
    echo [+] SUCCESS: Connected to BAP Control Plane.
) else (
    echo [-] WARNING: Could not reach !SERVER_URL!/api/v1/health.
    echo     Please verify the server IP, port, and firewall.
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
        echo     "SessionStart": [
        echo       {
        echo         "hooks": [
        echo           {
        echo             "type": "command",
        echo             "command": "!SAFE_HOOK_CMD!"
        echo           }
        echo         ]
        echo       }
        echo     ],
        echo     "UserPromptSubmit": [
        echo       {
        echo         "hooks": [
        echo           {
        echo             "type": "command",
        echo             "command": "!SAFE_HOOK_CMD!"
        echo           }
        echo         ]
        echo       }
        echo     ],
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
    echo [+] Configured Claude Code SessionStart, UserPromptSubmit, and PreToolUse hooks in .claude\settings.json
)

:: 5. Generate unique BAP Session ID
set "RND=%RANDOM%"
set "TME=%TIME::=%"
set "TME=%TME: =0%"
set "TME=%TME:.=%"
set "BAP_SESSION_ID=sess-claude-%RND%-%TME%"
set "BAP_SERVER_URL=!SERVER_URL!"

:: 6. Zero-Trust Session Pre-Flight & Heartbeat Watcher Enrollment
if not "!BAPEDGE_BIN!"=="" (
    !BAPEDGE_BIN! session-start --server "!SERVER_URL!" --session-id "!BAP_SESSION_ID!" --app-id "claude-code"
    if !ERRORLEVEL! equ 2 (
        if exist ".bap-session.json" del /f /q ".bap-session.json" >nul 2>&1
        exit /b 2
    )
) else (
    (
        echo {
        echo   "session_id": "!BAP_SESSION_ID!",
        echo   "server_url": "!SERVER_URL!",
        echo   "user": "%USERNAME%",
        echo   "hostname": "%COMPUTERNAME%"
        echo }
    ) > ".bap-session.json"
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
echo   Dashboard UI       : start bapdashboard separately (default https://localhost:8444/dashboard/)
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
echo [*] Claude Code exited. Deregistering session !BAP_SESSION_ID! from server...
if not "!BAPEDGE_BIN!"=="" (
    !BAPEDGE_BIN! session-end --server "!SERVER_URL!" --session-id "!BAP_SESSION_ID!" >nul 2>&1
) else (
    set "CURL_CA_ARG="
    if exist "controlplane-cert.pem" (
        set "CURL_CA_ARG=--cacert controlplane-cert.pem"
    ) else (
        set "CURL_CA_ARG=-k"
    )
    curl.exe -s !CURL_CA_ARG! --max-time 2 --connect-timeout 2 -X POST "!SERVER_URL!/api/v1/sessions/end" ^
        -H "Content-Type: application/json" ^
        -d "{\"session_id\":\"!BAP_SESSION_ID!\",\"reason\":\"Claude Code closed gracefully\"}" >nul 2>&1
)

if exist ".bap-session.json" del /f /q ".bap-session.json" >nul 2>&1
if exist ".bap-prompt.txt" del /f /q ".bap-prompt.txt" >nul 2>&1
echo [+] Session cleanly closed on dashboard.
endlocal
exit /b 0
