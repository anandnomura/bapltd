@echo off
setlocal
cd /d "%~dp0"
echo [*] Stopping BAP Simple Local Test processes...
taskkill /fi "WINDOWTITLE eq BAP-Test-ControlPlane*" /f >nul 2>&1
taskkill /fi "WINDOWTITLE eq BAP-Test-Gateway*" /f >nul 2>&1
taskkill /fi "WINDOWTITLE eq BAP-Test-Watch*" /f >nul 2>&1
del /f /q .bap-session.json .bap-prompt.txt >nul 2>&1
echo [OK] Simple local test daemons stopped.

