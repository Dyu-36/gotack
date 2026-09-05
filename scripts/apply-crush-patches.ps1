param(
    [Parameter(Mandatory = $true)]
    [string]$CrushDir,
    [string]$PatchDir = "",
    [switch]$SkipInputPipeline
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
if ($PatchDir -eq "") {
    $patchDir = Join-Path $repoRoot 'third_party/patches'
}
$hardeningScript = Join-Path $PSScriptRoot 'harden-crush-for-tack.ps1'

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

function Assert-NotContainsMarkers {
    param(
        [string]$Label,
        [string]$Text,
        [string[]]$Markers
    )
    foreach ($marker in $Markers) {
        if ($Text.Contains($marker)) {
            throw "Crush $Label marker unexpectedly present after hardening: $marker"
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

if (-not (Test-Path $hardeningScript)) {
    throw "Crush hardening script not found: $hardeningScript"
}
& $hardeningScript -CrushDir $CrushDir

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
    'PayloadTypeFile',
    'PayloadTypeRunComplete'
)
Assert-NotContainsMarkers -Label 'server contract' -Text $serverText -Markers @(
    'POST /v1/workspaces/{id}/questions/answer"',
    'POST /v1/workspaces/{id}/questions/cancel"'
)

$agentFiles = @(
    (Join-Path $CrushDir 'internal/agent/coordinator.go'),
    (Join-Path $CrushDir 'internal/config/config.go'),
    (Join-Path $CrushDir 'internal/agent/templates/coder.md.tpl'),
    (Join-Path $CrushDir 'internal/agent/templates/agentic_fetch_prompt.md.tpl'),
    (Join-Path $CrushDir 'internal/agent/templates/task.md.tpl'),
    (Join-Path $CrushDir 'internal/agent/tools/bash.md.tpl'),
    (Join-Path $CrushDir 'internal/agent/tools/crush_info.go'),
    (Join-Path $CrushDir 'internal/agent/tools/crush_logs.go'),
    (Join-Path $CrushDir 'internal/agent/tools/crush_info.md'),
    (Join-Path $CrushDir 'internal/agent/tools/crush_logs.md.tpl')
)
$agentText = ($agentFiles | ForEach-Object { Get-Content $_ -Raw }) -join "`n"
Assert-NotContainsMarkers -Label 'agent tool/identity' -Text $agentText -Markers @(
    'NewQuestionTool',
    'You are Crush',
    'agent for Crush',
    'analysis agent for Crush',
    'Generated with Crush',
    'Assisted-by: Crush',
    'Co-Authored-By: Crush',
    '"crush_info"',
    '"crush_logs"'
)
Assert-ContainsMarkers -Label 'Tack identity' -Text $agentText -Markers @(
    'You are Tack,',
    '"tack_info"',
    '"tack_logs"',
    'Generated with Tack'
)

$questionToolFiles = @(
    (Join-Path $CrushDir 'internal/agent/tools/question.go'),
    (Join-Path $CrushDir 'internal/agent/tools/question.md'),
    (Join-Path $CrushDir 'internal/agent/tools/question_test.go')
)
foreach ($questionToolFile in $questionToolFiles) {
    if (Test-Path $questionToolFile) {
        throw "Question agent-tool source unexpectedly present after hardening: $questionToolFile"
    }
}

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

Write-Host 'Crush server, agent-tool identity, and recall contracts verified after patching.'
