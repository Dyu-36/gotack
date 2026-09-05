#requires -Version 7.0
<#
.SYNOPSIS
    Build a clean pinned engine and run the required Windows black-box gate.
.DESCRIPTION
    Uses a unique OS/RUNNER_TEMP directory, never third_party/crush. No PATH
    fallback for the engine. -SkipBuild requires both explicit absolute artifacts
    from a previous -BuildOnly run of this exact committed candidate.
#>
[CmdletBinding()]
param(
    [switch]$SkipBuild,
    [switch]$BuildOnly,
    [string]$EngineBinary = '',
    [string]$Provenance = ''
)
$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$node = (Get-Command node -CommandType Application -ErrorAction Stop).Source
$powerShell = (Get-Process -Id $PID).Path
$arguments = @((Join-Path $PSScriptRoot 'input-pipeline/run.mjs'), '--powershell', $powerShell)
if ($SkipBuild) { $arguments += '--skip-build' }
if ($BuildOnly) { $arguments += '--build-only' }
if ($EngineBinary) { $arguments += @('--binary', $EngineBinary) }
if ($Provenance) { $arguments += @('--provenance', $Provenance) }
Push-Location $repoRoot
try {
    & $node @arguments
    if ($LASTEXITCODE -ne 0) { throw 'Input-pipeline gate failed; see the diagnostic code above.' }
}
finally { Pop-Location }
