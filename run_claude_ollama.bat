@echo off
setlocal EnableDelayedExpansion

echo ===============================================================================
echo   Launching Claude Code with Local Ollama & BAP Edge Interceptor
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

:: 4. Verify cchook interceptor exists
if not exist "cchook\interceptor.exe" (
    echo [*] Building cchook\interceptor.exe...
    cd cchook && go build -o interceptor.exe interceptor.go && cd ..
)

:: 5. Notify BAP Control Plane of Session Start (fail-securely ignored if offline)
curl.exe -s -X POST "%BAP_SERVER_URL%/api/v1/sessions/start" ^
    -H "Content-Type: application/json" ^
    -d "{\"session_id\":\"%BAP_SESSION_ID%\",\"app_id\":\"claude-code\",\"user_id\":\"%USERNAME%\",\"hostname\":\"%COMPUTERNAME%\"}" >nul 2>&1

echo [*] Ollama Endpoint: %ANTHROPIC_BASE_URL%
echo [*] Model:           %OLLAMA_MODEL%
echo [*] Session ID:      %BAP_SESSION_ID%
echo [*] PreToolUse Hook: cchook\interceptor.exe (Zero-Trust bapedge broker)
echo [*] Telemetry:       Streaming to %BAP_SERVER_URL% ^& ltd-audit.jsonl
echo ===============================================================================

:: 6. Auto-detect Claude executable (claude-code.cmd in corporate env, claude on personal laptop)
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
    echo [*] Executing prompt: "%*"
    call %CLAUDE_BIN% --model %OLLAMA_MODEL% -p "%*"
)

:: 7. Notify BAP Control Plane of Session End (with strict 2s timeout so it never hangs)
curl.exe -s --max-time 2 --connect-timeout 2 -X POST "%BAP_SERVER_URL%/api/v1/sessions/end" ^
    -H "Content-Type: application/json" ^
    -d "{\"session_id\":\"%BAP_SESSION_ID%\",\"reason\":\"session exited\"}" >nul 2>&1

echo.
echo [*] Claude Code session %BAP_SESSION_ID% completed gracefully.
endlocal
exit /b 0
