@echo off
setlocal
cd /d "%~dp0"
title BAP Zero-Trust MCP Server Interactive Demo
python scripts\mcp_demo.py
echo.
pause
endlocal

