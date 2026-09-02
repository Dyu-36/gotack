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
$serverFiles = @(
    (Join-Path $CrushDir 'internal/server/server.go'),
    (Join-Path $CrushDir 'internal/server/proto.go'),
    (Join-Path $CrushDir 'internal/server/events.go')
)
$serverText = ($serverFiles | ForEach-Object { Get-Content $_ -Raw }) -join "`n"
Assert-ContainsMarkers -Label 'server contract' -Text $serverText -Markers @(
    '/v1/workspaces/{id}/sessions',
    '/v1/workspaces/{id}/agent',
    '/v1/workspaces/{id}/agent/refresh-prompt',
    '/v1/workspaces/{id}/events',
    '/config/model',
    '/config/set',
    '/config/provider-key',
    '/permissions/grant',
    '/questions/answer',
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
