param(
    [Parameter(Mandatory = $false)]
    [switch]$Clean
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$frontendDir = Join-Path $repoRoot 'frontend'
$binDir = Join-Path $repoRoot 'build/bin'
$resourceDir = Join-Path $binDir 'resources'

Push-Location $repoRoot
try {
    pnpm --dir $frontendDir build
    if ($LASTEXITCODE -ne 0) {
        throw "frontend build failed with exit code $LASTEXITCODE"
    }

    $wailsArgs = @('build')
    if ($Clean) {
        $wailsArgs += '-clean'
    }
    wails @wailsArgs
    if ($LASTEXITCODE -ne 0) {
        throw "Wails build failed with exit code $LASTEXITCODE"
    }

    # Wails may clean build/bin, so assemble external runtime resources only
    # after the application executable has been produced.
    & (Join-Path $PSScriptRoot 'update-crush.ps1')
    & (Join-Path $PSScriptRoot 'build-office.ps1')
    & (Join-Path $PSScriptRoot 'prepare-resources.ps1')

    New-Item -ItemType Directory -Force -Path $resourceDir | Out-Null

    $runtimeBin = Join-Path $repoRoot 'resources/bin'
    if (Test-Path $runtimeBin) {
        Copy-Item (Join-Path $runtimeBin '*') $resourceDir -Recurse -Force
    }

    $skills = Join-Path $repoRoot 'resources/skills'
    if (Test-Path $skills) {
        $skillsTarget = Join-Path $resourceDir 'skills'
        $resolvedBin = [IO.Path]::GetFullPath($binDir).TrimEnd([IO.Path]::DirectorySeparatorChar)
        $resolvedSkillsTarget = [IO.Path]::GetFullPath($skillsTarget)
        if (-not $resolvedSkillsTarget.StartsWith($resolvedBin + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) {
            throw "refusing to replace skills outside build/bin: $resolvedSkillsTarget"
        }
        if (Test-Path $skillsTarget) {
            Remove-Item -LiteralPath $skillsTarget -Recurse -Force
        }
        Copy-Item $skills $skillsTarget -Recurse -Force
    }

    $helperLDFlags = '-H windowsgui -s -w'
    $learningHelpers = @('guard', 'memory', 'skills', 'recall')
    foreach ($helper in $learningHelpers) {
        $helperOutput = Join-Path $resourceDir "$helper.exe"
        Write-Host "Building $helper helper (windowsgui) -> $helperOutput"
        go build -trimpath -ldflags $helperLDFlags -o $helperOutput "./cmd/$helper"
        if ($LASTEXITCODE -ne 0) {
            throw "$helper build failed with exit code $LASTEXITCODE"
        }
    }

    $crush = Join-Path $resourceDir 'crush.exe'
    if (-not (Test-Path $crush)) {
        throw "runtime assembly failed: missing $crush"
    }
    foreach ($helper in $learningHelpers) {
        $helperBinary = Join-Path $resourceDir "$helper.exe"
        if (-not (Test-Path $helperBinary)) {
            throw "runtime assembly failed: missing $helperBinary"
        }
    }
    $python = Join-Path $resourceDir 'python.exe'
    if (-not (Test-Path $python)) {
        throw "runtime assembly failed: missing $python"
    }
    & $python -c "import openpyxl, ortools"
    if ($LASTEXITCODE -ne 0) {
        throw "runtime assembly failed: bundled Python cannot import openpyxl and ortools"
    }

    Write-Host "Tack built: $(Join-Path $binDir 'tack.exe')"
    Write-Host "Runtime resources: $resourceDir"
}
finally {
    Pop-Location
}
