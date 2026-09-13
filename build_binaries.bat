@echo off
setlocal EnableDelayedExpansion

set "ROOT_DIR=%~dp0"
cd /d "%ROOT_DIR%"

echo [1/4] Building bap-controlplane...
cd /d "%ROOT_DIR%bap-controlplane"
go build -o bapcontrolplane.exe ./cmd/server
if %ERRORLEVEL% neq 0 (
    echo Failed to build bapcontrolplane.exe
    exit /b 1
)
copy /y bapcontrolplane.exe "%ROOT_DIR%bapcontrolplane.exe" >nul
copy /y inspector.html "%ROOT_DIR%inspector.html" >nul

echo [2/4] Building bap-edge...
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

echo [3/4] Building cchook...
cd /d "%ROOT_DIR%cchook"
go build -o interceptor.exe interceptor.go

echo [4/4] Building copilot...
cd /d "%ROOT_DIR%copilot"
go build -o copilot_interceptor.exe copilot_interceptor.go

echo All binaries built and synchronized successfully.

