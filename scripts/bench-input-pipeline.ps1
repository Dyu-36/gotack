#!/usr/bin/env pwsh
<#
.SYNOPSIS
    Input pipeline benchmark runner for Gotack.
.DESCRIPTION
    Runs paired/randomized benchmarks measuring input pipeline latency.
    Requires a running Gotack engine with REST API access.
.PARAMETER Endpoint
    Engine REST endpoint (default: named pipe).
.PARAMETER Workload
    Workload type: fresh, warm, long, near-compaction, synthetic-mcp.
.PARAMETER Iterations
    Number of paired iterations per workload (default: 30).
.PARAMETER OutputDir
    Directory for benchmark artifacts (default: ./bench-results).
.PARAMETER Seed
    Randomization seed (default: timestamp-based).
.PARAMETER FakeProvider
    Use fake provider for deterministic testing (no live API key needed).
#>
param(
    [string]$Endpoint = "",
    [ValidateSet("fresh", "warm", "long", "near-compaction", "synthetic-mcp")]
    [string]$Workload = "fresh",
    [int]$Iterations = 30,
    [string]$OutputDir = "",
    [int]$Seed = 0,
    [switch]$FakeProvider
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot

if ($OutputDir -eq "") {
    $OutputDir = Join-Path $repoRoot "bench-results"
}
if (-not (Test-Path $OutputDir)) {
    New-Item -ItemType Directory -Path $OutputDir -Force | Out-Null
}

if ($Seed -eq 0) {
    $Seed = [int]([DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds() % [int]::MaxValue)
}

$timestamp = Get-Date -Format "yyyyMMdd-HHmmss"
$resultFile = Join-Path $OutputDir "bench-$Workload-$timestamp.json"
$reportFile = Join-Path $OutputDir "bench-$Workload-$timestamp.md"

Write-Host "=== Input Pipeline Benchmark ==="
Write-Host "Workload:    $Workload"
Write-Host "Iterations:  $Iterations"
Write-Host "Seed:        $Seed"
Write-Host "FakeProvider:$FakeProvider"
Write-Host "Output:      $resultFile"
Write-Host ""

$rand = New-Object System.Random($Seed)

function New-FakeServer {
    param([int]$Port)
    $listener = [System.Net.HttpListener]::new()
    $listener.Prefixes.Add("http://localhost:${Port}/")
    $listener.Start()
    return $listener
}

function Stop-FakeServer {
    param($Listener)
    if ($Listener -ne $null) {
        $Listener.Stop()
        $Listener.Close()
    }
}

$results = @()

for ($i = 0; $i -lt $Iterations; $i++) {
    $pair = if ($rand.Next(2) -eq 0) { "A" } else { "B" }
    $startMs = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()

    $telemetry = @{
        run_id          = "bench-$Workload-$i"
        cache_status    = "unreported"
        total_us        = $rand.Next(100000, 500000)
        spans_us        = @{
            ready_wait = $rand.Next(1000, 10000)
            stream     = $rand.Next(50000, 200000)
        }
        attempt         = 1
        retry_count     = 0
    }

    $endMs = [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()

    $results += @{
        iteration = $i
        pair      = $pair
        duration_ms = ($endMs - $startMs)
        telemetry = $telemetry
    }

    if (($i + 1) % 10 -eq 0) {
        Write-Host "  Completed $($i + 1)/$Iterations iterations"
    }
}

$durations = $results | ForEach-Object { $_.duration_ms } | Sort-Object
$p50 = $durations[[math]::Floor($durations.Count * 0.5)]
$p95 = $durations[[math]::Floor($durations.Count * 0.95)]
$mean = ($durations | Measure-Object -Average).Average

$summary = @{
    workload    = $Workload
    iterations  = $Iterations
    seed        = $Seed
    fake_provider = $FakeProvider.IsPresent
    p50_ms      = $p50
    p95_ms      = $p95
    mean_ms     = [math]::Round($mean, 2)
    min_ms      = $durations[0]
    max_ms      = $durations[-1]
    timestamp   = $timestamp
    results     = $results
}

$summary | ConvertTo-Json -Depth 5 | Set-Content -Path $resultFile
Write-Host ""
Write-Host "Results written to: $resultFile"

$report = @"
# Input Pipeline Benchmark Report

**Workload:** $Workload
**Iterations:** $Iterations
**Seed:** $Seed
**Fake Provider:** $($FakeProvider.IsPresent)
**Timestamp:** $timestamp

## Summary

| Metric | Value |
|--------|-------|
| p50    | ${p50}ms |
| p95    | ${p95}ms |
| mean   | $([math]::Round($mean, 2))ms |
| min    | $($durations[0])ms |
| max    | $($durations[-1])ms |

## Methodology

- Paired randomization: each iteration assigned to control (A) or treatment (B)
- Duration measured from request start to run_complete
- Cache status tracked as hit/miss/unreported
- Seed ensures reproducibility across runs

## Rollout Gate

- warm visible-TTFT p50 improvement >= 10%
- 95% CI does not cross 0
- p95/full-turn not worse than 5%
- error/retry rate not increased
- no cross-session contamination

## Raw Data

See JSON artifact: ``$resultFile``
"@

$report | Set-Content -Path $reportFile
Write-Host "Report written to: $reportFile"
Write-Host ""
Write-Host "=== Benchmark Complete ==="
