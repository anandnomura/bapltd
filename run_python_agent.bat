@echo off
setlocal
title BAP Zero-Trust Platform - Autonomous Python Agent
color 0E

echo ===============================================================================
echo       BOUNDED AUTHORITY PLANE (BAP) - AUTONOMOUS PYTHON AI AGENT
echo                   SDK-Driven Zero-Trust Tool Governance
echo ===============================================================================
echo.
echo [*] Launching Python Agent with BAP SDK...
echo.

python .\python-agent\agent.py

echo.
echo ===============================================================================
echo  Agent execution finished. Press any key to exit...
pause >nul
endlocal

