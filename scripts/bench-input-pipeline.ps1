#requires -Version 7.0
<#
.SYNOPSIS
    Paired input-pipeline benchmark driver (synthetic harness mode only).
.DESCRIPTION
    Runs control (A) and treatment (B) arms through a provenance-verified
    engine binary and the fake Responses provider via the E2E harness
    benchmark, then aggregates the measured telemetry with
    scripts/input-pipeline/benchmark.mjs. This driver never fabricates a
    latency observation and never sends live provider traffic: live
    benchmarking requires separately owner-authorized budget, credentials,
    and a preregistered gate, and is intentionally not implemented here.
    The synthetic report is therefore always labeled synthetic=true with
    decision=no-rollout, and prompt_cache_key stays default OFF.
.PARAMETER Pairs
    Independent pair count for the schedule (the preregistered live gate
    requires at least 30; synthetic correctness runs may use fewer).
.PARAMETER Seed
    Deterministic seed for the AB/BA schedule and bootstrap CI.
.PARAMETER EngineBinary
    Absolute engine binary path produced by
    scripts/test-input-pipeline-e2e.ps1 -BuildOnly.
.PARAMETER Provenance
    Matching provenance.json path from the same build.
#>
[CmdletBinding()]
param(
    [ValidateRange(1, 10000)][int]$Pairs = 3,
    [ValidateRange(0, 4294967295)][int]$Seed = 42,
    [string]$EngineBinary = '',
    [string]$Provenance = ''
)
$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot

$nodeCommands = @(Get-Command node -CommandType Application -ErrorAction Stop)
if ($nodeCommands.Count -lt 1) { throw 'Node executable was not found.' }
$node = $nodeCommands[0].Source
if (-not [IO.Path]::IsPathFullyQualified($node) -or -not (Test-Path -LiteralPath $node -PathType Leaf)) {
    throw 'Node executable must resolve to one absolute file.'
}

$runMjs = Join-Path $repoRoot 'scripts/input-pipeline/run.mjs'
$benchmarkMjs = Join-Path $repoRoot 'scripts/input-pipeline/benchmark.mjs'
if (-not (Test-Path -LiteralPath $benchmarkMjs -PathType Leaf)) { throw 'Benchmark statistics core missing.' }

if (-not ([IO.Path]::IsPathFullyQualified($EngineBinary) -and (Test-Path -LiteralPath $EngineBinary -PathType Leaf))) {
    throw 'EngineBinary must be an existing absolute file; build one with scripts/test-input-pipeline-e2e.ps1 -BuildOnly.'
}
if (-not ([IO.Path]::IsPathFullyQualified($Provenance) -and (Test-Path -LiteralPath $Provenance -PathType Leaf))) {
    throw 'Provenance must be an existing absolute file from the same build.'
}

# Fail-closed artifact verification, identical to the E2E gate lane.
& $node $runMjs --skip-build --verify-only --binary $EngineBinary --provenance $Provenance
if ($LASTEXITCODE -ne 0) { throw 'Provenance verification failed.' }

# The paired schedule comes from the reporting statistics core so the runner
# and the report can never disagree about pairing or seed handling.
$schedule = & $node $benchmarkMjs schedule $Pairs $Seed
if ($LASTEXITCODE -ne 0 -or -not $schedule) { throw 'Schedule generation failed.' }
$scheduleText = ($schedule -join '').Trim()

$outDir = Join-Path $repoRoot 'tmp/bench-input-pipeline'
New-Item -ItemType Directory -Path $outDir -Force | Out-Null
$stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
$records = Join-Path $outDir "bench-records-$stamp.jsonl"
$report = Join-Path $outDir "bench-report-$stamp.json"
# Exclusive create: a rerun must never append into another run's record file.
$stream = [IO.File]::Open($records, [IO.FileMode]::CreateNew, [IO.FileAccess]::Write, [IO.FileShare]::Read)
$stream.Dispose()

foreach ($pair in @(
        @('TACK_ENGINE_BINARY', (Resolve-Path -LiteralPath $EngineBinary).Path),
        @('TACK_ENGINE_PROVENANCE', (Resolve-Path -LiteralPath $Provenance).Path),
        @('TACK_E2E_REPO_ROOT', $repoRoot),
        @('TACK_E2E_NODE', $node),
        @('TACK_BENCH_RECORD', $records),
        @('TACK_BENCH_SCHEDULE', $scheduleText)
    )) {
    Set-Item -Path "Env:$($pair[0])" -Value $pair[1]
}

Write-Host "=== Input Pipeline Paired Benchmark (synthetic) ==="
Write-Host "Pairs:   $Pairs"
Write-Host "Seed:    $Seed"
Write-Host "Records: $records"
Write-Host ""

Push-Location $repoRoot
try {
    & go test -tags=e2e -run '^$' -bench 'BenchmarkPairedTurns' -benchtime=1x -count=1 -timeout 15m ./e2e/inputpipeline
    if ($LASTEXITCODE -ne 0) { throw 'Paired benchmark run failed.' }
}
finally {
    Pop-Location
    Remove-Item 'Env:TACK_ENGINE_BINARY', 'Env:TACK_ENGINE_PROVENANCE', 'Env:TACK_E2E_REPO_ROOT',
        'Env:TACK_E2E_NODE', 'Env:TACK_BENCH_RECORD', 'Env:TACK_BENCH_SCHEDULE' -ErrorAction SilentlyContinue
}

& $node $benchmarkMjs $records $report fresh $Pairs $Seed
if ($LASTEXITCODE -ne 0) { throw 'Benchmark report aggregation failed.' }

Write-Host ""
Write-Host "Report: $report"
Write-Host "Decision: no-rollout (synthetic correctness evidence only; prompt_cache_key remains OFF)."
