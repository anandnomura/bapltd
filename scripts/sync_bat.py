import os
import shutil

preamble = """@echo off
setlocal EnableExtensions

rem ==============================================================================
rem BAP Claude Code Transactional Zero-Trust Launcher
rem SELF-CONTAINED BAT PACKAGE (Supports Multi-Instance Claude Concurrency)
rem
rem Usage:
rem   run_claude_bap.bat
rem   run_claude_bap.bat --server https://localhost:8443
rem   run_claude_bap.bat --model sonnet
rem ==============================================================================

set "SCRIPT_DIR=%~dp0"
set "BAP_ORIGINAL_DIR=%SCRIPT_DIR%"
set "PS1_FILE=%SCRIPT_DIR%run_claude_bap.ps1"

if exist "%PS1_FILE%" (
    powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%PS1_FILE%" %*
    exit /b %ERRORLEVEL%
)

set "BAP_SELF=%~f0"
set "BAP_EXTRACT=%TEMP%\\bap-claude-session-guard-%RANDOM%-%RANDOM%.ps1"

powershell.exe -NoProfile -ExecutionPolicy Bypass -Command ^
  "$src=$env:BAP_SELF; $dst=$env:BAP_EXTRACT; " ^
  "$lines=[System.IO.File]::ReadAllLines($src); " ^
  "$idx=-1; for($i=0; $i -lt $lines.Length; $i++) { if ($lines[$i] -match '^rem __BAP_POWERSHELL__') { $idx=$i; break } }; " ^
  "if($idx -lt 0){ Write-Error 'Embedded BAP PowerShell payload marker not found.'; exit 2 }; " ^
  "$payload=$lines[($idx+1)..($lines.Length-1)]; " ^
  "[System.IO.File]::WriteAllLines($dst,$payload,[System.Text.UTF8Encoding]::new($false));"

if errorlevel 1 (
    echo [-] Failed to extract embedded BAP session guard.
    if exist "%BAP_EXTRACT%" del /f /q "%BAP_EXTRACT%" >nul 2>&1
    endlocal
    exit /b 2
)

powershell.exe -NoProfile -ExecutionPolicy Bypass -File "%BAP_EXTRACT%" %*
set "BAP_RC=%ERRORLEVEL%"

if exist "%BAP_EXTRACT%" del /f /q "%BAP_EXTRACT%" >nul 2>&1

endlocal & exit /b %BAP_RC%

rem __BAP_POWERSHELL__
"""

with open('run_claude_bap.ps1', 'r', encoding='utf-8') as f:
    ps1_content = f.read()

full_bat = preamble + ps1_content

with open('run_claude_bap.bat', 'w', encoding='utf-8', newline='\r\n') as f:
    f.write(full_bat)

os.makedirs('dist/windows-amd64/claude-client', exist_ok=True)
shutil.copyfile('run_claude_bap.ps1', 'dist/windows-amd64/claude-client/run_claude_bap.ps1')
shutil.copyfile('run_claude_bap.bat', 'dist/windows-amd64/claude-client/run_claude_bap.bat')
print("Successfully synced run_claude_bap.bat and dist files.")

