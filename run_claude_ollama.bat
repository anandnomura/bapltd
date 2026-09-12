@echo off
setlocal

echo ===============================================================================
echo   Launching Claude Code with Local Ollama & BAP Edge Interceptor
echo ===============================================================================

:: 1. Set Anthropic API redirection to local Ollama instance
set ANTHROPIC_BASE_URL=http://localhost:11434
set ANTHROPIC_API_KEY=ollama

:: 2. Target the local Ollama model
set OLLAMA_MODEL=claude-3-5-sonnet-20241022:latest

:: 3. Verify cchook interceptor exists
if not exist "cchook\interceptor.exe" (
    echo [*] Building cchook\interceptor.exe...
    cd cchook && go build -o interceptor.exe . && cd ..
)

echo [*] Ollama Endpoint: %ANTHROPIC_BASE_URL%
echo [*] Model:           %OLLAMA_MODEL%
echo [*] PreToolUse Hook: cchook\interceptor.exe (Zero-Trust bapedge broker)
echo [*] Audit Log:       ltd-audit.jsonl
echo ===============================================================================

if "%~1"=="" (
    echo [*] Starting interactive Claude Code session...
    echo [*] Try prompts like:
    echo     - "run git status"       (will be ALLOWED by bapedge)
    echo     - "show contents of .env" (will be DENIED by bapedge)
    echo     - "fetch google headers using curl" (will be DENIED by bapedge)
    echo.
    claude --model %OLLAMA_MODEL%
) else (
    echo [*] Executing prompt: "%*"
    claude --model %OLLAMA_MODEL% -p "%*"
)

endlocal

