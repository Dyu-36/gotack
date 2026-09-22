param(
    [string]$EngineSource = (Join-Path $PSScriptRoot '../third_party/engine-source'),
    [string]$Python = 'python',
    [string]$OutputDirectory = (Join-Path $PSScriptRoot '../artifacts')
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
if (-not $IsWindows -or -not [Environment]::Is64BitProcess) {
    throw 'The packaged desktop target is Windows x64; use PowerShell 7 x64 on Windows'
}
$repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$engineRoot = (Resolve-Path -LiteralPath $EngineSource).Path
$output = [IO.Path]::GetFullPath($OutputDirectory)

function Invoke-Checked {
    param([string]$Executable, [string[]]$Arguments)
    & $Executable @Arguments
    if ($LASTEXITCODE -ne 0) {
        throw "$Executable failed with exit code $LASTEXITCODE"
    }
}

Push-Location $repoRoot
$staging = $null
$previousEngine = $env:GOTACK_TEST_ENGINE
$previousRequired = $env:GOTACK_REQUIRE_ENGINE
try {
    Invoke-Checked 'pnpm' @('--dir', 'frontend', 'install', '--frozen-lockfile')
    Invoke-Checked 'wails' @('generate', 'module')
    Invoke-Checked 'go' @('test', '-mod=readonly', '-count=1', './...')
    Invoke-Checked 'go' @('vet', '-mod=readonly', './...')
    Invoke-Checked 'go' @('run', './internal/uievents/gen/main.go')
    Invoke-Checked 'git' @('diff', '--exit-code', '--', 'frontend/src/platform/events.generated.ts')
    Invoke-Checked 'pnpm' @('--dir', 'frontend', 'check')
    Invoke-Checked 'pnpm' @('--dir', 'frontend', 'test')
    Invoke-Checked 'pnpm' @('--dir', 'frontend', 'build')
    Invoke-Checked 'wails' @('build', '-platform', 'windows/amd64', '-clean', '-webview2', 'download')

    $engineExecutable = Join-Path $repoRoot 'build/bin/resources/tack-engine.exe'
    & (Join-Path $PSScriptRoot 'build-engine.ps1') -EngineSource $engineRoot -Output $engineExecutable
    Invoke-Checked $Python @((Join-Path $PSScriptRoot 'build-timetable-runtime.py'))

    $env:GOTACK_TEST_ENGINE = $engineExecutable
    $env:GOTACK_REQUIRE_ENGINE = '1'
    Invoke-Checked 'go' @('test', '-mod=readonly', '-count=1', '-run', '^TestBridge', '-v', '.')
    $engineManifest = Get-Content -LiteralPath "$engineExecutable.build.json" -Raw | ConvertFrom-Json
    $runtimeRoot = Join-Path $repoRoot 'build/bin/resources/python'
    $runtimeManifest = Get-Content -LiteralPath (Join-Path $runtimeRoot 'runtime-manifest.json') -Raw | ConvertFrom-Json
    if ($engineManifest.tests_run -ne $true -or $runtimeManifest.checks.cp_sat -ne $true -or $runtimeManifest.checks.excel_templates_roundtrip -ne $true) {
        throw 'Missing successful engine or timetable runtime validation'
    }

    New-Item -ItemType Directory -Path $output -Force | Out-Null
    $staging = Join-Path $repoRoot ('build/bin/product-' + [Guid]::NewGuid().ToString('N'))
    New-Item -ItemType Directory -Path (Join-Path $staging 'resources/python') -Force | Out-Null
    New-Item -ItemType Directory -Path (Join-Path $staging 'licenses') -Force | Out-Null
    Copy-Item -LiteralPath (Join-Path $repoRoot 'build/bin/gotack.exe') -Destination (Join-Path $staging 'gotack.exe')
    Copy-Item -LiteralPath $engineExecutable -Destination (Join-Path $staging 'resources/tack-engine.exe')
    Copy-Item -LiteralPath "$engineExecutable.build.json" -Destination (Join-Path $staging 'resources/tack-engine.exe.build.json')
    Get-ChildItem -LiteralPath $runtimeRoot -Force | Copy-Item -Destination (Join-Path $staging 'resources/python') -Recurse -Force
    Copy-Item -LiteralPath (Join-Path $repoRoot 'README.md') -Destination (Join-Path $staging 'README.md')
    $engineLicense = Join-Path $engineRoot 'LICENSE.md'
    if (-not (Test-Path -LiteralPath $engineLicense -PathType Leaf)) {
        throw 'Pinned engine license is missing'
    }
    Copy-Item -LiteralPath $engineLicense -Destination (Join-Path $staging 'licenses/tack-engine-LICENSE')
    $hostRevision = (& git rev-parse HEAD).Trim()
    if ($LASTEXITCODE -ne 0) { throw 'Cannot identify desktop revision' }
    $manifest = [ordered]@{
        schema = 1
        product = 'Gotack'
        target = 'windows-amd64'
        host_commit = $hostRevision
        engine_commit = $engineManifest.engine_commit
        host_sha256 = (Get-FileHash -LiteralPath (Join-Path $staging 'gotack.exe') -Algorithm SHA256).Hash.ToLowerInvariant()
        engine_sha256 = $engineManifest.executable_sha256
        timetable_runtime_manifest_sha256 = (Get-FileHash -LiteralPath (Join-Path $staging 'resources/python/runtime-manifest.json') -Algorithm SHA256).Hash.ToLowerInvariant()
        checks = @('desktop-tests', 'desktop-vet', 'frontend-check', 'frontend-tests', 'frontend-build', 'engine-tests', 'engine-vet', 'real-ipc-contract', 'packaged-cp-sat', 'excel-template-roundtrip')
    }
    $manifest | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath (Join-Path $staging 'product-manifest.json') -Encoding utf8NoBOM
    $archive = Join-Path $output 'gotack-windows-amd64.zip'
    Compress-Archive -Path (Join-Path $staging '*') -DestinationPath $archive -Force
    $digest = (Get-FileHash -LiteralPath $archive -Algorithm SHA256).Hash.ToLowerInvariant()
    # `sha256sum -c` (Linux, WSL, Git Bash, CI) rejects the trailing CR that
    # Set-Content adds on Windows, so write the published checksum file with
    # explicit LF line endings instead of the platform default.
    [IO.File]::WriteAllText((Join-Path $output 'SHA256SUMS'), "$digest  gotack-windows-amd64.zip`n", [Text.UTF8Encoding]::new($false))
    Copy-Item -LiteralPath (Join-Path $staging 'product-manifest.json') -Destination (Join-Path $output 'product-manifest.json') -Force
    Write-Output "Verified product: $archive"
} finally {
    $env:GOTACK_TEST_ENGINE = $previousEngine
    $env:GOTACK_REQUIRE_ENGINE = $previousRequired
    if ($null -ne $staging -and (Test-Path -LiteralPath $staging)) {
        Remove-Item -LiteralPath $staging -Recurse -Force
    }
    Pop-Location
}
