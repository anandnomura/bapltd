@echo off
setlocal enabledelayedexpansion

:: ==============================================================================
:: BAP (Bounded Authority Plane) - Launch Envoy Proxy on Podman / Docker (Windows)
:: Runs Envoy on port 10000 with ext_authz linked to BAP Control Plane on port 8080.
:: ==============================================================================

set "CONTAINER_NAME=bap-envoy-pep"
set "ENVOY_IMAGE=docker.io/envoyproxy/envoy:v1.31-latest"
set "SCRIPT_DIR=%~dp0"
set "CONFIG_FILE=%SCRIPT_DIR%envoy.yaml"

echo ===============================================================================
echo    BAP OPTION 1: ENVOY PROXY ON PODMAN / DOCKER (WINDOWS)
echo    Zero-Trust Ingress Gateway PEP (ext_authz Filter)
echo ===============================================================================

:: 1. Detect Container Engine (podman preferred, fallback to docker)
where podman >nul 2>&1
if %ERRORLEVEL% equ 0 (
    set "ENGINE=podman"
) else (
    where docker >nul 2>&1
    if %ERRORLEVEL% equ 0 (
        set "ENGINE=docker"
    ) else (
        echo [ERROR] Neither 'podman' nor 'docker' was found in your PATH.
        echo         To use Option 1, install Podman Desktop or Docker Desktop.
        echo         NOTE: Option 3 (bapgateway.exe) does NOT require any container engine!
        exit /b 1
    )
)
echo [+] Detected container engine: %ENGINE%

:: 2. Check if BAP Control Plane is running on port 8080
powershell -NoProfile -Command "try { $r = Invoke-WebRequest -Uri 'http://localhost:8080/api/v1/health' -TimeoutSec 2 -UseBasicParsing; exit 0 } catch { exit 1 }" >nul 2>&1
if %ERRORLEVEL% neq 0 (
    echo [WARNING] BAP Control Plane does not appear to be running on http://localhost:8080.
    echo           Please start bapcontrolplane.exe first:
    echo           .\bap-controlplane\bapcontrolplane.exe -port 8080
)

:: 3. Remove existing container if running
%ENGINE% rm -f %CONTAINER_NAME% >nul 2>&1

:: 4. Convert Windows path to container mount format
for /f "usebackq tokens=*" %%P in (`powershell -NoProfile -Command "$p = (Resolve-Path '%CONFIG_FILE%').Path -replace '\\', '/'; Write-Output $p"`) do set "MOUNT_PATH=%%P"

echo [*] Launching Envoy PEP container on port 10000...
%ENGINE% run -d --name %CONTAINER_NAME% -p 10000:10000 --add-host host.containers.internal:host-gateway -v "%MOUNT_PATH%":/etc/envoy/envoy.yaml:ro %ENVOY_IMAGE% envoy -c /etc/envoy/envoy.yaml --log-level info

if %ERRORLEVEL% equ 0 (
    echo [+] Envoy PEP successfully launched!
    echo     - Envoy Ingress URL    : http://localhost:10000/api/v1/financial-records
    echo     - Control Plane Target : http://host.containers.internal:8080/api/v1/auth/envoy
    echo     - View logs            : %ENGINE% logs -f %CONTAINER_NAME%
    echo     - Stop container       : run stop_envoy_podman.bat
) else (
    echo [ERROR] Failed to start Envoy container.
)
echo ===============================================================================

