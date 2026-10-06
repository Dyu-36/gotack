[CmdletBinding(SupportsShouldProcess = $true, ConfirmImpact = 'Low')]
param(
    [switch]$BuildOutputs,
    [switch]$Dependencies
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repoRoot = (Resolve-Path -LiteralPath (Join-Path $PSScriptRoot '..')).ProviderPath
$targets = @('artifacts', 'internal/changes')
if ($BuildOutputs) { $targets += @('build/bin', 'frontend/dist', 'frontend/wailsjs') }
if ($Dependencies) { $targets += @('node_modules', 'frontend/node_modules') }

foreach ($relative in $targets) {
    $candidate = [IO.Path]::GetFullPath((Join-Path $repoRoot $relative))
    if (-not $candidate.StartsWith($repoRoot + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) {
        throw "Cleanup target is outside the repository: $candidate"
    }
    if (-not (Test-Path -LiteralPath $candidate)) { continue }
    $target = (Resolve-Path -LiteralPath $candidate).ProviderPath
    if ($target -ne $candidate) { throw "Unexpected resolved cleanup target: $target" }
    $tracked = @(& git -C $repoRoot ls-files -- $relative)
    if ($LASTEXITCODE -ne 0 -or $tracked.Count -gt 0) {
        throw "Refusing to remove tracked files: $relative"
    }
    if ((Get-Item -LiteralPath $target -Force).Attributes -band [IO.FileAttributes]::ReparsePoint) {
        throw "Cleanup root is a link: $target"
    }
    if ($relative -eq 'internal/changes' -and @(Get-ChildItem -LiteralPath $target -Force).Count -gt 0) {
        throw 'The legacy internal/changes directory is no longer empty'
    }
    # Dependencies contain pnpm links. PowerShell removes links themselves;
    # all target roots above are fixed paths inside this repository.
    if ($PSCmdlet.ShouldProcess($target, 'Remove generated local files')) {
        Remove-Item -LiteralPath $target -Recurse -Force
        Write-Output "Removed $relative"
    }
}
