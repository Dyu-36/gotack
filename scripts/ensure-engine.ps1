param(
    [Parameter(Mandatory = $true)]
    [string]$HostBinary
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$pin = (Get-Content -LiteralPath (Join-Path $repoRoot '.tack-pin') -Raw).Trim()
if ($pin -notmatch '^[0-9a-f]{40}$') { throw 'Invalid engine pin' }

# Wails always writes the host to build/bin, but -o accepts a path and would
# leave a second host tree with its own 100+ MB engine copy inside it. Bundle
# the engine only beside the canonical host; see docs/development.md.
function Get-CanonicalDirectory {
    param([Parameter(Mandatory = $true)][string]$Path)
    return [IO.Path]::GetFullPath($Path).TrimEnd([IO.Path]::DirectorySeparatorChar, [IO.Path]::AltDirectorySeparatorChar)
}
$hostDirectory = Get-CanonicalDirectory (Split-Path -Parent ([IO.Path]::GetFullPath($HostBinary)))
$canonicalBin = Get-CanonicalDirectory (Join-Path $repoRoot 'build/bin')
if (-not [string]::Equals($hostDirectory, $canonicalBin, [StringComparison]::OrdinalIgnoreCase)) {
    throw "Refusing to bundle the pinned engine outside $canonicalBin (received $hostDirectory). Build the desktop host into build/bin, for example with 'wails build -clean'."
}
$output = Join-Path (Split-Path -Parent ([IO.Path]::GetFullPath($HostBinary))) 'resources/tack-engine.exe'
$cache = Join-Path $repoRoot 'resources/bin/tack-engine.exe'

function Test-PinnedEngine {
    param([string]$Executable)
    if (-not (Test-Path -LiteralPath $Executable -PathType Leaf) -or
        -not (Test-Path -LiteralPath "$Executable.build.json" -PathType Leaf)) { return $false }
    try {
        $manifest = Get-Content -LiteralPath "$Executable.build.json" -Raw | ConvertFrom-Json
        return $manifest.engine_commit -eq $pin -and $manifest.tests_run -eq $true -and
            $manifest.executable_sha256 -eq (Get-FileHash -LiteralPath $Executable -Algorithm SHA256).Hash.ToLowerInvariant()
    } catch { return $false }
}

if (Test-PinnedEngine $output) {
    Write-Output "Pinned engine ready: $output"
    exit 0
}
if (-not (Test-PinnedEngine $cache)) {
    $engineSource = $env:GOTACK_ENGINE_SOURCE
    if ([string]::IsNullOrWhiteSpace($engineSource)) {
        $engineSource = Join-Path $repoRoot 'tack-engine-source'
    }
    if (-not (Test-Path -LiteralPath $engineSource -PathType Container)) {
        throw 'Pinned engine source is missing. Follow docs/development.md to obtain it, or set GOTACK_ENGINE_SOURCE to its checkout. The desktop build requires a bundled engine.'
    }
    & (Join-Path $PSScriptRoot 'build-engine.ps1') -EngineSource $engineSource -Output $cache
    if (-not (Test-PinnedEngine $cache)) { throw 'Engine build did not produce a verified pinned binary' }
}
New-Item -ItemType Directory -Path (Split-Path -Parent $output) -Force | Out-Null
Copy-Item -LiteralPath $cache -Destination $output -Force
Copy-Item -LiteralPath "$cache.build.json" -Destination "$output.build.json" -Force
Write-Output "Bundled pinned engine: $output"
