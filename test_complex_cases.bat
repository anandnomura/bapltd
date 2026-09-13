@echo off
setlocal
cd /d "%~dp0"
powershell -ExecutionPolicy Bypass -NoProfile -File "%~dp0test_complex_cases.ps1"
exit /b %ERRORLEVEL%

