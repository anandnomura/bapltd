@echo off
setlocal EnableDelayedExpansion

set "ROOT_DIR=%~dp0"
cd /d "%ROOT_DIR%"

echo ===============================================================================
echo   Bounded Authority Plane (BAP) - Cross-Platform Multi-OS Build
echo   Building Windows, Linux (amd64 + arm64), and macOS (Intel + Apple Silicon)
echo ===============================================================================

powershell -NoProfile -ExecutionPolicy Bypass -File "%ROOT_DIR%build_all_platforms.ps1" %*

if %ERRORLEVEL% neq 0 (
    echo [ERROR] Multi-platform compilation failed.
    exit /b %ERRORLEVEL%
)

echo [SUCCESS] Multi-platform build complete. Binaries available in dist\

