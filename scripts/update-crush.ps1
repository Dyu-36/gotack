param(
    [Parameter(Mandatory = $false)]
    [string]$Commit = ''
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot

if (-not $Commit) {
    $pinFile = Join-Path $repoRoot '.crush-pin'
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

# Recall schema coupling (docs/contracts/gotack-recall-mcp.md): cmd/recall
# reads the private crush.db sessions/messages schema. A pin bump that renames
# these tables or columns must fail here loudly instead of silently returning
# empty session_search results.
$schemaFiles = @(
    (Join-Path $crushDir 'internal/db/migrations/20250424200609_initial.sql'),
    (Join-Path $crushDir 'internal/db/models.go')
)
$schemaText = ($schemaFiles | ForEach-Object { Get-Content $_ -Raw }) -join "`n"
$requiredSchemaMarkers = @(
    'CREATE TABLE IF NOT EXISTS sessions',
    'CREATE TABLE IF NOT EXISTS messages',
    'title TEXT',
    'role TEXT',
    'parts TEXT',
    'session_id TEXT',
    'updated_at INTEGER'
)

foreach ($marker in $requiredSchemaMarkers) {
    if (-not $schemaText.Contains($marker)) {
        throw "Crush recall schema marker missing at ${Commit}: $marker"
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
Write-Host "Crush recall schema markers verified at $Commit"
Write-Host "Bundled binary: $bundleExe"
