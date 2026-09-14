@echo off
chcp 65001 >nul
setlocal EnableDelayedExpansion
cd /d "%~dp0"
title BAP Zero-Trust Platform - Executive CIO Demonstration
mode con: cols=100 lines=38
color 0B

echo ====================================================================================================
echo               BOUNDED AUTHORITY PLANE (BAP) - EXECUTIVE DEMONSTRATION FOR THE CIO
echo                  Zero-Trust Pre-Execution Governance & Observability for AI Agents
echo ====================================================================================================
echo.
echo  Welcome! This demonstration showcases how BAP eliminates standing privileges for autonomous
echo  AI coding agents (Claude Code, GitHub Copilot) while preserving sub-2ms developer velocity.
echo.
echo  [*] Initializing pristine demonstration environment...
echo.

:: 0. Reset previous sessions
curl.exe -s --max-time 2 --connect-timeout 2 -X POST "http://localhost:8080/api/v1/sessions/reset" >nul 2>&1

:: 1. Ensure bapcontrolplane is running on port 8080
powershell -NoProfile -Command "if (Get-NetTCPConnection -LocalPort 8080 -State Listen -ErrorAction SilentlyContinue) { exit 0 } else { exit 1 }"
if %errorlevel% equ 0 (
    echo  [+] BAP Control Plane active on http://localhost:8080
) else (
    echo  [*] Starting BAP Control Plane daemon on port 8080...
    start "BAP Control Plane (8080)" /min "%~dp0bapcontrolplane.exe" -port 8080 -ttl 30 -trust-domain bap.internal
    ping -n 3 127.0.0.1 >nul
    echo  [+] Control Plane started successfully.
)

:: 2. Ensure bapgateway is running on port 9090
powershell -NoProfile -Command "if (Get-NetTCPConnection -LocalPort 9090 -State Listen -ErrorAction SilentlyContinue) { exit 0 } else { exit 1 }"
if %errorlevel% equ 0 (
    echo  [+] BAP Gateway PEP active on http://localhost:9090
) else (
    echo  [*] Starting BAP Gateway PEP daemon on port 9090...
    start "BAP Gateway PEP (9090)" /min "%~dp0bapgateway.exe" -port 9090
    ping -n 2 127.0.0.1 >nul
    echo  [+] Gateway PEP started successfully.
)

:: 3. Act 1: Spin up 5 live Claude Code agent workloads across enterprise squads
echo.
echo  ====================================================================================================
echo   ACT 1: ENROLLING 5 LIVE CLAUDE CODE DEVELOPER AGENTS (DUAL IDENTITY & SPIFFE)
echo  ====================================================================================================
echo.
python scripts\launch_fleet.py --count 5 --once

:: Resolve current shell PID so background daemon auto-deregisters if shell is closed
set "MY_PID="
for /f %%i in ('powershell -NoProfile -Command "$pid"') do set "MY_PID=%%i"

:: Start background heartbeat keepalive with auto-deregistration
taskkill /fi "WINDOWTITLE eq BAP-Fleet-Daemon*" /f >nul 2>&1
if "!MY_PID!"=="" (
    start "BAP-Fleet-Daemon" /min python scripts\launch_fleet.py --count 5
) else (
    start "BAP-Fleet-Daemon" /min python scripts\launch_fleet.py --count 5 --parent-pid !MY_PID!
)

:: 4. Launch browser inspector
echo.
echo  [*] Opening BAP Executive Cockpit & Live Fleet Radar in default browser...
start http://localhost:8080/inspector?mode=live

echo.
echo ====================================================================================================
echo   SUCCESS! The BAP Executive Cockpit is now live on your screen.
echo.
echo   You can either:
echo   - Click the interactive hero buttons directly in your browser cockpit, OR
echo   - Press any of the 1-click options below in this terminal.
echo ====================================================================================================
echo.

:MENU_LOOP
echo ----------------------------------------------------------------------------------------------------
echo  EXECUTIVE INTERACTIVE ACTIONS:
echo   [1] [PASS]      Test Safe Multi-Stage Build (Sub-2ms Zero-Friction Pass)
echo   [2] [BLOCK]     Simulate Malicious Attack (Gateway PEP Dropped at Perimeter)
echo   [3] [SCALE]     Toggle Fleet Scale (5 Active Squads vs 25 Enterprise Nodes)
echo   [4] [KILL-ALL]  Toggle Global Emergency Kill-Switch (CISO Enterprise Override)
echo   [5] [KILL-ONE]  Targeted Agent Isolation (Surgically Revoke 'Carol' Without Stopping Fleet)
echo   [6] [AUDIT]     Verify Cryptographic Audit Chain (100%% Anti-Tamper Merkle Proof)
echo   [7] [EXIT]      Exit Demonstration & Cleanly Deregister Fleet
echo ----------------------------------------------------------------------------------------------------
set /p OPT="Select action (1-7) [default: 1]: "
if "%OPT%"=="" set OPT=1

if "%OPT%"=="1" goto DO_SAFE
if "%OPT%"=="2" goto DO_ATTACK
if "%OPT%"=="3" goto DO_SCALE
if "%OPT%"=="4" goto DO_KILL
if "%OPT%"=="5" goto DO_KILL_ONE
if "%OPT%"=="6" goto DO_VERIFY
if "%OPT%"=="7" goto DO_EXIT
echo Invalid choice. Please enter 1-7.
goto MENU_LOOP

:DO_SAFE
echo.
python scripts\demo_actions.py safe
echo.
goto MENU_LOOP

:DO_ATTACK
echo.
python scripts\demo_actions.py attack
echo.
goto MENU_LOOP

:DO_SCALE
echo.
python scripts\demo_actions.py toggle-scale
echo.
goto MENU_LOOP

:DO_KILL
echo.
python scripts\demo_actions.py toggle-kill
echo.
goto MENU_LOOP

:DO_KILL_ONE
echo.
python scripts\demo_actions.py kill-agent --target carol
echo.
goto MENU_LOOP

:DO_VERIFY
echo.
python scripts\demo_actions.py verify
echo.
goto MENU_LOOP

:DO_EXIT
echo.
echo [*] Gracefully deregistering all agent workloads from Control Plane...
python scripts\demo_actions.py cleanup
taskkill /fi "WINDOWTITLE eq BAP-Fleet-Daemon*" /f >nul 2>&1
echo [+] All agent workloads cleanly deregistered from dashboard!
echo [+] Thank you for experiencing BAP!
timeout /t 2 >nul
endlocal
exit /b 0
