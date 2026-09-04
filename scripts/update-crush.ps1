param(
    [Parameter(Mandatory = $false)]
    [string]$Commit = ''
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot

if (-not $Commit) {
    $pinFile = Join-Path $repoRoot '.tack-pin'
    if (-not (Test-Path $pinFile)) {
        throw "No -Commit given and $pinFile does not exist."
    }
    $Commit = (Get-Content $pinFile -Raw).Trim()
}
if ($Commit -notmatch '^[0-9a-f]{40}$') {
    throw "Commit '$Commit' is not a 40-character lowercase SHA."
}

$crushDir = Join-Path $repoRoot 'third_party/crush'
$bundleDir = Join-Path $repoRoot 'build/bin/resources'
$bundleExe = Join-Path $bundleDir 'tack-engine.exe'
$legacyExe = Join-Path $bundleDir 'crush.exe'
$upstream = 'https://github.com/charmbracelet/crush.git'

if (-not (Test-Path (Join-Path $crushDir '.git'))) {
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $crushDir) | Out-Null
    git clone --filter=blob:none $upstream $crushDir
}

git -C $crushDir fetch --depth 1 origin $Commit
git -C $crushDir checkout --detach $Commit
git -C $crushDir reset --hard $Commit
git -C $crushDir clean -fd

$actual = (git -C $crushDir rev-parse HEAD).Trim()
if ($actual -ne $Commit) {
    throw "Crush checkout mismatch: expected $Commit, got $actual"
}

# This shared script owns both patch application and all reviewed server/schema
# contract assertions. Keeping the markers in one place prevents CI, release,
# and local update paths from drifting apart.
& (Join-Path $PSScriptRoot 'apply-crush-patches.ps1') -CrushDir $crushDir

New-Item -ItemType Directory -Force -Path $bundleDir | Out-Null
Push-Location $crushDir
try {
    go build -trimpath -o $bundleExe .
    Copy-Item $bundleExe $legacyExe -Force
}
finally {
    Pop-Location
}

Write-Host "Crush patch and contract verification completed at $Commit"
Write-Host "Bundled binary: $bundleExe"
