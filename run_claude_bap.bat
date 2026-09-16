@echo off
setlocal EnableDelayedExpansion

set "BAP_HOME=%~dp0"
title Claude Code (Governed by BAP Zero-Trust)

:: ===============================================================================
:: BAP CLAUDE CODE ZERO-TRUST LAUNCHER
::
:: IMPORTANT:
::   - Does NOT create or modify .claude\settings.json
::   - Creates a temporary session-specific Claude settings file
::   - Passes that file to Claude using --settings
::   - Deletes temporary settings when Claude exits
::   - Existing user/project Claude settings are left untouched
:: ===============================================================================

echo ===============================================================================
echo       CLAUDE CODE - BOUNDED AUTHORITY PLANE (BAP) GOVERNANCE LAUNCHER
echo ===============================================================================
echo [*] Working Directory: %CD%
echo.

:: Used for controlled cleanup/exit
set "BAP_EXIT_CODE=0"
set "BAP_SESSION_ACTIVE=0"
set "BAP_TEMP_SETTINGS="


:: ===============================================================================
:: 1. Resolve Linux Control Plane Server URL
:: ===============================================================================

set "SERVER_URL=%~1"

if "!SERVER_URL!"=="" (
    set "SERVER_URL=%BAP_SERVER_URL%"
)

if "!SERVER_URL!"=="" (

    set "CFG_FILE="

    if exist "bap-config.json" (
        set "CFG_FILE=bap-config.json"
    ) else if exist "%BAP_HOME%bap-config.json" (
        set "CFG_FILE=%BAP_HOME%bap-config.json"
    ) else if exist "%USERPROFILE%\bin\bap-config.json" (
        set "CFG_FILE=%USERPROFILE%\bin\bap-config.json"
    )

    if not "!CFG_FILE!"=="" (
        for /f "usebackq delims=" %%U in (`
            powershell -NoProfile -Command ^
            "(Get-Content '!CFG_FILE!' -Raw | ConvertFrom-Json).controlplane_url" 2^>nul
        `) do (
            set "SERVER_URL=%%U"
        )
    )
)

if "!SERVER_URL!"=="" (
    set "SERVER_URL=http://localhost:8080"
)


:: If user ran without args, show current server and allow override.
if "%~1"=="" (

    echo Current Target Server: !SERVER_URL!
    echo Enter Linux server URL or press [ENTER] to use default:

    set /p USER_INPUT="Linux Server URL [default: !SERVER_URL!]: "

    if not "!USER_INPUT!"=="" (
        set "SERVER_URL=!USER_INPUT!"
    )
)


:: Trim trailing slash
if "!SERVER_URL:~-1!"=="/" (
    set "SERVER_URL=!SERVER_URL:~0,-1!"
)


:: ===============================================================================
:: 2. Check Control Plane Connectivity
:: ===============================================================================

echo.
echo [*] Checking connectivity to BAP Control Plane at !SERVER_URL!...


:: Detect localhost target
echo !SERVER_URL! | findstr /i "localhost 127.0.0.1 ::1" >nul 2>&1

if !ERRORLEVEL! equ 0 (

    powershell -NoProfile -Command ^
        "$u = [System.Uri]'!SERVER_URL!';" ^
        "$port = $u.Port;" ^
        "if (Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue) { exit 0 } else { exit 1 }"

    if !ERRORLEVEL! neq 0 (

        echo [*] Launching local BAP Control Plane daemon on !SERVER_URL!...

        powershell -NoProfile -Command ^
            "$u = [System.Uri]'!SERVER_URL!';" ^
            "$port = $u.Port;" ^
            "$isHttps = $u.Scheme -eq 'https';" ^
            "$args = '-port ' + $port + ' -ttl 30 -trust-domain bap.internal';" ^
            "if ($isHttps) { $args += ' -https' };" ^
            "$p = if (Test-Path '%BAP_HOME%dist\windows-amd64\controlplane\bapcontrolplane.exe') {" ^
            "    '%BAP_HOME%dist\windows-amd64\controlplane\bapcontrolplane.exe'" ^
            "} elseif (Test-Path '%BAP_HOME%dist\windows-amd64\bapcontrolplane.exe') {" ^
            "    '%BAP_HOME%dist\windows-amd64\bapcontrolplane.exe'" ^
            "} else {" ^
            "    '%BAP_HOME%bap-controlplane\bapcontrolplane.exe'" ^
            "};" ^
            "Start-Process -FilePath $p -ArgumentList $args -WindowStyle Hidden"

        :: Give local control plane a moment to initialize
        ping -n 3 127.0.0.1 >nul
    )
)


curl.exe -s --max-time 3 --connect-timeout 2 ^
    -X GET "!SERVER_URL!/api/v1/health" ^
    | findstr /i "ok healthy ltd-service" >nul 2>&1


if !ERRORLEVEL! equ 0 (

    echo [+] SUCCESS: Connected to BAP Control Plane.

) else (

    echo [-] WARNING: Could not reach !SERVER_URL!/api/v1/health.
    echo     Please verify the server IP, port, and firewall.
    echo     Proceeding in local-edge mode - Cedar policies still enforced locally.
)


:: ===============================================================================
:: 3. Resolve bapedge Binary
:: ===============================================================================

set "BAPEDGE_BIN="

if exist "%BAP_HOME%dist\windows-amd64\claude-client\bapedge.exe" (

    set "BAPEDGE_BIN=%BAP_HOME%dist\windows-amd64\claude-client\bapedge.exe"

) else if exist "%BAP_HOME%dist\windows-amd64\bapedge.exe" (

    set "BAPEDGE_BIN=%BAP_HOME%dist\windows-amd64\bapedge.exe"

) else if exist "%BAP_HOME%bapedge.exe" (

    set "BAPEDGE_BIN=%BAP_HOME%bapedge.exe"

) else if exist "%USERPROFILE%\bin\bapedge.exe" (

    set "BAPEDGE_BIN=%USERPROFILE%\bin\bapedge.exe"

) else (

    where bapedge.exe >nul 2>&1

    if !ERRORLEVEL! equ 0 (
        set "BAPEDGE_BIN=bapedge.exe"
    )
)


:: Persist selected server for bapedge/cchook.
if not "!BAPEDGE_BIN!"=="" (

    "!BAPEDGE_BIN!" config set --server "!SERVER_URL!" >nul 2>&1

) else (

    powershell -NoProfile -Command ^
        "$p = if (Test-Path 'bap-config.json') {" ^
        "    'bap-config.json'" ^
        "} elseif (Test-Path '%BAP_HOME%bap-config.json') {" ^
        "    '%BAP_HOME%bap-config.json'" ^
        "} else {" ^
        "    $null" ^
        "};" ^
        "if ($p) {" ^
        "    $j = Get-Content $p -Raw | ConvertFrom-Json;" ^
        "    $j.controlplane_url = '!SERVER_URL!';" ^
        "    $j | ConvertTo-Json -Depth 5 | Set-Content $p -Encoding UTF8" ^
        "}" >nul 2>&1
)


:: ===============================================================================
:: 4. Resolve cchook Interceptor Binary
:: ===============================================================================

set "INTERCEPTOR_BIN="

if exist "%BAP_HOME%dist\windows-amd64\claude-client\cchook\interceptor.exe" (

    set "INTERCEPTOR_BIN=%BAP_HOME%dist\windows-amd64\claude-client\cchook\interceptor.exe"

) else if exist "%BAP_HOME%dist\windows-amd64\cchook-interceptor.exe" (

    set "INTERCEPTOR_BIN=%BAP_HOME%dist\windows-amd64\cchook-interceptor.exe"

) else if exist "%BAP_HOME%interceptor.exe" (

    set "INTERCEPTOR_BIN=%BAP_HOME%interceptor.exe"

) else if exist "%USERPROFILE%\bin\interceptor.exe" (

    set "INTERCEPTOR_BIN=%USERPROFILE%\bin\interceptor.exe"

) else if exist "%BAP_HOME%cchook\interceptor.exe" (

    set "INTERCEPTOR_BIN=%BAP_HOME%cchook\interceptor.exe"

) else (

    where interceptor.exe >nul 2>&1

    if !ERRORLEVEL! equ 0 (
        set "INTERCEPTOR_BIN=interceptor.exe"
    )
)


:: ===============================================================================
:: 5. Build Safe Hook Command
:: ===============================================================================

set "SAFE_HOOK_CMD=interceptor.exe"

if not "!INTERCEPTOR_BIN!"=="" (

    :: Convert Windows backslashes to forward slashes.
    set "INTERCEPTOR_SAFE=!INTERCEPTOR_BIN:\=/!"

    :: Quote executable path inside JSON.
    set "SAFE_HOOK_CMD=\"!INTERCEPTOR_SAFE!\""
)

echo [*] BAP Hook Command: !SAFE_HOOK_CMD!


:: ===============================================================================
:: 6. Generate BAP Session ID
:: ===============================================================================

set "RND=%RANDOM%"
set "TME=%TIME::=%"
set "TME=!TME: =0!"
set "TME=!TME:.=%"

set "BAP_SESSION_ID=sess-claude-!RND!-!TME!"
set "BAP_SERVER_URL=!SERVER_URL!"


:: ===============================================================================
:: 7. Create TEMPORARY Claude Settings File
::
:: We deliberately DO NOT write:
::
::      .claude\settings.json
::
:: The BAP hooks exist only because this launcher passes this file using:
::
::      claude --settings <temporary file>
::
:: ===============================================================================

set "BAP_TEMP_SETTINGS=%TEMP%\bap-claude-settings-!RND!-!TME!.json"

echo.
echo [*] Creating temporary BAP Claude settings:
echo     !BAP_TEMP_SETTINGS!


(
    echo {
    echo   "disableAllHooks": false,
    echo   "hooks": {
    echo     "SessionStart": [
    echo       {
    echo         "hooks": [
    echo           {
    echo             "type": "command",
    echo             "command": "!SAFE_HOOK_CMD!"
    echo           }
    echo         ]
    echo       }
    echo     ],
    echo     "UserPromptSubmit": [
    echo       {
    echo         "hooks": [
    echo           {
    echo             "type": "command",
    echo             "command": "!SAFE_HOOK_CMD!"
    echo           }
    echo         ]
    echo       }
    echo     ],
    echo     "PreToolUse": [
    echo       {
    echo         "matcher": "Bash^|Read^|View^|Edit^|Write",
    echo         "hooks": [
    echo           {
    echo             "type": "command",
    echo             "command": "!SAFE_HOOK_CMD!"
    echo           }
    echo         ]
    echo       }
    echo     ]
    echo   }
    echo }
) > "!BAP_TEMP_SETTINGS!"


:: Verify that the temporary settings file is valid JSON.
powershell -NoProfile -Command ^
    "try { Get-Content '!BAP_TEMP_SETTINGS!' -Raw | ConvertFrom-Json | Out-Null; exit 0 } catch { exit 1 }"


if !ERRORLEVEL! neq 0 (

    echo.
    echo [-] ERROR: Temporary Claude BAP settings file is invalid JSON.
    echo     !BAP_TEMP_SETTINGS!

    set "BAP_EXIT_CODE=10"
    goto BAP_CLEANUP
)


echo [+] Temporary BAP hook settings created successfully.


:: ===============================================================================
:: 8. Zero-Trust Session Pre-Flight
:: ===============================================================================

if not "!BAPEDGE_BIN!"=="" (

    "!BAPEDGE_BIN!" session-start ^
        --server "!SERVER_URL!" ^
        --session-id "!BAP_SESSION_ID!" ^
        --app-id "claude-code"

    set "SESSION_START_RC=!ERRORLEVEL!"


    :: Policy explicitly denied session startup.
    if "!SESSION_START_RC!"=="2" (

        echo.
        echo [-] BAP denied Claude Code session startup.
        echo [-] Claude Code will NOT be launched.

        set "BAP_EXIT_CODE=2"

        goto BAP_CLEANUP
    )


    :: Treat successful enrollment as active.
    if "!SESSION_START_RC!"=="0" (
        set "BAP_SESSION_ACTIVE=1"
    )

) else (

    :: Edge binary unavailable.
    :: Leave local session information for cchook/interceptor.

    (
        echo {
        echo   "session_id": "!BAP_SESSION_ID!",
        echo   "server_url": "!SERVER_URL!",
        echo   "user": "%USERNAME%",
        echo   "hostname": "%COMPUTERNAME%"
        echo }
    ) > ".bap-session.json"

    set "BAP_SESSION_ACTIVE=1"
)


:: ===============================================================================
:: 9. Show Session Information
:: ===============================================================================

echo.
echo ===============================================================================
echo   CLAUDE CODE IS NOW GOVERNED BY BAP ZERO-TRUST
echo ===============================================================================
echo   Target Control Plane : !SERVER_URL!
echo   Active Session ID    : !BAP_SESSION_ID!
echo   User Identity        : %USERNAME% on %COMPUTERNAME%
echo   Policy Enforcement   : Local Cedar Broker (^< 1.5ms)
echo   Live Telemetry       : Streaming to !SERVER_URL!
echo   Liveness Watcher     : Active
echo.
echo   Claude Settings      : TEMPORARY SESSION SETTINGS
echo   Settings File        : !BAP_TEMP_SETTINGS!
echo   Project settings     : NOT MODIFIED
echo.
echo   Dashboard UI         : start bapdashboard separately
echo                          default https://localhost:8444/dashboard/
echo.
echo   You will see your active Claude Code session glowing on the Live Radar!
echo ===============================================================================
echo.


:: ===============================================================================
:: 10. Auto-detect Claude Executable
:: ===============================================================================

set "CLAUDE_BIN="


where claude-code.cmd >nul 2>&1

if !ERRORLEVEL! equ 0 (

    set "CLAUDE_BIN=claude-code.cmd"

) else (

    where claude >nul 2>&1

    if !ERRORLEVEL! equ 0 (

        set "CLAUDE_BIN=claude"

    ) else (

        where claude.exe >nul 2>&1

        if !ERRORLEVEL! equ 0 (

            set "CLAUDE_BIN=claude.exe"

        ) else (

            echo.
            echo [-] ERROR: Claude Code executable could not be found.
            echo     Tried:
            echo       claude-code.cmd
            echo       claude
            echo       claude.exe
            echo.

            set "BAP_EXIT_CODE=11"

            goto BAP_CLEANUP
        )
    )
)


echo [*] Claude executable: !CLAUDE_BIN!
echo.


:: ===============================================================================
:: 11. Launch Claude Code
::
:: IMPORTANT:
::
:: Put BAP --settings AFTER forwarded arguments.
::
:: That makes the BAP session settings the final explicit settings argument
:: supplied by this launcher.
::
:: ===============================================================================

echo [*] Starting Claude Code with BAP session governance...
echo.

call !CLAUDE_BIN! %* --settings "!BAP_TEMP_SETTINGS!"


:: Preserve Claude's exit code.
set "CLAUDE_EXIT_CODE=!ERRORLEVEL!"

echo.
echo [*] Claude Code exited with code !CLAUDE_EXIT_CODE!.

set "BAP_EXIT_CODE=!CLAUDE_EXIT_CODE!"


:: ===============================================================================
:: 12. Common Cleanup
:: ===============================================================================

:BAP_CLEANUP

echo.


:: ------------------------------------------------------------------------------
:: End BAP session if one was started
:: ------------------------------------------------------------------------------

if "!BAP_SESSION_ACTIVE!"=="1" (

    echo [*] Deregistering session !BAP_SESSION_ID! from BAP...

    if not "!BAPEDGE_BIN!"=="" (

        "!BAPEDGE_BIN!" session-end ^
            --server "!SERVER_URL!" ^
            --session-id "!BAP_SESSION_ID!" >nul 2>&1

    ) else (

        set "CURL_CA_ARG="

        if exist "controlplane-cert.pem" (

            set "CURL_CA_ARG=--cacert controlplane-cert.pem"

        ) else (

            set "CURL_CA_ARG=-k"
        )


        curl.exe -s !CURL_CA_ARG! ^
            --max-time 2 ^
            --connect-timeout 2 ^
            -X POST "!SERVER_URL!/api/v1/sessions/end" ^
            -H "Content-Type: application/json" ^
            -d "{\"session_id\":\"!BAP_SESSION_ID!\",\"reason\":\"Claude Code session ended\"}" ^
            >nul 2>&1
    )
)


:: ------------------------------------------------------------------------------
:: Delete temporary Claude session settings
:: ------------------------------------------------------------------------------

if not "!BAP_TEMP_SETTINGS!"=="" (

    if exist "!BAP_TEMP_SETTINGS!" (

        del /f /q "!BAP_TEMP_SETTINGS!" >nul 2>&1

        if exist "!BAP_TEMP_SETTINGS!" (

            echo [-] WARNING: Could not remove temporary BAP Claude settings:
            echo     !BAP_TEMP_SETTINGS!

        ) else (

            echo [+] Removed temporary BAP Claude hook settings.
        )
    )
)


:: ------------------------------------------------------------------------------
:: Remove temporary BAP workspace files
:: ------------------------------------------------------------------------------

if exist ".bap-session.json" (
    del /f /q ".bap-session.json" >nul 2>&1
)

if exist ".bap-prompt.txt" (
    del /f /q ".bap-prompt.txt" >nul 2>&1
)


echo [+] BAP session cleanup complete.
echo [+] Existing .claude\settings.json was not modified.
echo.

endlocal & exit /b %BAP_EXIT_CODE%