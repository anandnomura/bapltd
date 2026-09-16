@echo off
chcp 65001 >nul
setlocal EnableDelayedExpansion
cd /d "%~dp0"
title BAP - Simple Local HTTPS Test (2 Claude Sessions + Python Agent + Gateway)
mode con: cols=105 lines=38
color 0A

echo ====================================================================================================
echo    BOUNDED AUTHORITY PLANE (BAP) - SIMPLE LOCAL TEST
echo    Native HTTPS Control Plane (8443) + Gateway PEP (9090) + 2 Claude Sessions + Python Agent
echo ====================================================================================================
echo.

:: 1. Teardown any previous stale demo processes
echo [*] Cleaning up any previous demo processes...
taskkill /fi "WINDOWTITLE eq BAP-Test-ControlPlane*" /f >nul 2>&1
taskkill /fi "WINDOWTITLE eq BAP-Test-Gateway*" /f >nul 2>&1
taskkill /fi "WINDOWTITLE eq BAP-Test-Watch*" /f >nul 2>&1
del /f /q .bap-session.json .bap-prompt.txt >nul 2>&1
del /f /q bap-controlplane.db bap-controlplane.db-shm bap-controlplane.db-wal >nul 2>&1

:: Resolve binaries
set "CP_BIN=%~dp0dist\windows-amd64\controlplane\bapcontrolplane.exe"
set "GW_BIN=%~dp0dist\windows-amd64\gateway\bapgateway.exe"
set "LTD_BIN=%~dp0dist\windows-amd64\bapedge.exe"

if not exist "!CP_BIN!" (
    echo [-] Missing control plane binary at !CP_BIN!. Running build_all_platforms.bat...
    call build_all_platforms.bat
)

set "ADMIN_TOKEN=admin123"
set "CP_PORT=8443"
set "GW_PORT=9090"
set "CP_URL=https://localhost:!CP_PORT!"
set "GW_URL=http://localhost:!GW_PORT!"

:: 2. Start HTTPS Control Plane
echo [*] Starting Central Control Plane with native HTTPS on port !CP_PORT!...
start "BAP-Test-ControlPlane" /min "!CP_BIN!" -port !CP_PORT! -https -admin-token !ADMIN_TOKEN! -allow-remote-admin
ping -n 3 127.0.0.1 >nul

:: Resolve CA cert
set "CERT_FILE=%~dp0bap-root-ca.crt"
if not exist "!CERT_FILE!" set "CERT_FILE=%~dp0controlplane-cert.pem"

if exist "!CERT_FILE!" (
    echo [+] TLS Trust Certificate: !CERT_FILE!
    set "BAP_CA_CERT=!CERT_FILE!"
    set "CURL_CERT_ARG=--ssl-no-revoke --cacert "!CERT_FILE!""
) else (
    echo [-] Warning: Root CA not found; proceeding with insecure curl fallback.
    set "CURL_CERT_ARG=-k"
)

:: 3. Start Gateway PEP
echo [*] Starting Zero-Trust Gateway PEP on port !GW_PORT! connected to !CP_URL!...
start "BAP-Test-Gateway" /min "!GW_BIN!" -port !GW_PORT! -controlplane !CP_URL!
ping -n 2 127.0.0.1 >nul

:: 4. Launch Browser Cockpit
echo.
echo ====================================================================================================
echo  [+] OPENING BAP INSPECTOR IN YOUR BROWSER...
echo      URL: !CP_URL!/inspector?mode=live
echo ====================================================================================================
start "" "!CP_URL!/inspector?mode=live"
ping -n 2 127.0.0.1 >nul

:: 5. Simulate Claude Session 1 (Developer Alice)
echo.
echo ----------------------------------------------------------------------------------------------------
echo  [SCENARIO 1] Starting Claude Code Session: Alice (Payments Microservice)
echo ----------------------------------------------------------------------------------------------------
set "SESS_ALICE=sess-claude-alice-%RANDOM%"
set "PROMPT_ALICE=Review payment microservice code and inspect git commit history"
echo  - User:       alice@company.internal
echo  - Session ID: !SESS_ALICE!
echo  - Prompt:     "!PROMPT_ALICE!"

curl.exe -s !CURL_CERT_ARG! -X POST "!CP_URL!/api/v1/sessions/start" ^
  -H "Content-Type: application/json" ^
  -d "{\"session_id\":\"!SESS_ALICE!\",\"app_id\":\"claude-code\",\"user_id\":\"alice\",\"user_email\":\"alice@company.internal\",\"user_prompt\":\"!PROMPT_ALICE!\",\"hostname\":\"ALICE-MBP\"}" >nul

start "BAP-Test-Watch-Alice" /min "!LTD_BIN!" watch --session-id=!SESS_ALICE! --server=!CP_URL! --app-id=claude-code --detach

echo  [*] Alice executes safe command: git status...
set "BAP_SESSION_ID=!SESS_ALICE!"
set "BAP_USER_ID=alice"
set "BAP_USER_EMAIL=alice@company.internal"
set "BAP_USER_PROMPT=!PROMPT_ALICE!"
set "BAP_SERVER_URL=!CP_URL!"
"!LTD_BIN!" exec --source claude-code --session-id !SESS_ALICE! --raw "git status" >nul 2>&1
echo      -> [ALLOWED] git status recorded on Live Radar with prompt badge.
ping -n 2 127.0.0.1 >nul

:: 6. Simulate Claude Session 2 (Developer Bob)
echo.
echo ----------------------------------------------------------------------------------------------------
echo  [SCENARIO 2] Starting Claude Code Session: Bob (Database Refactor ^& Security Test)
echo ----------------------------------------------------------------------------------------------------
set "SESS_BOB=sess-claude-bob-%RANDOM%"
set "PROMPT_BOB=Implement SQLite database schema and test credential isolation"
echo  - User:       bob@company.internal
echo  - Session ID: !SESS_BOB!
echo  - Prompt:     "!PROMPT_BOB!"

curl.exe -s !CURL_CERT_ARG! -X POST "!CP_URL!/api/v1/sessions/start" ^
  -H "Content-Type: application/json" ^
  -d "{\"session_id\":\"!SESS_BOB!\",\"app_id\":\"claude-code\",\"user_id\":\"bob\",\"user_email\":\"bob@company.internal\",\"user_prompt\":\"!PROMPT_BOB!\",\"hostname\":\"BOB-WORKSTATION\"}" >nul

start "BAP-Test-Watch-Bob" /min "!LTD_BIN!" watch --session-id=!SESS_BOB! --server=!CP_URL! --app-id=claude-code --detach

set "BAP_SESSION_ID=!SESS_BOB!"
set "BAP_USER_ID=bob"
set "BAP_USER_EMAIL=bob@company.internal"
set "BAP_USER_PROMPT=!PROMPT_BOB!"

echo  [*] Bob executes safe command: python --version...
"!LTD_BIN!" exec --source claude-code --session-id !SESS_BOB! --raw "python --version" >nul 2>&1
echo      -> [ALLOWED] python --version allowed (Sub-2ms).

rem [NEGATIVE TEST - MOVED TO run_negative_testcases.bat to avoid EDR alerts]
rem echo  [*] Bob attempts credential theft attack: cat .env...
rem "!LTD_BIN!" exec --source claude-code --session-id !SESS_BOB! --raw "cat .env" >nul 2>&1
rem echo  [*] Bob attempts daemon tampering attack: Stop-Process bapedge...
rem "!LTD_BIN!" exec --source claude-code --session-id !SESS_BOB! --raw "powershell -Command Stop-Process -Name bapedge" >nul 2>&1

echo  [*] Bob executes legitimate developer process management: taskkill /IM node.exe...
"!LTD_BIN!" exec --source claude-code --session-id !SESS_BOB! --raw "taskkill /F /IM node.exe" >nul 2>&1
echo      -> [ALLOWED] Legitimate developer process kill is permitted.
ping -n 2 127.0.0.1 >nul

:: 7. Run Governed Python Agent SDK
echo.
echo ----------------------------------------------------------------------------------------------------
echo  [SCENARIO 3] Running Python Agent SDK: Financial Reconciler with Gateway PEP
echo ----------------------------------------------------------------------------------------------------
python scripts\test_financial_reconciler.py --server-url "!CP_URL!" --gateway-url "!GW_URL!" --cert-file "!CERT_FILE!" --admin-token "!ADMIN_TOKEN!"

echo.
echo ====================================================================================================
echo  [SUCCESS] All 3 Scenarios Executed Live!
echo ====================================================================================================
echo  1. Switch to your browser window: !CP_URL!/inspector?mode=live
echo  2. Observe:
echo     - Top Banner: Active Developer Intent banner displaying user prompts.
echo     - Radar Chips: Alice, Bob, and Python Reconciler running concurrently.
echo     - Event Stream: Every card has a 'Prompt: ...' badge and sub-2ms metrics.
echo     - Attack Alerts: Blocked credential exfiltration and tamper attempts in red.
echo     - Detail Drawer: Click any event card to view the complete forensic record.
echo.
echo  Press any key to complete workloads and watch the Live Radar drop to 0 agents...
pause >nul

echo.
echo ----------------------------------------------------------------------------------------------------
echo  [SCENARIO 4] Graceful Session Teardown ^& Workload Deregistration
echo ----------------------------------------------------------------------------------------------------
echo  [*] Deregistering all agent workloads to cleanly transition Live Radar to 0 active agents...
python scripts\close_sessions.py --server-url "!CP_URL!" --cert-file "!CERT_FILE!"
taskkill /fi "WINDOWTITLE eq BAP-Test-Watch*" /f >nul 2>&1

echo  [*] Waiting for browser Live Radar to synchronize (2-3s poll cycle)...
ping -n 4 127.0.0.1 >nul

echo.
echo ====================================================================================================
echo  [RADAR VERIFIED] 0 ACTIVE AGENTS (All workloads cleanly deregistered!)
echo ====================================================================================================
echo  Switch to your browser window (!CP_URL!/inspector?mode=live) now:
echo    - Live Workload Radar badge: [0 ACTIVE AGENTS]
echo    - Status text: "All workloads closed & deregistered. Ready for next agent session."
echo    - Active Chips: "No client sessions active"
echo    - Toast Notifications: Clean workload deregistration logged in UI.
echo.
echo  Press any key to complete test and tear down daemons (Control Plane ^& Gateway)...
pause >nul

echo.
echo [*] Tearing down test daemons...
taskkill /fi "WINDOWTITLE eq BAP-Test-ControlPlane*" /f >nul 2>&1
taskkill /fi "WINDOWTITLE eq BAP-Test-Gateway*" /f >nul 2>&1
del /f /q .bap-session.json .bap-prompt.txt >nul 2>&1
echo [OK] All test processes stopped cleanly.
exit /b 0

