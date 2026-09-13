@echo off
setlocal

set "CONTAINER_NAME=bap-envoy-pep"

where podman >nul 2>&1
if %ERRORLEVEL% equ 0 (
    podman rm -f %CONTAINER_NAME% >nul 2>&1
    echo [+] Podman container '%CONTAINER_NAME%' stopped and removed.
    exit /b 0
)

where docker >nul 2>&1
if %ERRORLEVEL% equ 0 (
    docker rm -f %CONTAINER_NAME% >nul 2>&1
    echo [+] Docker container '%CONTAINER_NAME%' stopped and removed.
    exit /b 0
)

echo [-] No container runtime found.

