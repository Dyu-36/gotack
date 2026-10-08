param([Parameter(Mandatory = $true)][string]$HostBinary)
$ErrorActionPreference = 'Stop'
$repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$hostDirectory = [IO.Path]::GetFullPath((Split-Path -Parent $HostBinary))
$canonical = [IO.Path]::GetFullPath((Join-Path $repoRoot 'build/bin'))
if (-not [string]::Equals($hostDirectory, $canonical, [StringComparison]::OrdinalIgnoreCase)) { throw 'Build the desktop into build/bin' }
$source = & (Join-Path $PSScriptRoot 'source-info.ps1')
$cache = Join-Path $repoRoot 'resources/bin/gotack.exe'
$valid = $false
if ((Test-Path -LiteralPath $cache) -and (Test-Path -LiteralPath "$cache.build.json")) {
    $manifest = Get-Content -LiteralPath "$cache.build.json" -Raw | ConvertFrom-Json
    $valid = $manifest.engine_commit -eq $source.Commit -and $manifest.source_digest -eq $source.Digest -and
        $manifest.tests_run -eq $true -and $manifest.protocol -eq 1 -and
        $manifest.executable_sha256 -eq (Get-FileHash -LiteralPath $cache -Algorithm SHA256).Hash.ToLowerInvariant()
}
if (-not $valid) { & (Join-Path $PSScriptRoot 'build-engine.ps1') -Output $cache }
Copy-Item -LiteralPath $cache -Destination (Join-Path $hostDirectory 'gotack.exe') -Force
Copy-Item -LiteralPath "$cache.build.json" -Destination (Join-Path $hostDirectory 'gotack.exe.build.json') -Force
