param(
    [Parameter(Mandatory = $false)]
    [ValidatePattern('^[0-9a-f]{40}$')]
    [string]$Commit = '6d14dd93a9e526505f7de54ae5999431bc32a793'
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$crushDir = Join-Path $repoRoot 'third_party/crush'
$bundleDir = Join-Path $repoRoot 'build/bin/resources'
$bundleExe = Join-Path $bundleDir 'crush.exe'
$upstream = 'https://github.com/charmbracelet/crush.git'

if (-not (Test-Path (Join-Path $crushDir '.git'))) {
    New-Item -ItemType Directory -Force -Path (Split-Path -Parent $crushDir) | Out-Null
    git clone --filter=blob:none $upstream $crushDir
}

git -C $crushDir fetch --depth 1 origin $Commit
git -C $crushDir checkout --detach $Commit

$actual = (git -C $crushDir rev-parse HEAD).Trim()
if ($actual -ne $Commit) {
    throw "Crush checkout mismatch: expected $Commit, got $actual"
}

# These markers intentionally fail loudly when upstream routes/events move.
# A pin update is not accepted until internal/crushapi is reviewed against the
# new server contract.
$serverFiles = @(
    (Join-Path $crushDir 'internal/server/server.go'),
    (Join-Path $crushDir 'internal/server/proto.go'),
    (Join-Path $crushDir 'internal/server/events.go')
)
$serverText = ($serverFiles | ForEach-Object { Get-Content $_ -Raw }) -join "`n"
$requiredMarkers = @(
    '/v1/workspaces/{id}/sessions',
    '/v1/workspaces/{id}/agent',
    '/v1/workspaces/{id}/events',
    '/config/model',
    '/config/set',
    '/config/provider-key',
    '/permissions/grant',
    '/questions/answer',
    'PayloadTypeFile',
    'PayloadTypeRunComplete'
)

foreach ($marker in $requiredMarkers) {
    if (-not $serverText.Contains($marker)) {
        throw "Crush contract marker missing at ${Commit}: $marker"
    }
}

New-Item -ItemType Directory -Force -Path $bundleDir | Out-Null
Push-Location $crushDir
try {
    go build -trimpath -o $bundleExe .
}
finally {
    Pop-Location
}

Write-Host "Crush contract markers verified at $Commit"
Write-Host "Bundled binary: $bundleExe"
