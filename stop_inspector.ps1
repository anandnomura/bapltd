Write-Host "===============================================================================" -ForegroundColor Cyan
Write-Host "  Stopping BAP Control Plane Daemon" -ForegroundColor Cyan
Write-Host "==============================================================================="

$procs = Get-Process -Name bapcontrolplane, ltd-service -ErrorAction SilentlyContinue
if ($procs) {
    foreach ($p in $procs) {
        Write-Host "[*] Stopping process $($p.ProcessName) (PID: $($p.Id))..."
        Stop-Process -Id $p.Id -Force
    }
    Write-Host "[+] BAP Control Plane processes terminated." -ForegroundColor Green
} else {
    Write-Host "[*] No bapcontrolplane process found."
}

# Check port 8080
$conns = Get-NetTCPConnection -LocalPort 8080 -State Listen -ErrorAction SilentlyContinue
if ($conns) {
    foreach ($c in $conns) {
        Write-Host "[*] Terminating process holding port 8080 (PID: $($c.OwningProcess))..."
        Stop-Process -Id $c.OwningProcess -Force -ErrorAction SilentlyContinue
    }
}

Start-Sleep -Seconds 1
$remaining = Get-NetTCPConnection -LocalPort 8080 -State Listen -ErrorAction SilentlyContinue
if ($remaining) {
    Write-Host "[!] Warning: Port 8080 is still in use." -ForegroundColor Yellow
} else {
    Write-Host "[+] Port 8080 is now completely released." -ForegroundColor Green
    Write-Host "[+] BAP Control Plane & Inspector stopped successfully." -ForegroundColor Green
}
Write-Host "==============================================================================="

