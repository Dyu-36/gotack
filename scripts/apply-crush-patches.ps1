param(
    [Parameter(Mandatory = $true)]
    [string]$CrushDir
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$patchDir = Join-Path $repoRoot 'third_party/patches'

if (-not (Test-Path (Join-Path $CrushDir '.git'))) {
    throw "Crush checkout not found at $CrushDir"
}
if (-not (Test-Path $patchDir)) {
    return
}

function Test-GitApply {
    param(
        [string[]]$GitArgs
    )
    $previousPreference = $ErrorActionPreference
    $ErrorActionPreference = 'SilentlyContinue'
    try {
        & git @GitArgs *> $null
        $succeeded = $LASTEXITCODE -eq 0
    }
    finally {
        $ErrorActionPreference = $previousPreference
    }
    return $succeeded
}

function Assert-ContainsMarkers {
    param(
        [string]$Label,
        [string]$Text,
        [string[]]$Markers
    )
    foreach ($marker in $Markers) {
        if (-not $Text.Contains($marker)) {
            throw "Crush $Label marker missing after patching: $marker"
        }
    }
}

$patches = Get-ChildItem -Path $patchDir -Filter '*.patch' -File | Sort-Object Name
foreach ($patch in $patches) {
    $baseArgs = @('-C', $CrushDir, 'apply')
    if (Test-GitApply -GitArgs ($baseArgs + @('--check', $patch.FullName))) {
        & git @baseArgs --whitespace=nowarn $patch.FullName
        if ($LASTEXITCODE -ne 0) {
            throw "Failed to apply Crush patch $($patch.Name)"
        }
        Write-Host "Applied Crush patch: $($patch.Name)"
        continue
    }

    if (Test-GitApply -GitArgs ($baseArgs + @('--reverse', '--check', $patch.FullName))) {
        Write-Host "Crush patch already applied: $($patch.Name)"
        continue
    }

    throw "Crush patch $($patch.Name) does not apply cleanly to $CrushDir"
}

# Keep every consumer of this script, including CI and release builds, on the
# same reviewed API and recall-schema contract as scripts/update-crush.ps1.
# The closing quote deliberately delimits every route: without it, model would
# also match models, and set would also match set-batch.
$serverFiles = @(
    (Join-Path $CrushDir 'internal/server/server.go'),
    (Join-Path $CrushDir 'internal/server/proto.go'),
    (Join-Path $CrushDir 'internal/server/events.go')
)
$serverText = ($serverFiles | ForEach-Object { Get-Content $_ -Raw }) -join "`n"
Assert-ContainsMarkers -Label 'server contract' -Text $serverText -Markers @(
    'POST /v1/workspaces/{id}/sessions"',
    'GET /v1/workspaces/{id}/agent"',
    'POST /v1/workspaces/{id}/agent/refresh-prompt"',
    'GET /v1/workspaces/{id}/events"',
    'POST /v1/workspaces/{id}/config/model"',
    'POST /v1/workspaces/{id}/config/models"',
    'POST /v1/workspaces/{id}/config/set"',
    'POST /v1/workspaces/{id}/config/set-batch"',
    'POST /v1/workspaces/{id}/config/remove"',
    'POST /v1/workspaces/{id}/config/provider-key"',
    'POST /v1/workspaces/{id}/config/refresh-oauth"',
    'POST /v1/workspaces/{id}/permissions/grant"',
    'POST /v1/workspaces/{id}/questions/answer"',
    'PayloadTypeFile',
    'PayloadTypeRunComplete'
)

$schemaFiles = @(
    (Join-Path $CrushDir 'internal/db/migrations/20250424200609_initial.sql'),
    (Join-Path $CrushDir 'internal/db/models.go')
)
$schemaText = ($schemaFiles | ForEach-Object { Get-Content $_ -Raw }) -join "`n"
Assert-ContainsMarkers -Label 'recall schema' -Text $schemaText -Markers @(
    'CREATE TABLE IF NOT EXISTS sessions',
    'CREATE TABLE IF NOT EXISTS messages',
    'title TEXT',
    'role TEXT',
    'parts TEXT',
    'session_id TEXT',
    'updated_at INTEGER'
)

Write-Host 'Crush server and recall contracts verified after patching.'
