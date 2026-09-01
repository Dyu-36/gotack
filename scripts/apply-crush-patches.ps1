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
