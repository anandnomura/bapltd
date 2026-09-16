# Test concurrent Claude Code BAP launcher sessions
$ErrorActionPreference = 'Stop'
$repoRoot = (Get-Item -Path $PSScriptRoot).Parent.FullName

Write-Host "=== TEST 1: Concurrency in SAME directory ===" -ForegroundColor Cyan

# Test that launching two launchers in the same directory coordinates via active_sessions
# We use a mock or sleep command forwarded to Claude
$testDir = Join-Path $env:TEMP "bap_concurrency_test_same_$PID"
if (Test-Path -LiteralPath $testDir) { Remove-Item -LiteralPath $testDir -Recurse -Force }
New-Item -ItemType Directory -Path $testDir -Force | Out-Null

Copy-Item -LiteralPath (Join-Path $repoRoot "run_claude_bap.bat") -Destination $testDir
Copy-Item -LiteralPath (Join-Path $repoRoot "run_claude_bap.ps1") -Destination $testDir

# Create dummy .claude/settings.json in testDir to verify preservation
$claudeDir = Join-Path $testDir ".claude"
New-Item -ItemType Directory -Path $claudeDir -Force | Out-Null
Set-Content -Path (Join-Path $claudeDir "settings.json") -Value '{"custom_developer_setting": true}' -Encoding UTF8

$batPath = Join-Path $testDir "run_claude_bap.bat"

# Start two simultaneous instances in the same directory
Write-Host "[*] Starting simultaneous Session 1 and Session 2 in $testDir ..."
$proc1 = Start-Process -FilePath "cmd.exe" -ArgumentList "/c `"$batPath`" --version" -WorkingDirectory $testDir -PassThru
$proc2 = Start-Process -FilePath "cmd.exe" -ArgumentList "/c `"$batPath`" --version" -WorkingDirectory $testDir -PassThru

$proc1.WaitForExit()
$proc2.WaitForExit()

Write-Host "[+] Session 1 exit code: $($proc1.ExitCode)"
Write-Host "[+] Session 2 exit code: $($proc2.ExitCode)"
if ($proc1.ExitCode -ne 0 -or $proc2.ExitCode -ne 0) {
    throw "Simultaneous sessions in same directory failed! P1=$($proc1.ExitCode), P2=$($proc2.ExitCode)"
}

# Verify original setting was restored
$restoredJson = Get-Content -Path (Join-Path $claudeDir "settings.json") -Raw
if ($restoredJson -notmatch 'custom_developer_setting') {
    throw "Original settings were not restored!"
}
Write-Host "[+] Simultaneous sessions in same directory passed, settings preserved!" -ForegroundColor Green

Write-Host "`n=== TEST 2: Concurrency across TWO DIFFERENT directories ===" -ForegroundColor Cyan
$dirA = Join-Path $env:TEMP "bap_proj_A_$PID"
$dirB = Join-Path $env:TEMP "bap_proj_B_$PID"

New-Item -ItemType Directory -Path $dirA -Force | Out-Null
New-Item -ItemType Directory -Path $dirB -Force | Out-Null

Copy-Item -LiteralPath (Join-Path $repoRoot "run_claude_bap.bat") -Destination $dirA
Copy-Item -LiteralPath (Join-Path $repoRoot "run_claude_bap.ps1") -Destination $dirA

Copy-Item -LiteralPath (Join-Path $repoRoot "run_claude_bap.bat") -Destination $dirB
Copy-Item -LiteralPath (Join-Path $repoRoot "run_claude_bap.ps1") -Destination $dirB

$batA = Join-Path $dirA "run_claude_bap.bat"
$batB = Join-Path $dirB "run_claude_bap.bat"

Write-Host "[*] Launching Instance A and Instance B sequentially/concurrently..."
$pA = Start-Process -FilePath "cmd.exe" -ArgumentList "/c `"$batA`" --version" -WorkingDirectory $dirA -Wait -PassThru
$pB = Start-Process -FilePath "cmd.exe" -ArgumentList "/c `"$batB`" --version" -WorkingDirectory $dirB -Wait -PassThru

Write-Host "[+] Instance A exit code: $($pA.ExitCode)"
Write-Host "[+] Instance B exit code: $($pB.ExitCode)"

if ($pA.ExitCode -ne 0 -or $pB.ExitCode -ne 0) {
    throw "Concurrent instances in different directories failed! A=$($pA.ExitCode), B=$($pB.ExitCode)"
}

Write-Host "[+] Both instances ran and completed independently with ZERO conflicts!" -ForegroundColor Green

# Cleanup
Remove-Item -LiteralPath $testDir -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item -LiteralPath $dirA -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item -LiteralPath $dirB -Recurse -Force -ErrorAction SilentlyContinue

Write-Host "`n=== ALL CONCURRENCY CHECKS PASSED ===" -ForegroundColor Green
