param(
    [string]$EngineSource = (Join-Path $PSScriptRoot '../third_party/engine-source'),
    [string]$Output = (Join-Path $PSScriptRoot '../build/bin/resources/tack-engine.exe')
)

$ErrorActionPreference = 'Stop'
$repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$engineRoot = (Resolve-Path -LiteralPath $EngineSource).Path
$outputPath = [IO.Path]::GetFullPath($Output)
$pin = (Get-Content -LiteralPath (Join-Path $repoRoot '.tack-pin') -Raw).Trim()
$revision = (& git -C $engineRoot rev-parse HEAD).Trim()
if ($LASTEXITCODE -ne 0 -or $revision -ne $pin) {
    throw "Engine checkout must match .tack-pin: $pin"
}

$sourceFiles = @(& git -C $engineRoot -c core.quotepath=false ls-files --cached --others --exclude-standard)
if ($LASTEXITCODE -ne 0) { throw 'Cannot enumerate engine source' }
$sourceEntries = foreach ($relativePath in ($sourceFiles | Sort-Object -Unique -CaseSensitive)) {
    $sourcePath = Join-Path $engineRoot $relativePath
    if (Test-Path -LiteralPath $sourcePath -PathType Leaf) {
        $digest = (Get-FileHash -LiteralPath $sourcePath -Algorithm SHA256).Hash.ToLowerInvariant()
        "$relativePath $digest"
    }
}
$sourceBytes = [Text.Encoding]::UTF8.GetBytes(($sourceEntries -join "`n"))
$hasher = [Security.Cryptography.SHA256]::Create()
try { $sourceDigest = (($hasher.ComputeHash($sourceBytes) | ForEach-Object { $_.ToString('x2') }) -join '') }
finally { $hasher.Dispose() }
$builtAt = [DateTime]::UtcNow.ToString('yyyy-MM-ddTHH:mm:ssZ')
$package = 'github.com/charmbracelet/crush/internal/version'
$linkerFlags = "-X $package.Commit=$revision -X $package.BuildID=$sourceDigest -X $package.SourceDigest=$sourceDigest -X $package.BuiltAt=$builtAt"
New-Item -ItemType Directory -Path (Split-Path -Parent $outputPath) -Force | Out-Null
Push-Location $engineRoot
try {
    & go build -trimpath -ldflags $linkerFlags -o $outputPath .
    if ($LASTEXITCODE -ne 0) { throw 'Engine compilation failed' }
} finally {
    Pop-Location
}
$manifest = [ordered]@{
    engine_commit = $revision
    source_digest = $sourceDigest
    source_files = $sourceEntries.Count
    built_at = $builtAt
    executable_sha256 = (Get-FileHash -LiteralPath $outputPath -Algorithm SHA256).Hash.ToLowerInvariant()
    tests_run = $false
}
$manifest | ConvertTo-Json | Set-Content -LiteralPath "$outputPath.build.json" -Encoding utf8
Write-Output "Built $outputPath (source $sourceDigest)"
