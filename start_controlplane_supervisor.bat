@echo off
setlocal
cd /d "%~dp0"
echo ===============================================================================
echo   Starting BAP Control Plane under Process Supervisor Watchdog
echo ===============================================================================
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0scripts\supervise_controlplane.ps1" %*

