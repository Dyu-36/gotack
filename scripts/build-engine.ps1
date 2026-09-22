param(
    [string]$EngineSource = (Join-Path $PSScriptRoot '../third_party/engine-source'),
    [string]$Output = (Join-Path $PSScriptRoot '../build/bin/resources/tack-engine.exe')
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$engineRoot = (Resolve-Path -LiteralPath $EngineSource).Path
$outputPath = [IO.Path]::GetFullPath($Output)
$pin = (Get-Content -LiteralPath (Join-Path $repoRoot '.tack-pin') -Raw).Trim()
if ($pin -notmatch '^[0-9a-f]{40}$') { throw 'Invalid engine pin' }
$revision = (& git -C $engineRoot rev-parse HEAD).Trim()
if ($LASTEXITCODE -ne 0 -or $revision -ne $pin) {
    throw "Engine checkout must match .tack-pin: $pin"
}
$dirty = @(& git -C $engineRoot status --porcelain --untracked-files=normal)
if ($LASTEXITCODE -ne 0 -or $dirty.Count -ne 0) {
    throw 'Refusing to package an engine with modified or untracked sources'
}

Push-Location $engineRoot
try {
    # The engine keeps its cross-package test harness behind the gotacktest
    # build tag so it never ships; type-checking tests therefore needs the tag.
    & go test -tags gotacktest -mod=readonly -count=1 ./...
    if ($LASTEXITCODE -ne 0) { throw 'Pinned engine tests failed; packaging stopped' }
    # JSONSchemaAlias intentionally uses a value receiver so invopop/jsonschema
    # can discover it on non-pointer generic Map types. Disable only vet's
    # copylocks analyzer; all other vet analyzers still gate packaging.
    & go vet -tags gotacktest -copylocks=false -mod=readonly ./...
    if ($LASTEXITCODE -ne 0) { throw 'Pinned engine analysis failed; packaging stopped' }
} finally {
    Pop-Location
}
$dirty = @(& git -C $engineRoot status --porcelain --untracked-files=normal)
if ($LASTEXITCODE -ne 0 -or $dirty.Count -ne 0) {
    throw 'Engine sources changed while validating; packaging stopped'
}
$sourceFiles = @(& git -C $engineRoot -c core.quotepath=false ls-files --cached)
if ($LASTEXITCODE -ne 0) { throw 'Cannot enumerate engine source' }
$sourceEntries = @(foreach ($relativePath in ($sourceFiles | Sort-Object -Unique -CaseSensitive)) {
    $sourcePath = Join-Path $engineRoot $relativePath
    if (-not (Test-Path -LiteralPath $sourcePath -PathType Leaf)) {
        throw "Missing tracked source: $relativePath"
    }
    $digest = (Get-FileHash -LiteralPath $sourcePath -Algorithm SHA256).Hash.ToLowerInvariant()
    "$relativePath $digest"
})
$sourceBytes = [Text.Encoding]::UTF8.GetBytes(($sourceEntries -join "`n"))
$sourceDigest = [Convert]::ToHexString([Security.Cryptography.SHA256]::HashData($sourceBytes)).ToLowerInvariant()
$commitEpoch = (& git -C $engineRoot show -s --format=%ct HEAD).Trim()
if ($LASTEXITCODE -ne 0) { throw 'Cannot read source timestamp' }
$builtAt = [DateTimeOffset]::FromUnixTimeSeconds([long]$commitEpoch).UtcDateTime.ToString('yyyy-MM-ddTHH:mm:ssZ')
$package = 'github.com/charmbracelet/crush/internal/version'
$linkerFlags = "-X $package.Commit=$revision -X $package.BuildID=$sourceDigest -X $package.SourceDigest=$sourceDigest -X $package.BuiltAt=$builtAt"
New-Item -ItemType Directory -Path (Split-Path -Parent $outputPath) -Force | Out-Null
$stagedOutput = "$outputPath.stage-$PID"
try {
    Push-Location $engineRoot
    try {
        & go build -mod=readonly -trimpath -ldflags $linkerFlags -o $stagedOutput .
        if ($LASTEXITCODE -ne 0) { throw 'Engine compilation failed' }
    } finally {
        Pop-Location
    }
    $manifest = [ordered]@{
        schema = 1
        engine_commit = $revision
        source_digest = $sourceDigest
        source_files = $sourceEntries.Count
        source_timestamp = $builtAt
        executable_sha256 = (Get-FileHash -LiteralPath $stagedOutput -Algorithm SHA256).Hash.ToLowerInvariant()
        tests_run = $true
        checks = @('go test -tags gotacktest -mod=readonly -count=1 ./...', 'go vet -tags gotacktest -copylocks=false -mod=readonly ./...')
    }
    $manifest | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath "$stagedOutput.build.json" -Encoding utf8NoBOM
    Move-Item -LiteralPath $stagedOutput -Destination $outputPath -Force
    Move-Item -LiteralPath "$stagedOutput.build.json" -Destination "$outputPath.build.json" -Force
} finally {
    Remove-Item -LiteralPath $stagedOutput, "$stagedOutput.build.json" -ErrorAction SilentlyContinue
}
Write-Output "Built and tested engine $revision (source $sourceDigest)"
