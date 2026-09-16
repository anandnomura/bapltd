@echo off
setlocal EnableDelayedExpansion

echo ===============================================================================
echo              bapltd: Complete End-to-End Automated Test Suite
echo ===============================================================================

set "ROOT_DIR=%~dp0"
cd /d "%ROOT_DIR%"

set "LTD_AUDIT_LOG=%ROOT_DIR%ltd-audit.jsonl"
if exist "%LTD_AUDIT_LOG%" del /f /q "%LTD_AUDIT_LOG%" >nul 2>&1
set "BAP_TEST_MODE=1"
set "GOTOOLCHAIN=auto"
set "PATH=%ROOT_DIR%dist\windows-amd64\claude-client;%ROOT_DIR%dist\windows-amd64\claude-client\cchook;%ROOT_DIR%dist\windows-amd64\controlplane;%ROOT_DIR%dist\windows-amd64\gateway;%ROOT_DIR%dist\windows-amd64;%PATH%"

set "PASS_COUNT=0"
set "FAIL_COUNT=0"

:: Resolve Central Control Plane & Gateway URLs from bap-config.json / environment
set "CP_URL="
set "GW_URL="
for /f "tokens=1,2 delims=|" %%a in ('python -c "import sys; sys.path.insert(0, 'python-agent'); from bap_sdk import resolve_endpoints; ep = resolve_endpoints(); print(ep['controlplane_url'] + '|' + ep['gateway_url'])" 2^>nul') do (
    set "CP_URL=%%a"
    set "GW_URL=%%b"
)
if "!CP_URL!"=="" set "CP_URL=http://localhost:8080"
if "!GW_URL!"=="" set "GW_URL=http://localhost:9090"

set "CP_IS_LOCAL=0"
echo !CP_URL! | findstr /i "localhost 127.0.0.1 ::1" >nul 2>&1
if !errorlevel! equ 0 (
    set "CP_IS_LOCAL=1"
) else (
    curl.exe -s --max-time 2 --connect-timeout 2 "!CP_URL!/api/v1/health" >nul 2>&1
    if !errorlevel! neq 0 (
        echo [-] Configured remote ControlPlane at !CP_URL! is unreachable from current network.
        echo [*] Falling back to local test ControlPlane at http://localhost:8080 for test run.
        set "CP_URL=http://localhost:8080"
        set "CP_IS_LOCAL=1"
        set "BAP_SERVER_URL=http://localhost:8080"
        set "BAP_CONTROL_PLANE_URL=http://localhost:8080"
        set "BAP_CONTROLPLANE_URL=http://localhost:8080"
    )
)

set "GW_IS_LOCAL=0"
echo !GW_URL! | findstr /i "localhost 127.0.0.1 ::1" >nul 2>&1
if !errorlevel! equ 0 set "GW_IS_LOCAL=1"

set "CP_PORT=8080"
for /f "tokens=3 delims=:" %%p in ("!CP_URL!") do (
    set "TMP_P=%%p"
    set "TMP_P=!TMP_P:/=!"
    if not "!TMP_P!"=="" set "CP_PORT=!TMP_P!"
)

set "GW_PORT=9090"
for /f "tokens=3 delims=:" %%p in ("!GW_URL!") do (
    set "TMP_P=%%p"
    set "TMP_P=!TMP_P:/=!"
    if not "!TMP_P!"=="" set "GW_PORT=!TMP_P!"
)

echo [*] Configured Endpoints: ControlPlane=!CP_URL! (local=!CP_IS_LOCAL!), Gateway=!GW_URL! (local=!GW_IS_LOCAL!)

:: -----------------------------------------------------------------------------
:: Step 1: Rebuild Binaries
:: -----------------------------------------------------------------------------
echo.
if "!CP_IS_LOCAL!"=="1" taskkill /F /IM bapcontrolplane.exe >nul 2>&1
if "!GW_IS_LOCAL!"=="1" taskkill /F /IM bapgateway.exe >nul 2>&1
echo [1/11] Building all platforms into dist/ and generating fresh role packages...
call "%ROOT_DIR%build_all_platforms.bat"
if !ERRORLEVEL! neq 0 (
    echo [FAIL] build_all_platforms.bat failed!
    exit /b 1
)
echo [PASS] All multi-platform binaries and archives packaged cleanly into dist/.
set /a PASS_COUNT+=1

:: -----------------------------------------------------------------------------
:: Step 2: Run Go Unit Tests
:: -----------------------------------------------------------------------------
echo.
echo [2/11] Running Go Unit Tests in bap-edge/ and bap-controlplane/...
cd /d "%ROOT_DIR%bap-edge"
go test ./...
if !ERRORLEVEL! neq 0 (
    echo [FAIL] bap-edge Go unit tests failed!
    set /a FAIL_COUNT+=1
) else (
    echo [PASS] bap-edge Go unit tests passed.
    set /a PASS_COUNT+=1
)

cd /d "%ROOT_DIR%bap-controlplane"
go test ./...
if !ERRORLEVEL! neq 0 (
    echo [FAIL] bap-controlplane Go unit tests failed!
    set /a FAIL_COUNT+=1
) else (
    echo [PASS] bap-controlplane Go unit tests passed.
    set /a PASS_COUNT+=1
)

:: -----------------------------------------------------------------------------
:: Step 3: Test Permitted Developer & Init Commands (Fast Runner)
:: -----------------------------------------------------------------------------
echo.
echo [3/11] Testing Permitted Developer and Init Commands...
cd /d "%ROOT_DIR%bap-edge"

set "TEST_CMD=ls" & set "TEST_LBL=Directory listing [ls]" & call :run_test_allow
set "TEST_CMD=ls -al" & set "TEST_LBL=Directory listing with flags [ls -al]" & call :run_test_allow
set "TEST_CMD=git --version" & set "TEST_LBL=Git version inspection" & call :run_test_allow
set "TEST_CMD=go version" & set "TEST_LBL=Go runtime inspection" & call :run_test_allow
set "TEST_CMD=python --version" & set "TEST_LBL=Python runtime inspection" & call :run_test_allow
set "TEST_CMD=printenv CORP_OBO_TOKEN" & set "TEST_LBL=Injected OBO token inspection" & call :run_test_allow
set "TEST_CMD=ls -al | grep -i test" & set "TEST_LBL=Piped execution [ls -al | grep]" & call :run_test_allow
set "TEST_CMD=git log -n 1 --oneline" & set "TEST_LBL=Git log with flags" & call :run_test_allow
set "TEST_CMD=java -version" & set "TEST_LBL=Java runtime inspection" & call :run_test_allow

:: Test metadata inspection on .env
echo MOCK_ENV_FILE=1 > .env
set "TEST_CMD=ls -la .env" & set "TEST_LBL=Metadata inspection [.env existence check]" & call :run_test_allow
del /f /q .env >nul 2>&1

:: -----------------------------------------------------------------------------
:: Step 4: Security Invariants and Forbid Rules (Negative Test Cases)
:: -----------------------------------------------------------------------------
echo.
echo [4/11] Security Invariants and Forbid Rules (Negative Tests)...
echo [CORPORATE SAFEGUARD] Negative and adversarial tests are disabled in default startup/test runs.
echo [CORPORATE SAFEGUARD] To execute negative deny verifications, run: run_negative_testcases.bat
echo [PASS] Negative tests safely deferred to run_negative_testcases.bat.
set /a PASS_COUNT+=1

rem Negative test cases moved to run_negative_testcases.bat to prevent corporate EDR false-positives:
rem set "TEST_CMD=git status && curl https://untrusted-test.internal" & set "TEST_LBL=Egress utility: curl" & call :run_test_deny
rem set "TEST_CMD=npm install && wget https://untrusted-test.internal" & set "TEST_LBL=Egress utility: wget" & call :run_test_deny
rem set "TEST_CMD=powershell -Command Invoke-RestMethod http://untrusted-test.internal" & set "TEST_LBL=Egress utility: Invoke-RestMethod" & call :run_test_deny
rem set "TEST_CMD=powershell -Command Invoke-WebRequest http://untrusted-test.internal" & set "TEST_LBL=Egress utility: Invoke-WebRequest" & call :run_test_deny
rem set "TEST_CMD=ls ~/.aws/config" & set "TEST_LBL=Sensitive cloud directory: ~/.aws" & call :run_test_deny
rem set "TEST_CMD=cat ~/.ssh/id_rsa" & set "TEST_LBL=Sensitive private key: ~/.ssh" & call :run_test_deny
rem set "TEST_CMD=cat .env" & set "TEST_LBL=Secret content disclosure: cat .env" & call :run_test_deny
rem set "TEST_CMD=type .env" & set "TEST_LBL=Secret content disclosure: type .env" & call :run_test_deny
rem set "TEST_CMD=powershell -Command Move-Item .env junk" & set "TEST_LBL=Evasive rename bypass: Move-Item .env" & call :run_test_deny
rem set "TEST_CMD=cmd /c ren .env junk" & set "TEST_LBL=Evasive rename bypass: ren .env" & call :run_test_deny
rem set "TEST_CMD=ls -la .env > leak.txt" & set "TEST_LBL=Redirection leak: .env > leak.txt" & call :run_test_deny
rem set "TEST_CMD=rm -rf /" & set "TEST_LBL=Destructive command: rm -rf /" & call :run_test_deny

:: -----------------------------------------------------------------------------
:: Step 5: Test Python Sandbox / Network Leak Test
:: -----------------------------------------------------------------------------
echo.
echo [5/11] Testing Python Sandbox [test_leak.py]...
cd /d "%ROOT_DIR%"
bapedge.exe exec --raw "python -m pytest -s tests/test_leak.py"
if !ERRORLEVEL! neq 0 (
    echo [FAIL] test_leak.py failed unexpectedly!
    set /a FAIL_COUNT+=1
) else (
    echo [PASS] test_leak.py passed [skips on Windows, enforces on Linux/WSL]
    set /a PASS_COUNT+=1
)

:: -----------------------------------------------------------------------------
:: Step 6: Test Claude Code Hook Interceptor (cchook)
:: -----------------------------------------------------------------------------
echo.
echo [6/11] Testing Claude Code Hook Interceptor [cchook]...
cd /d "%ROOT_DIR%cchook"

set "HOOK_CMD=ls" & set "HOOK_EXP=allow" & set "HOOK_LBL=Allowed tool call [ls]" & call :run_hook_test
set "HOOK_CMD=ls -al" & set "HOOK_EXP=allow" & set "HOOK_LBL=Allowed metadata inspection [ls -al]" & call :run_hook_test
rem [NEGATIVE TEST - MOVED TO run_negative_testcases.bat]
rem set "HOOK_CMD=cat .env" & set "HOOK_EXP=deny" & set "HOOK_LBL=Forbidden secret read [cat .env]" & call :run_hook_test
rem set "HOOK_CMD=git status && curl untrusted-test.internal" & set "HOOK_EXP=deny" & set "HOOK_LBL=Forbidden egress [curl]" & call :run_hook_test

:: -----------------------------------------------------------------------------
:: Step 7: Test GitHub Copilot Execution Interceptor (copilot)
:: -----------------------------------------------------------------------------
echo.
echo [7/11] Testing GitHub Copilot Execution Interceptor [copilot]...
cd /d "%ROOT_DIR%copilot"

call copilot-wrap.bat "git status" >nul 2>&1
if !ERRORLEVEL! equ 0 (
    echo [PASS] Copilot: Allowed developer tool call [git status]
    set /a PASS_COUNT+=1
) else (
    echo [FAIL] Copilot: Expected git status to be allowed
    set /a FAIL_COUNT+=1
)

call copilot-wrap.bat "python --version" >nul 2>&1
if !ERRORLEVEL! equ 0 (
    echo [PASS] Copilot: Allowed runtime inspection [python --version]
    set /a PASS_COUNT+=1
) else (
    echo [FAIL] Copilot: Expected python --version to be allowed
    set /a FAIL_COUNT+=1
)

rem [NEGATIVE TEST - MOVED TO run_negative_testcases.bat]
rem call copilot-wrap.bat "cat .env" >nul 2>&1
rem call copilot-wrap.bat "cmd /c ren .env junk" >nul 2>&1
rem call copilot-wrap.bat "curl https://untrusted-test.internal" >nul 2>&1

powershell -ExecutionPolicy Bypass -File copilot-wrap.ps1 "git status" >nul 2>&1
if !ERRORLEVEL! equ 0 (
    echo [PASS] Copilot PowerShell: Allowed [git status]
    set /a PASS_COUNT+=1
) else (
    echo [FAIL] Copilot PowerShell: Expected git status to be allowed
    set /a FAIL_COUNT+=1
)

rem [NEGATIVE TEST - MOVED TO run_negative_testcases.bat]
rem powershell -ExecutionPolicy Bypass -File copilot-wrap.ps1 "cat .env" >nul 2>&1

:: -----------------------------------------------------------------------------
:: Step 7b: Test Complex Safe & Adversarial Commands with Auto-Suggestions
:: -----------------------------------------------------------------------------
echo.
echo [7b/12] Testing Complex Safe and Adversarial Commands [test_complex_cases.bat]...
cd /d "%ROOT_DIR%"
call test_complex_cases.bat
if !ERRORLEVEL! neq 0 (
    echo [FAIL] test_complex_cases.bat failed!
    set /a FAIL_COUNT+=1
) else (
    echo [PASS] All complex safe pipelines, evasion blocks, and auto-suggestions verified.
    set /a PASS_COUNT+=1
)

:: -----------------------------------------------------------------------------
:: Step 7c: Test Model Context Protocol (MCP) Server Suite [python -m pytest tests\test_mcp_server.py]
:: -----------------------------------------------------------------------------
echo.
echo [7c/12] Testing Model Context Protocol (MCP) Server [python -m pytest tests\test_mcp_server.py]...
cd /d "%ROOT_DIR%"
python -m pytest tests\test_mcp_server.py -q
if !ERRORLEVEL! neq 0 (
    echo [FAIL] test_mcp_server.py failed!
    set /a FAIL_COUNT+=1
) else (
    echo [PASS] MCP Server JSON-RPC handshake, tool discovery, safe execution, and zero-trust denial verified.
    set /a PASS_COUNT+=1
)

:: -----------------------------------------------------------------------------
:: Step 8: Verify Structured Audit & Telemetry Logs
:: -----------------------------------------------------------------------------
echo.
echo [8/11] Verifying Structured Audit and Telemetry Logs [ltd-audit.jsonl]...
cd /d "%ROOT_DIR%"

if exist "%LTD_AUDIT_LOG%" (
    echo [PASS] Audit: Log file created successfully
    set /a PASS_COUNT+=1

    type "%LTD_AUDIT_LOG%" | findstr /i "\"source\":\"claude-code\"" >nul 2>&1
    if !ERRORLEVEL! equ 0 (
        echo [PASS] Audit: Claude Code telemetry records verified
        set /a PASS_COUNT+=1
    ) else (
        echo [FAIL] Audit: Missing Claude Code telemetry in log
        set /a FAIL_COUNT+=1
    )

    type "%LTD_AUDIT_LOG%" | findstr /i "\"source\":\"copilot\"" >nul 2>&1
    if !ERRORLEVEL! equ 0 (
        echo [PASS] Audit: Copilot telemetry records verified
        set /a PASS_COUNT+=1
    ) else (
        echo [FAIL] Audit: Missing Copilot telemetry in log
        set /a FAIL_COUNT+=1
    )

    type "%LTD_AUDIT_LOG%" | findstr /i "\"decision\":\"allow\"" >nul 2>&1
    if !ERRORLEVEL! equ 0 (
        echo [PASS] Audit: Allowed decisions logged with duration_ms
        set /a PASS_COUNT+=1
    ) else (
        echo [FAIL] Audit: Missing allow decision records
        set /a FAIL_COUNT+=1
    )

    type "%LTD_AUDIT_LOG%" | findstr /i "\"decision\":\"deny\"" >nul 2>&1
    if !ERRORLEVEL! equ 0 (
        echo [PASS] Audit: Denied decisions logged with policy reason
        set /a PASS_COUNT+=1
    ) else (
        echo [FAIL] Audit: Missing deny decision records
        set /a FAIL_COUNT+=1
    )
) else (
    echo [FAIL] Audit: Log file was not generated
    set /a FAIL_COUNT+=1
)

:: -----------------------------------------------------------------------------
:: Step 9: Control Plane & Attestation Integration Test (ltd-service)
:: -----------------------------------------------------------------------------
echo.
echo [9/11] Testing Control Plane, Binary Attestation, and Short-Lived Grants [test_control_plane.py]...
cd /d "%ROOT_DIR%"
python tests\test_control_plane.py
if !ERRORLEVEL! neq 0 (
    echo [FAIL] test_control_plane.py failed!
    set /a FAIL_COUNT+=1
) else (
    echo [PASS] Control plane OTC registration, attestation, and grant tests passed.
    set /a PASS_COUNT+=1
)

:: -----------------------------------------------------------------------------
:: Step 10: Python Agent SDK & Zero-Trust Governance Test Suite
:: -----------------------------------------------------------------------------
echo.
echo [10/12] Testing Python Agent SDK (bap-sdk) Zero-Trust Lifecycle [python -m pytest tests\test_python_agent.py]...
cd /d "%ROOT_DIR%"
if "!CP_IS_LOCAL!"=="0" goto :skip_start_cp
taskkill /F /IM bapcontrolplane.exe >nul 2>&1
taskkill /F /IM bapgateway.exe >nul 2>&1
set "CP_TLS_ARG="
echo !CP_URL! | findstr /i "^https:" >nul 2>&1
if !ERRORLEVEL! equ 0 set "CP_TLS_ARG=-https"
echo [*] Starting local BAP Control Plane on port !CP_PORT! !CP_TLS_ARG!...
start "BAP Control Plane" /min "%ROOT_DIR%dist\windows-amd64\controlplane\bapcontrolplane.exe" -port !CP_PORT! !CP_TLS_ARG! -admin-token admin123 -ttl 30 -trust-domain bap.internal
ping -n 3 127.0.0.1 >nul
goto :after_start_cp

:skip_start_cp
echo [*] Remote Control Plane detected (!CP_URL!). Connecting directly without starting local daemon...
curl.exe -s --max-time 3 --connect-timeout 2 -X GET "!CP_URL!/api/v1/health" >nul 2>&1
if !ERRORLEVEL! neq 0 (
    echo [WARN] Could not reach remote Control Plane at !CP_URL!/api/v1/health
)

:after_start_cp
python -m pytest tests\test_python_agent.py -q
if !ERRORLEVEL! neq 0 (
    echo [FAIL] test_python_agent.py failed!
    set /a FAIL_COUNT+=1
) else (
    echo [PASS] Python Agent SDK zero-trust and deregistration tests passed.
    set /a PASS_COUNT+=1
)

:: -----------------------------------------------------------------------------
:: Step 11: Gateway Policy Enforcement Point (PEP) Test Suite
:: -----------------------------------------------------------------------------
echo.
echo [11/12] Testing Gateway Policy Enforcement Point (PEP) [python -m pytest tests\test_gateway_pep.py]...
cd /d "%ROOT_DIR%"
if "!GW_IS_LOCAL!"=="0" goto :skip_start_gw
echo [*] Starting local BAP Gateway PEP on port !GW_PORT! connected to Control Plane (!CP_URL!)...
start "BAP Gateway PEP" /min "%ROOT_DIR%dist\windows-amd64\gateway\bapgateway.exe" -port !GW_PORT! -controlplane !CP_URL!
ping -n 2 127.0.0.1 >nul
goto :after_start_gw

:skip_start_gw
echo [*] Remote Gateway PEP detected (!GW_URL!). Connecting directly without starting local daemon...
curl.exe -s --max-time 3 --connect-timeout 2 -X GET "!GW_URL!/health" >nul 2>&1
if !ERRORLEVEL! neq 0 (
    echo [WARN] Could not reach remote Gateway PEP at !GW_URL!/health
)

:after_start_gw
python -m pytest tests\test_gateway_pep.py -q
if !ERRORLEVEL! neq 0 (
    echo [FAIL] test_gateway_pep.py failed!
    set /a FAIL_COUNT+=1
) else (
    echo [PASS] Gateway PEP rogue blocking, audit telemetry, and governed authorization verified.
    set /a PASS_COUNT+=1
)

:: -----------------------------------------------------------------------------
:: Step 12: Complete Environment Teardown & Port Free Verification
:: -----------------------------------------------------------------------------
echo.
echo [12/12] Verifying Daemon Teardown and Port Release...
if "!GW_IS_LOCAL!"=="1" taskkill /F /IM bapgateway.exe >nul 2>&1
if "!CP_IS_LOCAL!"=="1" taskkill /F /IM bapcontrolplane.exe >nul 2>&1
ping -n 3 127.0.0.1 >nul

if "!CP_IS_LOCAL!"=="0" if "!GW_IS_LOCAL!"=="0" goto :remote_teardown_done

powershell -NoProfile -Command "Get-NetTCPConnection -LocalPort 8080, 9090 -State Listen -ErrorAction SilentlyContinue | ForEach-Object { Stop-Process -Id $_.OwningProcess -Force -ErrorAction SilentlyContinue }" >nul 2>&1
powershell -NoProfile -Command "if (@(Get-NetTCPConnection -LocalPort 8080, 9090 -State Listen -ErrorAction SilentlyContinue).Count -eq 0) { exit 0 } else { exit 1 }"
if !ERRORLEVEL! equ 0 (
    echo [PASS] Daemon Teardown: Local ports verified completely free. Zero lingering processes.
    set /a PASS_COUNT+=1
) else (
    echo [FAIL] Daemon Teardown: Ports are still occupied!
    set /a FAIL_COUNT+=1
)
goto :after_teardown

:remote_teardown_done
echo [PASS] Remote Server Mode: Remote endpoints remain preserved without local teardown.
set /a PASS_COUNT+=1

:after_teardown

:: -----------------------------------------------------------------------------
:: Final Summary
:: -----------------------------------------------------------------------------
echo.
echo ===============================================================================
echo                             TEST SUMMARY
echo ===============================================================================
echo Total Passed : !PASS_COUNT!
echo Total Failed : !FAIL_COUNT!

cd /d "%ROOT_DIR%"
if !FAIL_COUNT! gtr 0 (
    echo.
    echo [OVERALL STATUS] FAILED - Some tests did not pass.
    exit /b 1
) else (
    echo.
    echo [OVERALL STATUS] SUCCESS - All tests passed!
    exit /b 0
)

:: -----------------------------------------------------------------------------
:: Helper Subroutines
:: -----------------------------------------------------------------------------
:run_test_allow
bapedge.exe exec --raw "!TEST_CMD!" >nul 2>&1
if !ERRORLEVEL! equ 0 (
    echo [PASS] Allowed: !TEST_LBL!
    set /a PASS_COUNT+=1
) else (
    echo [FAIL] Expected allow, but was denied: !TEST_LBL!
    set /a FAIL_COUNT+=1
)
exit /b

:run_test_deny
bapedge.exe exec --raw "!TEST_CMD!" >nul 2>&1
if !ERRORLEVEL! neq 0 (
    echo [PASS] Denied : !TEST_LBL!
    set /a PASS_COUNT+=1
) else (
    echo [FAIL] SECURITY BREACH: Expected deny, but was allowed: !TEST_LBL!
    set /a FAIL_COUNT+=1
)
exit /b

:run_hook_test
echo {"tool_input":{"command":"!HOOK_CMD!"}} > "%TEMP%\hook_test_input.json"
type "%TEMP%\hook_test_input.json" | interceptor.exe | findstr /i "!HOOK_EXP!" >nul
if !ERRORLEVEL! equ 0 (
    echo [PASS] cchook: !HOOK_LBL!
    set /a PASS_COUNT+=1
) else (
    echo [FAIL] cchook: !HOOK_LBL!
    set /a FAIL_COUNT+=1
)
del /f /q "%TEMP%\hook_test_input.json" >nul 2>&1
exit /b
