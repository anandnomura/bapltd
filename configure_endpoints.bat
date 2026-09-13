@echo off
setlocal enabledelayedexpansion

:: ==============================================================================
:: BAP (Bounded Authority Plane) - Central Endpoint Configuration Helper
:: Configures the Central Control Plane host and Gateway URLs for developer laptops.
:: ==============================================================================

echo ===============================================================================
echo   BAP CENTRAL ENDPOINT CONFIGURATION
echo ===============================================================================

set "CLI_CP=%~1"
set "CLI_GW=%~2"

if "%CLI_CP%" neq "" (
    set "NEW_CP=%CLI_CP%"
    if "%CLI_GW%" neq "" (
        set "NEW_GW=%CLI_GW%"
    ) else (
        set "NEW_GW="
    )
    goto :apply_config
)

:: Interactive Prompt Mode
echo Current Configuration:
if exist "bapedge.exe" (
    bapedge.exe config show
) else (
    type bap-config.json 2>nul
)
echo.
echo Enter the new corporate Central Control Plane URL:
echo (Example: https://bap-controlplane.corp.internal:8080 or http://10.20.30.40:8080)
set /p NEW_CP="Control Plane URL: "

if "%NEW_CP%"=="" (
    echo [-] No URL entered. Configuration unchanged.
    exit /b 0
)

echo.
echo Enter the corporate API Gateway URL (or press [ENTER] to keep default):
set /p NEW_GW="Gateway URL [optional]: "

:apply_config
if exist "bapedge.exe" (
    if "%NEW_GW%" neq "" (
        bapedge.exe config set --server "%NEW_CP%" --gateway "%NEW_GW%"
    ) else (
        bapedge.exe config set --server "%NEW_CP%"
    )
) else (
    powershell -NoProfile -Command ^
        "$p = 'bap-config.json'; $json = Get-Content $p -Raw | ConvertFrom-Json; $json.controlplane_url = '%NEW_CP%';" ^
        "if ('%NEW_GW%' -ne '') { $json.gateway_url = '%NEW_GW%' };" ^
        "$json | ConvertTo-Json -Depth 5 | Set-Content $p -Encoding UTF8"
    echo [+] Configuration saved directly to bap-config.json
)

echo.
echo ===============================================================================
echo   SUCCESS: Central BAP endpoints configured!
echo   All Claude Code, Copilot, and Python agents will now connect to:
echo   ==> Control Plane: %NEW_CP%
echo ===============================================================================

