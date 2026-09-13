@echo off
setlocal
cd /d "%~dp0"
echo [*] Testing exact complex inline PowerShell command via bapedge.exe...
echo.

bapedge.exe exec "powershell -NoProfile -Command \"$sw = [System.Diagnostics.Stopwatch]::StartNew(); $body = @{ model='claude-3-5-sonnet-20241022:latest'; messages=@(@{role='user'; content='Say the word READY and nothing else'}) } ^| ConvertTo-Json -Depth 5; $res = Invoke-RestMethod -Uri 'http://localhost:11434/v1/chat/completions' -Method Post -Body $body -ContentType 'application/json'; $sw.Stop(); Write-Host ('Elapsed ms: ' + $sw.ElapsedMilliseconds); Write-Host ('Content: ' + $res.choices[0].message.content)\""

echo.
echo [*] Exit Code: %ERRORLEVEL%
endlocal
