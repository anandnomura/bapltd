with open('build_all_platforms.ps1', 'r', encoding='utf-8') as f:
    code = f.read()

safe_fn = """function Safe-CopyItem {
    param($Source, $Destination)
    try {
        Copy-Item -LiteralPath $Source -Destination $Destination -Force
    }
    catch [System.IO.IOException] {
        Write-Warning "File in use ($Destination); skipping overwrite."
    }
}
"""

if 'function Safe-CopyItem' not in code:
    code = code.replace('Set-Location $rootDir', 'Set-Location $rootDir\n\n' + safe_fn)

code = code.replace('Copy-Item', 'Safe-CopyItem')

target_block = """        if (Test-Path (Join-Path $rootDir "run_claude_bap.bat")) {
            Safe-CopyItem (Join-Path $rootDir "run_claude_bap.bat") (Join-Path $clientDir "run_claude_bap.bat") -Force
        }"""

replacement_block = """        if (Test-Path (Join-Path $rootDir "run_claude_bap.bat")) {
            Safe-CopyItem (Join-Path $rootDir "run_claude_bap.bat") (Join-Path $clientDir "run_claude_bap.bat") -Force
        }
        if (Test-Path (Join-Path $rootDir "run_claude_bap.ps1")) {
            Safe-CopyItem (Join-Path $rootDir "run_claude_bap.ps1") (Join-Path $clientDir "run_claude_bap.ps1") -Force
        }"""

code = code.replace(target_block, replacement_block)

with open('build_all_platforms.ps1', 'w', encoding='utf-8', newline='\r\n') as f:
    f.write(code)

print('Updated build_all_platforms.ps1 successfully')

