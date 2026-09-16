@echo off
setlocal
cd /d "%~dp0"
title BAP Live Agent Fleet Demonstration
echo ===============================================================================
echo   BAP ZERO-TRUST: DYNAMIC AGENT FLEET LIFECYCLE DEMONSTRATION
echo ===============================================================================
echo.

python demo_live_agents.py

if errorlevel 1 (
    echo.
    echo [-] Demo exited with error code %ERRORLEVEL%.
)

pause

