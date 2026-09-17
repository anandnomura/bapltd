@echo off
setlocal
cd /d "%~dp0"
title BAP Live Agent Fleet Demonstration
echo ===============================================================================
echo   BAP ZERO-TRUST: DYNAMIC AGENT FLEET LIFECYCLE DEMONSTRATION
echo ===============================================================================
echo Usage:
echo   run_agent_demo.bat               (Interactive: select 5 or scale up to 35)
echo   run_agent_demo.bat 35            (Direct launch of 35-agent live fleet)
echo   run_agent_demo.bat --count 35    (Scale up to 35 agents with custom options)
echo.

python demo_live_agents.py %*

if errorlevel 1 (
    echo.
    echo [-] Demo exited with error code %ERRORLEVEL%.
)

pause

