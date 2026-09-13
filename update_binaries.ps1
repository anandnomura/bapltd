param()
$ErrorActionPreference = "Stop"

if (Test-Path "bapedge.old") {
    Remove-Item -Force "bapedge.old" -ErrorAction SilentlyContinue
}

Move-Item -Force "bapedge.exe" "bapedge.old"
Copy-Item -Force "bap-edge\bapedge.exe" "bapedge.exe"

if (Test-Path "bapmcp.old") {
    Remove-Item -Force "bapmcp.old" -ErrorAction SilentlyContinue
}
if (Test-Path "bapmcp.exe") {
    Move-Item -Force "bapmcp.exe" "bapmcp.old" -ErrorAction SilentlyContinue
}
Copy-Item -Force "bap-edge\bapedge.exe" "bapmcp.exe"
Copy-Item -Force "bap-edge\bapedge.exe" "ltd-agent.exe"

if (Test-Path "bapcontrolplane.old") {
    Remove-Item -Force "bapcontrolplane.old" -ErrorAction SilentlyContinue
}
if (Test-Path "bapcontrolplane.exe") {
    Move-Item -Force "bapcontrolplane.exe" "bapcontrolplane.old" -ErrorAction SilentlyContinue
}
Copy-Item -Force "bap-controlplane\bapcontrolplane.exe" "bapcontrolplane.exe"
Copy-Item -Force "bap-controlplane\inspector.html" "inspector.html"

Write-Host "Binaries successfully updated."
