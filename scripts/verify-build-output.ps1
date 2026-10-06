param(
    [string]$BinDirectory = (Join-Path $PSScriptRoot '../build/bin'),
    [string]$HostBinary = 'gotack.exe'
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$binRoot = [IO.Path]::GetFullPath($BinDirectory).TrimEnd([IO.Path]::DirectorySeparatorChar, [IO.Path]::AltDirectorySeparatorChar)
if (-not (Test-Path -LiteralPath $binRoot -PathType Container)) {
    throw "Wails output directory is missing: $binRoot"
}
$hostPath = Join-Path $binRoot $HostBinary
if (-not (Test-Path -LiteralPath $hostPath -PathType Leaf)) {
    throw "Missing desktop host binary: $hostPath"
}

# Wails joins -o onto build/bin, so a path passed to -o leaves a duplicate host
# tree (and a second engine copy) behind. Only the host, the bundled resources
# directory and WebView2 loader DLLs belong in the output directory.
$unexpected = @(foreach ($entry in @(Get-ChildItem -LiteralPath $binRoot -Force)) {
    $isHost = $entry.Name -eq $HostBinary -and -not $entry.PSIsContainer
    $isResources = $entry.PSIsContainer -and $entry.Name -eq 'resources'
    $isLoader = -not $entry.PSIsContainer -and $entry.Extension -ieq '.dll'
    if (-not ($isHost -or $isResources -or $isLoader)) { $entry.Name }
})
if ($unexpected.Count -gt 0) {
    throw ("Unexpected entries in the Wails output directory {0}: {1}. Build with 'wails build -clean' and never pass a directory to -o." -f $binRoot, ($unexpected -join ', '))
}

$engine = Join-Path $binRoot 'resources/tack-engine.exe'
$engineManifest = "$engine.build.json"
foreach ($required in @($engine, $engineManifest)) {
    if (-not (Test-Path -LiteralPath $required -PathType Leaf)) {
        throw "Missing bundled engine payload: $required. The Windows post-build hook must run ensure-engine.ps1."
    }
}
$manifest = Get-Content -LiteralPath $engineManifest -Raw | ConvertFrom-Json
$pin = (Get-Content -LiteralPath (Join-Path $repoRoot '.tack-pin') -Raw).Trim()
if ($manifest.engine_commit -ne $pin) {
    throw "Bundled engine $($manifest.engine_commit) does not match .tack-pin $pin"
}
if ($manifest.tests_run -ne $true) {
    throw 'Bundled engine manifest does not record passing engine tests'
}
$digest = (Get-FileHash -LiteralPath $engine -Algorithm SHA256).Hash.ToLowerInvariant()
if ($manifest.executable_sha256 -ne $digest) {
    throw 'Bundled engine checksum does not match its manifest'
}
Write-Output "Verified build output: $hostPath with engine $($manifest.engine_commit)"
