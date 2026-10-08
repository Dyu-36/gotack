param(
    [string]$Output = (Join-Path $PSScriptRoot '../build/bin/gotack.exe')
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$engineRoot = $repoRoot
$outputPath = [IO.Path]::GetFullPath($Output)
$revision = (& git -C $repoRoot rev-parse HEAD).Trim()
if ($LASTEXITCODE -ne 0) { throw 'Cannot identify Gotack revision' }

Push-Location $engineRoot
try {
    # The engine keeps its cross-package test harness behind the gotacktest
    # build tag so it never ships; type-checking tests therefore needs the tag.
    & go test -tags gotacktest -mod=readonly -count=1 ./...
    if ($LASTEXITCODE -ne 0) { throw 'Unified product tests failed; packaging stopped' }
    # JSONSchemaAlias intentionally uses a value receiver so invopop/jsonschema
    # can discover it on non-pointer generic Map types. Disable only vet's
    # copylocks analyzer; all other vet analyzers still gate packaging.
    & go vet -tags gotacktest -copylocks=false -mod=readonly ./...
    if ($LASTEXITCODE -ne 0) { throw 'Unified product analysis failed; packaging stopped' }
} finally {
    Pop-Location
}
$source = & (Join-Path $PSScriptRoot 'source-info.ps1')
$sourceDigest = $source.Digest
$commitEpoch = (& git -C $engineRoot show -s --format=%ct HEAD).Trim()
if ($LASTEXITCODE -ne 0) { throw 'Cannot read source timestamp' }
$builtAt = [DateTimeOffset]::FromUnixTimeSeconds([long]$commitEpoch).UtcDateTime.ToString('yyyy-MM-ddTHH:mm:ssZ')
$package = 'github.com/Dyu-36/gotack/internal/agentcore/version'
$linkerFlags = "-X github.com/Dyu-36/gotack/internal/buildinfo.Commit=$revision -X github.com/Dyu-36/gotack/internal/buildinfo.SourceDigest=$sourceDigest -X $package.Commit=$revision -X $package.BuildID=$sourceDigest -X $package.SourceDigest=$sourceDigest -X $package.BuiltAt=$builtAt"
New-Item -ItemType Directory -Path (Split-Path -Parent $outputPath) -Force | Out-Null
$stagedOutput = "$outputPath.stage-$PID"
try {
    Push-Location $engineRoot
    try {
        & go build -mod=readonly -trimpath -ldflags $linkerFlags -o $stagedOutput ./cmd/gotack
        if ($LASTEXITCODE -ne 0) { throw 'Engine compilation failed' }
    } finally {
        Pop-Location
    }
    $after = & (Join-Path $PSScriptRoot 'source-info.ps1')
    if ($after.Digest -ne $sourceDigest -or $after.Commit -ne $revision) { throw 'Sources changed during compilation' }
    $manifest = [ordered]@{
        schema = 1
        protocol = 1
        linker_flags = $linkerFlags
        engine_commit = $revision
        source_digest = $sourceDigest
        source_files = $source.Files
        source_timestamp = $builtAt
        executable_sha256 = (Get-FileHash -LiteralPath $stagedOutput -Algorithm SHA256).Hash.ToLowerInvariant()
        tests_run = $true
        checks = @('go test -tags gotacktest -mod=readonly -count=1 ./...', 'go vet -tags gotacktest -copylocks=false -mod=readonly ./...')
    }
    $manifestJson = $manifest | ConvertTo-Json -Depth 5
    $utf8NoBOM = New-Object System.Text.UTF8Encoding $false
    [IO.File]::WriteAllText("$stagedOutput.build.json", $manifestJson, $utf8NoBOM)
    Move-Item -LiteralPath $stagedOutput -Destination $outputPath -Force
    Move-Item -LiteralPath "$stagedOutput.build.json" -Destination "$outputPath.build.json" -Force
} finally {
    Remove-Item -LiteralPath $stagedOutput, "$stagedOutput.build.json" -ErrorAction SilentlyContinue
}
Write-Output "Built and tested engine $revision (source $sourceDigest)"
