@echo off
setlocal EnableDelayedExpansion

echo ===============================================================================
echo   Launching Claude Code with Local Ollama ^& BAP Edge Interceptor
echo ===============================================================================

:: 1. Set Anthropic API redirection to local Ollama instance
set ANTHROPIC_BASE_URL=http://localhost:11434
set ANTHROPIC_API_KEY=ollama

:: 2. Target the local Ollama model
set OLLAMA_MODEL=claude-3-5-sonnet-20241022:latest

:: 3. Generate unique BAP Session ID
set RND=%RANDOM%
set TME=%TIME::=%
set TME=%TME: =0%
set TME=%TME:.=%
set BAP_SESSION_ID=sess-claude-%RND%-%TME%

if "%BAP_SERVER_URL%"=="" (
    if exist "bap-config.json" (
        for /f "usebackq delims=" %%U in (`powershell -NoProfile -Command "(Get-Content bap-config.json -Raw | ConvertFrom-Json).controlplane_url"`) do set "BAP_SERVER_URL=%%U"
    )
)
if "%BAP_SERVER_URL%"=="" set BAP_SERVER_URL=http://localhost:8080

:: 4. Resolve and build cchook interceptor
if not exist "cchook\interceptor.exe" (
    echo [*] Building cchook\interceptor.exe...
    cd cchook && go build -o interceptor.exe interceptor.go && cd ..
)

:: Ensure .claude/settings.json hooks are configured
if not exist ".claude" mkdir .claude
(
    echo {
    echo   "hooks": {
    echo     "SessionStart": [
    echo       {
    echo         "hooks": [
    echo           {
    echo             "type": "command",
    echo             "command": "./cchook/interceptor.exe"
    echo           }
    echo         ]
    echo       }
    echo     ],
    echo     "UserPromptSubmit": [
    echo       {
    echo         "hooks": [
    echo           {
    echo             "type": "command",
    echo             "command": "./cchook/interceptor.exe"
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
    echo             "command": "./cchook/interceptor.exe"
    echo           }
    echo         ]
    echo       }
    echo     ]
    echo   }
    echo }
) > ".claude\settings.json"

:: Capture prompt if passed on command line
if not "%~1"=="" (
    set "BAP_USER_PROMPT=%~1"
    echo %~1> ".bap-prompt.txt"
)

:: 5. Write workspace session marker
(
    echo {
    echo   "session_id": "%BAP_SESSION_ID%",
    echo   "server_url": "%BAP_SERVER_URL%",
    echo   "user": "%USERNAME%",
    echo   "hostname": "%COMPUTERNAME%",
    echo   "user_prompt": "%BAP_USER_PROMPT%"
    echo }
) > ".bap-session.json"

:: Resolve bapedge binary for continuous background heartbeat watcher
set "BAPEDGE_BIN="
if exist "dist\windows-amd64\claude-client\bapedge.exe" (
    set "BAPEDGE_BIN=dist\windows-amd64\claude-client\bapedge.exe"
) else if exist "dist\windows-amd64\bapedge.exe" (
    set "BAPEDGE_BIN=dist\windows-amd64\bapedge.exe"
) else if exist "bapedge.exe" (
    set "BAPEDGE_BIN=bapedge.exe"
) else (
    where bapedge.exe >nul 2>&1
    if !ERRORLEVEL! equ 0 set "BAPEDGE_BIN=bapedge.exe"
)

:: Start continuous background heartbeat watcher thread
if not "!BAPEDGE_BIN!"=="" (
    !BAPEDGE_BIN! watch --server "%BAP_SERVER_URL%" --session-id "%BAP_SESSION_ID%" --detach >nul 2>&1
)

:: 6. Notify BAP Control Plane of Session Start (fail-securely ignored if offline)
curl.exe -s --max-time 2 --connect-timeout 2 -X POST "%BAP_SERVER_URL%/api/v1/sessions/start" ^
    -H "Content-Type: application/json" ^
    -d "{\"session_id\":\"%BAP_SESSION_ID%\",\"app_id\":\"claude-code\",\"user_id\":\"%USERNAME%\",\"hostname\":\"%COMPUTERNAME%\",\"user_prompt\":\"%BAP_USER_PROMPT%\"}" >nul 2>&1

echo [*] Ollama Endpoint: %ANTHROPIC_BASE_URL%
echo [*] Model:           %OLLAMA_MODEL%
echo [*] Session ID:      %BAP_SESSION_ID%
echo [*] PreToolUse Hook: cchook\interceptor.exe (Zero-Trust bapedge broker)
echo [*] Liveness Watcher: Active (heartbeats pulsing, no idle timeout)
echo [*] Telemetry:       Streaming to %BAP_SERVER_URL% ^& ltd-audit.jsonl
echo ===============================================================================

:: 7. Auto-detect Claude executable (claude-code.cmd in corporate env, claude on personal laptop)
set CLAUDE_BIN=
where claude-code.cmd >nul 2>&1
if %ERRORLEVEL% equ 0 (
    set CLAUDE_BIN=claude-code.cmd
) else (
    where claude >nul 2>&1
    if %ERRORLEVEL% equ 0 (
        set CLAUDE_BIN=claude
    ) else (
        where claude.exe >nul 2>&1
        if %ERRORLEVEL% equ 0 (
            set CLAUDE_BIN=claude.exe
        ) else (
            set CLAUDE_BIN=claude
        )
    )
)

echo [*] Claude Binary:   %CLAUDE_BIN%
echo.

if "%~1"=="" (
    echo [*] Starting interactive Claude Code session...
    echo [*] Try prompts like:
    echo     - "run git status"       (will be ALLOWED by bapedge)
    echo     - "show contents of .env" (will be DENIED by bapedge)
    echo     - "fetch google headers using curl" (will be DENIED by bapedge)
    echo.
    call %CLAUDE_BIN% --model %OLLAMA_MODEL%
) else (
    echo [*] Executing prompt: %~1
    call %CLAUDE_BIN% --dangerously-skip-permissions --model %OLLAMA_MODEL% -p %1 <nul
)

:: 8. Notify BAP Control Plane of Session End (with strict 2s timeout so it never hangs)
curl.exe -s --max-time 2 --connect-timeout 2 -X POST "%BAP_SERVER_URL%/api/v1/sessions/end" ^
    -H "Content-Type: application/json" ^
    -d "{\"session_id\":\"%BAP_SESSION_ID%\",\"reason\":\"session exited\"}" >nul 2>&1

if exist ".bap-session.json" del /f /q ".bap-session.json" >nul 2>&1
if exist ".bap-prompt.txt" del /f /q ".bap-prompt.txt" >nul 2>&1

echo.
echo [*] Claude Code session %BAP_SESSION_ID% completed gracefully.
endlocal
exit /b 0
