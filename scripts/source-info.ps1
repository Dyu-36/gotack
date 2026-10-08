$ErrorActionPreference = 'Stop'
$repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$revision = (& git -C $repoRoot rev-parse HEAD).Trim()
if ($LASTEXITCODE -ne 0) { throw 'Cannot identify Gotack revision' }
$files = @(& git -C $repoRoot -c core.quotepath=false ls-files --cached --others --exclude-standard)
if ($LASTEXITCODE -ne 0) { throw 'Cannot enumerate Gotack sources' }
$entries = @(foreach ($relative in ($files | Sort-Object -Unique -CaseSensitive)) {
    $path = Join-Path $repoRoot $relative
    if (-not (Test-Path -LiteralPath $path -PathType Leaf)) { throw "Missing source: $relative" }
    "$relative $((Get-FileHash -LiteralPath $path -Algorithm SHA256).Hash.ToLowerInvariant())"
})
$bytes = [Text.Encoding]::UTF8.GetBytes(($entries -join "`n"))
$digest = [Convert]::ToHexString([Security.Cryptography.SHA256]::HashData($bytes)).ToLowerInvariant()
[pscustomobject]@{ Commit = $revision; Digest = $digest; Files = $entries.Count }
