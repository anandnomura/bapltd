<#
.SYNOPSIS
GitHub Copilot Terminal Execution Wrapper for PowerShell.
Routes Copilot commands through ltd-agent with Cedar security policies.
#>
[CmdletBinding()]
param (
    [Parameter(ValueFromRemainingArguments = $true)]
    [string[]]$CommandArgs
)

$cmdString = $CommandArgs -join ' '
if ([string]::IsNullOrWhiteSpace($cmdString)) {
    Write-Error "[copilot-wrap.ps1] No command provided to execute."
    exit 1
}

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$interceptorExe = Join-Path $scriptDir "copilot_interceptor.exe"
$interceptorBin = Join-Path $scriptDir "copilot_interceptor"
$ltdExe = Join-Path $scriptDir "ltd-agent.exe"

if (Test-Path $interceptorExe) {
    & $interceptorExe $cmdString
} elseif (Test-Path $interceptorBin) {
    & $interceptorBin $cmdString
} elseif (Test-Path $ltdExe) {
    & $ltdExe exec --source copilot --raw $cmdString
} else {
    & ltd-agent exec --source copilot --raw $cmdString
}

exit $LASTEXITCODE
