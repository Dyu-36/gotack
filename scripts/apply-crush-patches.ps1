param(
    [Parameter(Mandatory = $true)]
    [string]$CrushDir,
    [string]$PatchDir = '',
    [switch]$SkipInputPipeline
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
if ($PatchDir -eq '') { $PatchDir = Join-Path $repoRoot 'third_party/patches' }
$PatchDir = [IO.Path]::GetFullPath($PatchDir)
$hardeningScript = Join-Path $PSScriptRoot 'harden-crush-for-tack.ps1'

if (-not (Test-Path (Join-Path $CrushDir '.git'))) {
    throw "Crush checkout not found at $CrushDir"
}
$manifestPath = Join-Path $PatchDir 'manifest.json'
if (-not (Test-Path $manifestPath -PathType Leaf)) {
    throw 'Patch manifest missing; replay must not silently omit a phase.'
}
$manifest = Get-Content $manifestPath -Raw -Encoding UTF8 | ConvertFrom-Json
if ($manifest.schema_version -ne 1 -or
    $manifest.compatibility -isnot [array] -or $manifest.compatibility.Count -eq 0 -or
    $manifest.input_pipeline -isnot [array]) {
    throw 'Invalid patch manifest version or phase arrays.'
}
$names = New-Object 'System.Collections.Generic.HashSet[string]' ([StringComparer]::Ordinal)
foreach ($name in @($manifest.compatibility) + @($manifest.input_pipeline)) {
    if ($name -isnot [string] -or $name -cnotmatch '^[a-z0-9][a-z0-9._-]*\.patch$') {
        throw 'Invalid patch filename in manifest.'
    }
    if (-not $names.Add($name)) { throw 'Duplicate patch in manifest.' }
    if (-not (Test-Path (Join-Path $PatchDir $name) -PathType Leaf)) { throw "Missing declared patch: $name" }
}
$diskPatches = @(Get-ChildItem -Path $PatchDir -Filter '*.patch' -File -Recurse)
if ($diskPatches.Count -ne $names.Count) { throw 'Patch inventory differs from manifest.' }
foreach ($patch in $diskPatches) {
    if ($patch.DirectoryName -ne $PatchDir -or -not $names.Contains($patch.Name) -or
        ($patch.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
        throw 'Unlisted, nested, or linked patch; update the explicit manifest before replay.'
    }
}

function Test-GitApply {
    param([string[]]$GitArgs)
    $previousPreference = $ErrorActionPreference
    $ErrorActionPreference = 'SilentlyContinue'
    try {
        & git @GitArgs *> $null
        $succeeded = $LASTEXITCODE -eq 0
    }
    finally { $ErrorActionPreference = $previousPreference }
    return $succeeded
}

function Apply-PatchPhase {
    param([string]$Phase, [AllowEmptyCollection()][string[]]$PatchNames)
    foreach ($name in $PatchNames) {
        $patchPath = Join-Path $PatchDir $name
        $baseArgs = @('-C', $CrushDir, 'apply')
        if (Test-GitApply -GitArgs ($baseArgs + @('--check', $patchPath))) {
            & git @baseArgs --whitespace=nowarn $patchPath
            if ($LASTEXITCODE -ne 0) { throw "Failed to apply Crush patch: $name" }
            Write-Host "Applied ${Phase} patch: $name"
        }
        elseif (Test-GitApply -GitArgs ($baseArgs + @('--reverse', '--check', $patchPath))) {
            Write-Host "Already applied ${Phase} patch: $name"
        }
        else { throw "Crush patch does not apply cleanly: $name" }
    }
}

function Assert-ContainsMarkers {
    param([string]$Label, [string]$Text, [string[]]$Markers)
    foreach ($marker in $Markers) {
        if (-not $Text.Contains($marker)) { throw "Crush $Label marker missing after patching: $marker" }
    }
}
function Assert-NotContainsMarkers {
    param([string]$Label, [string]$Text, [string[]]$Markers)
    foreach ($marker in $Markers) {
        if ($Text.Contains($marker)) { throw "Crush $Label marker unexpectedly present after hardening: $marker" }
    }
}

# The phase boundary is executable, not an alphabetical zz-* convention.
Apply-PatchPhase -Phase 'compatibility' -PatchNames @($manifest.compatibility)
if (-not (Test-Path $hardeningScript -PathType Leaf)) { throw 'Crush hardening script missing.' }
& $hardeningScript -CrushDir $CrushDir
if ($SkipInputPipeline) {
    Write-Host 'Input-pipeline patches explicitly skipped; this is not an input-pipeline release candidate.'
}
else { Apply-PatchPhase -Phase 'input_pipeline' -PatchNames @($manifest.input_pipeline) }

# Every caller retains the existing REST, identity and recall contract checks.
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
    'NewQuestionTool', 'You are Crush', 'agent for Crush', 'analysis agent for Crush',
    'Generated with Crush', 'Assisted-by: Crush', 'Co-Authored-By: Crush', '"crush_info"', '"crush_logs"'
)
Assert-ContainsMarkers -Label 'Tack identity' -Text $agentText -Markers @(
    'You are Tack,', '"tack_info"', '"tack_logs"', 'Generated with Tack'
)
foreach ($file in @('question.go', 'question.md', 'question_test.go')) {
    if (Test-Path (Join-Path $CrushDir "internal/agent/tools/$file")) {
        throw "Question agent-tool source unexpectedly present after hardening: $file"
    }
}
$schemaFiles = @(
    (Join-Path $CrushDir 'internal/db/migrations/20250424200609_initial.sql'),
    (Join-Path $CrushDir 'internal/db/models.go')
)
$schemaText = ($schemaFiles | ForEach-Object { Get-Content $_ -Raw }) -join "`n"
Assert-ContainsMarkers -Label 'recall schema' -Text $schemaText -Markers @(
    'CREATE TABLE IF NOT EXISTS sessions', 'CREATE TABLE IF NOT EXISTS messages',
    'title TEXT', 'role TEXT', 'parts TEXT', 'session_id TEXT', 'updated_at INTEGER'
)
Write-Host 'Crush server, agent-tool identity, and recall contracts verified after ordered replay.'
