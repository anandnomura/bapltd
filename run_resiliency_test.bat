@echo off
setlocal
cd /d "%~dp0"
echo ===============================================================================
echo   Running Headless BAP Resiliency, Scaling, and Fault Tolerance Suite
echo ===============================================================================
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\test_resiliency_headless.ps1" %*
exit /b %errorlevel%

