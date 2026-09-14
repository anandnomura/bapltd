@echo off
setlocal EnableDelayedExpansion

set "ROOT_DIR=%~dp0"
cd /d "%ROOT_DIR%"
set "GOTOOLCHAIN=local"

if "%1"=="--all" goto :build_all
if "%1"=="-all" goto :build_all
if "%1"=="all" goto :build_all

echo [1/5] Building bap-controlplane...
cd /d "%ROOT_DIR%bap-controlplane"
go build -o bapcontrolplane.exe ./cmd/server
if %ERRORLEVEL% neq 0 (
    echo Failed to build bapcontrolplane.exe
    exit /b 1
)
copy /y bapcontrolplane.exe "%ROOT_DIR%bapcontrolplane.exe" >nul
copy /y inspector.html "%ROOT_DIR%inspector.html" >nul

echo [2/5] Building bap-edge...
cd /d "%ROOT_DIR%bap-edge"
go build -o bapedge.exe .
if %ERRORLEVEL% neq 0 (
    echo Failed to build bapedge.exe
    exit /b 1
)
copy /y bapedge.exe "%ROOT_DIR%bapedge.exe" >nul
copy /y bapedge.exe "%ROOT_DIR%bapmcp.exe" >nul
copy /y bapedge.exe "%ROOT_DIR%ltd-agent.exe" >nul
copy /y bapedge.exe "%ROOT_DIR%cchook\bapedge.exe" >nul
copy /y bapedge.exe "%ROOT_DIR%cchook\ltd-agent.exe" >nul
copy /y bapedge.exe "%ROOT_DIR%copilot\bapedge.exe" >nul
copy /y bapedge.exe "%ROOT_DIR%copilot\ltd-agent.exe" >nul

echo [3/5] Building bap-gateway...
cd /d "%ROOT_DIR%bap-gateway"
go build -o bapgateway.exe .
if %ERRORLEVEL% neq 0 (
    echo Failed to build bapgateway.exe
    exit /b 1
)
copy /y bapgateway.exe "%ROOT_DIR%bapgateway.exe" >nul

echo [4/5] Building cchook...
cd /d "%ROOT_DIR%cchook"
go build -o interceptor.exe interceptor.go

echo [5/5] Building copilot...
cd /d "%ROOT_DIR%copilot"
go build -o copilot_interceptor.exe copilot_interceptor.go

echo All Windows binaries built and synchronized successfully.
echo (Tip: Run 'build_binaries.bat --all' or 'build_all_platforms.bat' to build Linux and macOS binaries)
exit /b 0

:build_all
call "%ROOT_DIR%build_all_platforms.bat" %*
exit /b %ERRORLEVEL%
