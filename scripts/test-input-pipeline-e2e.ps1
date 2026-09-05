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

# A Windows runner may expose more than one node.exe application on PATH.
# Resolve exactly the first command PowerShell would invoke instead of reading
# .Source from the whole result set, which coerces multiple paths into one
# invalid executable string.
$nodeCommands = @(Get-Command node -CommandType Application -ErrorAction Stop)
if ($nodeCommands.Count -lt 1) { throw 'Node executable was not found.' }
$node = $nodeCommands[0].Source
if (-not [IO.Path]::IsPathFullyQualified($node) -or -not (Test-Path -LiteralPath $node -PathType Leaf)) {
    throw 'Node executable must resolve to one absolute file.'
}

$powerShell = (Get-Process -Id $PID).Path
if (-not [IO.Path]::IsPathFullyQualified($powerShell) -or -not (Test-Path -LiteralPath $powerShell -PathType Leaf)) {
    throw 'PowerShell executable must resolve to one absolute file.'
}

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
