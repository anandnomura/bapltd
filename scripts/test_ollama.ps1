# Query local Ollama instance with high-precision timing
$sw = [System.Diagnostics.Stopwatch]::StartNew()

$body = @{
    model = 'claude-3-5-sonnet-20241022:latest'
    messages = @(
        @{ role = 'user'; content = 'Say the word READY and nothing else' }
    )
} | ConvertTo-Json -Depth 5

try {
    $res = Invoke-RestMethod -Uri 'http://localhost:11434/v1/chat/completions' -Method Post -Body $body -ContentType 'application/json'
    $sw.Stop()
    Write-Host "============================================================" -ForegroundColor Cyan
    Write-Host "  OLLAMA POWERSHELL TEST SUCCESS" -ForegroundColor Green
    Write-Host "============================================================" -ForegroundColor Cyan
    Write-Host "Model Response : $($res.choices[0].message.content)"
    Write-Host "Elapsed ms     : $($sw.ElapsedMilliseconds) ms ($([math]::Round($sw.ElapsedMilliseconds / 1000, 2)) s)"
    Write-Host "============================================================" -ForegroundColor Cyan
} catch {
    Write-Host "[!] Error contacting Ollama: $_" -ForegroundColor Red
}

