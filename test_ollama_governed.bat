@echo off
setlocal EnableDelayedExpansion
cd /d "%~dp0"
title BAP Zero-Trust - Governed Ollama Execution Test

echo ===============================================================================
echo        BOUNDED AUTHORITY PLANE (BAP) - GOVERNED OLLAMA TEST SUITE
echo ===============================================================================
echo.
echo  This script demonstrates two fundamental Zero-Trust security principles:
echo.
echo  TEST 1: Shell Egress Interception
echo  -----------------------------------------------------------------------------
echo  What happens when an agent tries to execute raw PowerShell 'Invoke-RestMethod'
echo  or curl directly in shell commands?
echo  ==^> BAP Cedar policy BLOCKS it cold at the broker boundary (DLP protection).
echo.
echo  TEST 2: Authorized Governed Execution
echo  -----------------------------------------------------------------------------
echo  How should local LLM inferences run safely under BAP supervision?
echo  ==^> BAP authorizes legitimate developer tools (python scripts/benchmark_ollama.py)
echo      with sub-2ms policy evaluation while logging full audit telemetry.
echo.
echo ===============================================================================
echo.

:: -----------------------------------------------------------------------------
:: TEST 1: Egress Denial (PowerShell Invoke-RestMethod)
:: -----------------------------------------------------------------------------
echo [*] TEST 1: Executing raw PowerShell Invoke-RestMethod via bapedge...
echo [*] Command: bapedge exec --raw "powershell -Command Invoke-RestMethod http://localhost:11434"
echo.
bapedge.exe exec --raw "powershell -Command Invoke-RestMethod http://localhost:11434"
if %errorlevel% neq 0 (
    echo.
    echo [+] SUCCESS: BAP Cedar policy correctly DENIED Invoke-RestMethod!
    echo     Egress utilities cannot be executed from agent shell calls.
) else (
    echo.
    echo [!] WARNING: Command was unexpectedly allowed. Check policy.cedar.
)

echo.
echo ===============================================================================
echo.

:: -----------------------------------------------------------------------------
:: TEST 2: Governed Python Ollama Benchmark
:: -----------------------------------------------------------------------------
echo [*] TEST 2: Executing governed Python Ollama inference benchmark via bapedge...
echo [*] Command: bapedge exec python scripts\benchmark_ollama.py
echo.
bapedge.exe exec python scripts\benchmark_ollama.py
if %errorlevel% equ 0 (
    echo [+] SUCCESS: Governed Python inference completed under BAP supervision!
) else (
    echo [!] Note: Ensure Ollama is running on port 11434 with model 'claude-3-5-sonnet-20241022:latest'.
)

echo.
echo ===============================================================================
echo  All tests complete. Check ltd-audit.jsonl for the immutable SHA-256 audit log.
echo ===============================================================================
echo.
endlocal

