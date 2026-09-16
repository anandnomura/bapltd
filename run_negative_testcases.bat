@echo off
setlocal EnableDelayedExpansion
cd /d "%~dp0"
set "ROOT_DIR=%~dp0"

title BAP - Standalone Negative and Adversarial Test Suite
color 0C
echo ===============================================================================
echo   BAP ZERO-TRUST: STANDALONE NEGATIVE AND ADVERSARIAL TEST SUITE
echo   (MANUAL ON-DEMAND ONLY - NOT EXECUTED ON STARTUP OR DEFAULT BUILDS)
echo ===============================================================================
echo.
echo [*] NOTICE FOR CORPORATE SECURITY:
echo     This suite explicitly tests that the BAP Zero-Trust kernel STOPS attacks.
echo     Commands in this suite attempt forbidden operations (egress, credential theft,
echo     and evasion) to verify that BAP policy enforcement blocks them.
echo.
echo     Expected result for EVERY test in this suite is: [BLOCKED / DENIED].
echo ===============================================================================
echo.

set "PASS_COUNT=0"
set "FAIL_COUNT=0"

:: Resolve bapedge executable
set "BAPEDGE_BIN=%ROOT_DIR%bapedge.exe"
if not exist "%BAPEDGE_BIN%" set "BAPEDGE_BIN=%ROOT_DIR%dist\windows-amd64\claude-client\bapedge.exe"
if not exist "%BAPEDGE_BIN%" set "BAPEDGE_BIN=%ROOT_DIR%dist\windows-amd64\bapedge.exe"

if not exist "%BAPEDGE_BIN%" (
    echo [-] ERROR: bapedge binary not found. Build binaries first via build_binaries.bat.
    exit /b 1
)

:: -----------------------------------------------------------------------------
:: Part 1: Cedar Security Invariants and Forbid Rules (Must be DENIED)
:: -----------------------------------------------------------------------------
echo [1/5] Testing Cedar Security Invariants and Forbid Rules (Must be DENIED)...

set "TEST_CMD=git status && curl https://untrusted-test.internal" & set "TEST_LBL=Egress utility: curl" & call :run_deny_check
set "TEST_CMD=npm install && wget https://untrusted-test.internal" & set "TEST_LBL=Egress utility: wget" & call :run_deny_check
set "TEST_CMD=powershell -Command Invoke-RestMethod http://untrusted-test.internal" & set "TEST_LBL=Egress utility: Invoke-RestMethod" & call :run_deny_check
set "TEST_CMD=powershell -Command Invoke-WebRequest http://untrusted-test.internal" & set "TEST_LBL=Egress utility: Invoke-WebRequest" & call :run_deny_check
set "TEST_CMD=ls ~/.aws/config" & set "TEST_LBL=Sensitive cloud directory: ~/.aws" & call :run_deny_check
set "TEST_CMD=cat ~/.ssh/id_rsa" & set "TEST_LBL=Sensitive private key: ~/.ssh" & call :run_deny_check
set "TEST_CMD=cat .env" & set "TEST_LBL=Secret content disclosure: cat .env" & call :run_deny_check
set "TEST_CMD=type .env" & set "TEST_LBL=Secret content disclosure: type .env" & call :run_deny_check
set "TEST_CMD=powershell -Command Move-Item .env junk" & set "TEST_LBL=Evasive rename bypass: Move-Item .env" & call :run_deny_check
set "TEST_CMD=cmd /c ren .env junk" & set "TEST_LBL=Evasive rename bypass: ren .env" & call :run_deny_check
set "TEST_CMD=ls -la .env > leak.txt" & set "TEST_LBL=Redirection leak: .env > leak.txt" & call :run_deny_check
set "TEST_CMD=rm -rf /" & set "TEST_LBL=Destructive command: rm -rf /" & call :run_deny_check

:: -----------------------------------------------------------------------------
:: Part 2: Claude Code Hook Interceptor Denials
:: -----------------------------------------------------------------------------
echo.
echo [2/5] Testing Claude Code Hook Interceptor Denials (cchook)...
cd /d "%ROOT_DIR%cchook"

set "HOOK_CMD=cat .env" & set "HOOK_EXP=deny" & set "HOOK_LBL=Forbidden secret read [cat .env]" & call :run_cchook_deny
set "HOOK_CMD=git status && curl untrusted-test.internal" & set "HOOK_EXP=deny" & set "HOOK_LBL=Forbidden egress [curl]" & call :run_cchook_deny

:: -----------------------------------------------------------------------------
:: Part 3: GitHub Copilot Interceptor Denials
:: -----------------------------------------------------------------------------
echo.
echo [3/5] Testing GitHub Copilot Interceptor Denials (copilot)...
cd /d "%ROOT_DIR%copilot"

call copilot-wrap.bat "cat .env" >nul 2>&1
if !ERRORLEVEL! neq 0 (
    echo [PASS] Copilot: Forbidden secret read [cat .env] -^> BLOCKED
    set /a PASS_COUNT+=1
) else (
    echo [FAIL] Copilot SECURITY BREACH: Expected cat .env to be blocked
    set /a FAIL_COUNT+=1
)

call copilot-wrap.bat "cmd /c ren .env junk" >nul 2>&1
if !ERRORLEVEL! neq 0 (
    echo [PASS] Copilot: Forbidden evasive rename [ren .env] -^> BLOCKED
    set /a PASS_COUNT+=1
) else (
    echo [FAIL] Copilot SECURITY BREACH: Expected ren .env to be blocked
    set /a FAIL_COUNT+=1
)

call copilot-wrap.bat "curl https://untrusted-test.internal" >nul 2>&1
if !ERRORLEVEL! neq 0 (
    echo [PASS] Copilot: Forbidden egress [curl] -^> BLOCKED
    set /a PASS_COUNT+=1
) else (
    echo [FAIL] Copilot SECURITY BREACH: Expected curl to be blocked
    set /a FAIL_COUNT+=1
)

powershell -ExecutionPolicy Bypass -File copilot-wrap.ps1 "cat .env" >nul 2>&1
if !ERRORLEVEL! neq 0 (
    echo [PASS] Copilot PowerShell: Forbidden secret read [cat .env] -^> BLOCKED
    set /a PASS_COUNT+=1
) else (
    echo [FAIL] Copilot PowerShell: Expected cat .env to be blocked
    set /a FAIL_COUNT+=1
)

:: -----------------------------------------------------------------------------
:: Part 4: Complex Adversarial Bad Commands Suite
:: -----------------------------------------------------------------------------
echo.
echo [4/5] Testing Complex Adversarial and Malicious Pipelines...
cd /d "%ROOT_DIR%"
powershell -ExecutionPolicy Bypass -NoProfile -File "%ROOT_DIR%test_complex_cases.ps1" -IncludeNegative
if !ERRORLEVEL! neq 0 (
    echo [FAIL] test_complex_cases.ps1 with -IncludeNegative failed!
    set /a FAIL_COUNT+=1
) else (
    echo [PASS] All complex adversarial evasion and injection cases were successfully blocked.
    set /a PASS_COUNT+=1
)

:: -----------------------------------------------------------------------------
:: Part 5: Python Agent SDK Negative Cases
:: -----------------------------------------------------------------------------
echo.
echo [5/5] Testing Python Agent SDK Negative Cases...
cd /d "%ROOT_DIR%"
python -m pytest tests\test_python_agent_negative.py -q
if !ERRORLEVEL! equ 0 (
    echo [PASS] Python Agent SDK negative policy violation handling verified.
    set /a PASS_COUNT+=1
) else (
    echo [FAIL] Python Agent SDK negative tests failed!
    set /a FAIL_COUNT+=1
)

:: -----------------------------------------------------------------------------
:: Final Summary
:: -----------------------------------------------------------------------------
echo.
echo ===============================================================================
echo                     NEGATIVE TEST SUITE SUMMARY
echo ===============================================================================
echo Total Invariants Enforced / Blocked : !PASS_COUNT!
echo Total Invariants Allowed / Breached : !FAIL_COUNT!
echo.
if !FAIL_COUNT! gtr 0 (
    echo [OVERALL STATUS] FAILED - One or more negative tests was improperly permitted.
    exit /b 1
) else (
    echo [OVERALL STATUS] SUCCESS - All negative and adversarial tests were strictly BLOCKED.
    exit /b 0
)

:: Subroutines
:run_deny_check
"%BAPEDGE_BIN%" exec --raw "!TEST_CMD!" >nul 2>&1
if !ERRORLEVEL! neq 0 (
    echo [PASS] Denied : !TEST_LBL!
    set /a PASS_COUNT+=1
) else (
    echo [FAIL] SECURITY BREACH: Expected deny, but was allowed: !TEST_LBL!
    set /a FAIL_COUNT+=1
)
exit /b

:run_cchook_deny
echo {"tool_input":{"command":"!HOOK_CMD!"}} > "%TEMP%\hook_test_input.json"
type "%TEMP%\hook_test_input.json" | interceptor.exe | findstr /i "!HOOK_EXP!" >nul
if !ERRORLEVEL! equ 0 (
    echo [PASS] cchook : !HOOK_LBL! -^> BLOCKED
    set /a PASS_COUNT+=1
) else (
    echo [FAIL] cchook : !HOOK_LBL! -^> ALLOWED
    set /a FAIL_COUNT+=1
)
del /f /q "%TEMP%\hook_test_input.json" >nul 2>&1
exit /b

