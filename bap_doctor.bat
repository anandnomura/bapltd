@echo off
setlocal EnableExtensions

set "ROOT_DIR=%~dp0"
powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%ROOT_DIR%scripts\bap_doctor.ps1" %*
exit /b %ERRORLEVEL%
