#!/usr/bin/env pwsh
<#
.SYNOPSIS
    End-to-end test script for the Gotack input pipeline.
.DESCRIPTION
    Builds the engine from pin + patches, runs fake OpenAI server and fake stdio MCP,
    sends prompts through REST, reads SSE events, and validates provider capture.
    Must not import Crush internals.
.PARAMETER CrushDir
    Path to Crush checkout. If not provided, clones from pin.
.PARAMETER SkipBuild
    Skip engine build (use existing binary).
#>
param(
    [string]$CrushDir = "",
    [switch]$SkipBuild
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$patchScript = Join-Path $repoRoot 'scripts/apply-crush-patches.ps1'
$pinFile = Join-Path $repoRoot '.tack-pin'
$e2eDir = Join-Path $repoRoot 'e2e/inputpipeline'

Write-Host "=== Gotack Input Pipeline E2E Tests ==="
Write-Host ""

# Step 1: Determine Crush checkout
if ($CrushDir -eq "") {
    $CrushDir = Join-Path $env:RUNNER_TEMP "crush-e2e"
    if (-not (Test-Path $CrushDir)) {
        $pin = (Get-Content $pinFile -Raw).Trim()
        Write-Host "Cloning Crush at pin $pin..."
        & git clone https://github.com/charmbracelet/crush.git $CrushDir
        & git -C $CrushDir checkout $pin
    }
}

if (-not (Test-Path (Join-Path $CrushDir '.git'))) {
    throw "Crush checkout not found at $CrushDir"
}

# Step 2: Apply patches
Write-Host "Applying Crush patches..."
& $patchScript -CrushDir $CrushDir
if ($LASTEXITCODE -ne 0) {
    throw "Failed to apply Crush patches"
}

# Step 3: Build engine (unless skipped)
$engineBinary = ""
if (-not $SkipBuild) {
    Write-Host "Building engine..."
    $buildOutput = Join-Path $env:RUNNER_TEMP "tack-engine-e2e.exe"
    & go -C $CrushDir build -o $buildOutput ./cmd/crush
    if ($LASTEXITCODE -ne 0) {
        throw "Engine build failed"
    }
    $engineBinary = $buildOutput
    Write-Host "Engine built: $engineBinary"
} else {
    $candidates = @("tack-engine.exe", "tack-engine", "crush")
    foreach ($c in $candidates) {
        $found = Get-Command $c -ErrorAction SilentlyContinue
        if ($found) {
            $engineBinary = $found.Source
            break
        }
    }
    if ($engineBinary -eq "") {
        throw "No engine binary found. Run without -SkipBuild or set TACK_ENGINE_BINARY."
    }
}

# Step 4: Run E2E tests
Write-Host ""
Write-Host "Running E2E tests..."
$env:TACK_ENGINE_BINARY = $engineBinary

& go test -tags=e2e ./e2e/inputpipeline -count=1 -v -timeout=300s
$testResult = $LASTEXITCODE

Write-Host ""
if ($testResult -eq 0) {
    Write-Host "=== E2E Tests PASSED ==="
} else {
    Write-Host "=== E2E Tests FAILED (exit code: $testResult) ==="
}

exit $testResult
