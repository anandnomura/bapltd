@echo off
setlocal
:: GitHub Copilot Terminal Execution Wrapper for Windows
:: Routes Copilot commands through ltd-agent with Cedar security policies

set "SCRIPT_DIR=%~dp0"
if exist "%SCRIPT_DIR%copilot_interceptor.exe" (
    "%SCRIPT_DIR%copilot_interceptor.exe" %*
) else if exist "%SCRIPT_DIR%..\ltd-agent\ltd-agent.exe" (
    "%SCRIPT_DIR%..\ltd-agent\ltd-agent.exe" exec --source copilot --raw %*
) else (
    ltd-agent.exe exec --source copilot --raw %*
)
exit /b %ERRORLEVEL%

