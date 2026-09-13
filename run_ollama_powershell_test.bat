@echo off
setlocal
cd /d "%~dp0"
title BAP Governed Execution - Ollama PowerShell Test

echo ===============================================================================
echo      BOUNDED AUTHORITY PLANE (BAP) - GOVERNED POWERSHELL OLLAMA TEST
echo ===============================================================================
echo.
echo  Command:
echo  bapedge.exe exec powershell -NoProfile -ExecutionPolicy Bypass -File scripts\test_ollama.ps1
echo.
echo [*] Executing under BAP Zero-Trust broker...
echo.

bapedge.exe exec powershell -NoProfile -ExecutionPolicy Bypass -File scripts\test_ollama.ps1

echo.
echo ===============================================================================
echo  Check ltd-audit.jsonl to verify the immutable SHA-256 hash-chained audit record.
echo ===============================================================================
echo.
pause
endlocal

