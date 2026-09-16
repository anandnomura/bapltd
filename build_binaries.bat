@echo off
setlocal EnableDelayedExpansion

set "ROOT_DIR=%~dp0"
cd /d "%ROOT_DIR%"
set "GOTOOLCHAIN=auto"

if "%1"=="--all" goto :build_all
if "%1"=="-all" goto :build_all
echo [0/7] Ensuring BAP Root CA and TLS credentials...
powershell -NoProfile -ExecutionPolicy Bypass -File "%ROOT_DIR%scripts\ensure_certs.ps1"
if %ERRORLEVEL% neq 0 (
    echo [WARNING] ensure_certs.ps1 failed, continuing build...
)

echo [1/7] Building dashboard assets...
cd /d "%ROOT_DIR%dashboard"
call npm run build
if %ERRORLEVEL% neq 0 exit /b 1

echo [2/7] Building bap-controlplane and standalone dashboard...
cd /d "%ROOT_DIR%bap-controlplane"
go build -o bapcontrolplane.exe ./cmd/server
if %ERRORLEVEL% neq 0 (
    echo Failed to build bapcontrolplane.exe
    exit /b 1
)
copy /y bapcontrolplane.exe "%ROOT_DIR%bapcontrolplane.exe" >nul
go build -o bapdashboard.exe ./cmd/dashboard
if %ERRORLEVEL% neq 0 (
    echo Failed to build bapdashboard.exe
    exit /b 1
)
copy /y bapdashboard.exe "%ROOT_DIR%bapdashboard.exe" >nul

echo [3/7] Building bap-edge...
cd /d "%ROOT_DIR%bap-edge"
go build -o bapedge.exe .
if %ERRORLEVEL% neq 0 (
    echo Failed to build bapedge.exe
    exit /b 1
)
copy /y bapedge.exe "%ROOT_DIR%bapedge.exe" >nul 2>&1 || (
    del /f /q "%ROOT_DIR%bapedge.exe.old" >nul 2>&1
    ren "%ROOT_DIR%bapedge.exe" bapedge.exe.old >nul 2>&1
    copy /y bapedge.exe "%ROOT_DIR%bapedge.exe" >nul
)
copy /y bapedge.exe "%ROOT_DIR%bapmcp.exe" >nul 2>&1 || (
    del /f /q "%ROOT_DIR%bapmcp.exe.old" >nul 2>&1
    ren "%ROOT_DIR%bapmcp.exe" bapmcp.exe.old >nul 2>&1
    copy /y bapedge.exe "%ROOT_DIR%bapmcp.exe" >nul
)
copy /y bapedge.exe "%ROOT_DIR%ltd-agent.exe" >nul 2>&1 || (
    del /f /q "%ROOT_DIR%ltd-agent.exe.old" >nul 2>&1
    ren "%ROOT_DIR%ltd-agent.exe" ltd-agent.exe.old >nul 2>&1
    copy /y bapedge.exe "%ROOT_DIR%ltd-agent.exe" >nul
)
copy /y bapedge.exe "%ROOT_DIR%cchook\bapedge.exe" >nul 2>&1 || (
    del /f /q "%ROOT_DIR%cchook\bapedge.exe.old" >nul 2>&1
    ren "%ROOT_DIR%cchook\bapedge.exe" bapedge.exe.old >nul 2>&1
    copy /y bapedge.exe "%ROOT_DIR%cchook\bapedge.exe" >nul
)
copy /y bapedge.exe "%ROOT_DIR%cchook\ltd-agent.exe" >nul 2>&1 || (
    del /f /q "%ROOT_DIR%cchook\ltd-agent.exe.old" >nul 2>&1
    ren "%ROOT_DIR%cchook\ltd-agent.exe" ltd-agent.exe.old >nul 2>&1
    copy /y bapedge.exe "%ROOT_DIR%cchook\ltd-agent.exe" >nul
)
copy /y bapedge.exe "%ROOT_DIR%copilot\bapedge.exe" >nul 2>&1 || (
    del /f /q "%ROOT_DIR%copilot\bapedge.exe.old" >nul 2>&1
    ren "%ROOT_DIR%copilot\bapedge.exe" bapedge.exe.old >nul 2>&1
    copy /y bapedge.exe "%ROOT_DIR%copilot\bapedge.exe" >nul
)
copy /y bapedge.exe "%ROOT_DIR%copilot\ltd-agent.exe" >nul 2>&1 || (
    del /f /q "%ROOT_DIR%copilot\ltd-agent.exe.old" >nul 2>&1
    ren "%ROOT_DIR%copilot\ltd-agent.exe" ltd-agent.exe.old >nul 2>&1
    copy /y bapedge.exe "%ROOT_DIR%copilot\ltd-agent.exe" >nul
)

echo [4/7] Building bap-gateway...
cd /d "%ROOT_DIR%bap-gateway"
go build -o bapgateway.exe .
if %ERRORLEVEL% neq 0 (
    echo Failed to build bapgateway.exe
    exit /b 1
)
copy /y bapgateway.exe "%ROOT_DIR%bapgateway.exe" >nul

echo [5/7] Building cchook...
cd /d "%ROOT_DIR%cchook"
go build -o interceptor.exe interceptor.go

echo [6/7] Building copilot...
cd /d "%ROOT_DIR%copilot"
go build -o copilot_interceptor.exe copilot_interceptor.go

echo [7/7] Synchronizing Inspector cockpits and assets...
copy /y "%ROOT_DIR%inspector.html" "%ROOT_DIR%bap-controlplane\inspector.html" >nul
copy /y "%ROOT_DIR%inspector_v2.html" "%ROOT_DIR%bap-controlplane\inspector_v2.html" >nul
for /r "%ROOT_DIR%dist" %%F in (inspector.html) do if exist "%%F" copy /y "%ROOT_DIR%inspector.html" "%%F" >nul 2>&1
for /r "%ROOT_DIR%dist" %%F in (inspector_v2.html) do if exist "%%F" copy /y "%ROOT_DIR%inspector_v2.html" "%%F" >nul 2>&1
if exist "%ROOT_DIR%dist\windows-amd64" (
    copy /y "%ROOT_DIR%bapcontrolplane.exe" "%ROOT_DIR%dist\windows-amd64\bapcontrolplane.exe" >nul 2>&1
    copy /y "%ROOT_DIR%bapedge.exe" "%ROOT_DIR%dist\windows-amd64\bapedge.exe" >nul 2>&1
    copy /y "%ROOT_DIR%bapdashboard.exe" "%ROOT_DIR%dist\windows-amd64\bapdashboard.exe" >nul 2>&1
    copy /y "%ROOT_DIR%bapgateway.exe" "%ROOT_DIR%dist\windows-amd64\bapgateway.exe" >nul 2>&1
    copy /y "%ROOT_DIR%inspector.html" "%ROOT_DIR%dist\windows-amd64\inspector.html" >nul 2>&1
    copy /y "%ROOT_DIR%inspector_v2.html" "%ROOT_DIR%dist\windows-amd64\inspector_v2.html" >nul 2>&1
)
if exist "%ROOT_DIR%dist\windows-amd64\controlplane" (
    copy /y "%ROOT_DIR%bapcontrolplane.exe" "%ROOT_DIR%dist\windows-amd64\controlplane\bapcontrolplane.exe" >nul 2>&1
    copy /y "%ROOT_DIR%inspector.html" "%ROOT_DIR%dist\windows-amd64\controlplane\inspector.html" >nul 2>&1
    copy /y "%ROOT_DIR%inspector_v2.html" "%ROOT_DIR%dist\windows-amd64\controlplane\inspector_v2.html" >nul 2>&1
)
if exist "%ROOT_DIR%dist\windows-amd64\claude-client" (
    copy /y "%ROOT_DIR%bapedge.exe" "%ROOT_DIR%dist\windows-amd64\claude-client\bapedge.exe" >nul 2>&1
    copy /y "%ROOT_DIR%cchook\interceptor.exe" "%ROOT_DIR%dist\windows-amd64\claude-client\cchook\interceptor.exe" >nul 2>&1
)
if exist "%ROOT_DIR%dist\windows-amd64\dashboard" (
    copy /y "%ROOT_DIR%bapdashboard.exe" "%ROOT_DIR%dist\windows-amd64\dashboard\bapdashboard.exe" >nul 2>&1
    copy /y "%ROOT_DIR%inspector.html" "%ROOT_DIR%dist\windows-amd64\dashboard\inspector.html" >nul 2>&1
    copy /y "%ROOT_DIR%inspector_v2.html" "%ROOT_DIR%dist\windows-amd64\dashboard\inspector_v2.html" >nul 2>&1
)
if exist "%ROOT_DIR%dist\windows-amd64\gateway" (
    copy /y "%ROOT_DIR%bapgateway.exe" "%ROOT_DIR%dist\windows-amd64\gateway\bapgateway.exe" >nul 2>&1
)

powershell -NoProfile -ExecutionPolicy Bypass -File "%ROOT_DIR%scripts\check_stale_copies.ps1"
if %ERRORLEVEL% neq 0 (
    echo [ERROR] Stale asset audit detected mismatches!
    exit /b %ERRORLEVEL%
)

echo All Windows binaries built and synchronized successfully.
echo (Tip: Run 'build_binaries.bat --all' or 'build_all_platforms.bat' to build Linux and macOS binaries)
exit /b 0

:build_all
call "%ROOT_DIR%build_all_platforms.bat" %*
exit /b %ERRORLEVEL%
