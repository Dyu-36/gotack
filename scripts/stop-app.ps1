param(
    [string[]]$ProcessName = @('gotack-desktop', 'gotack', 'tack-engine'),
    [int]$TimeoutSeconds = 15
)

$ErrorActionPreference = 'Stop'
Set-StrictMode -Version Latest
$repoRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '..'))
$binRoot = [IO.Path]::GetFullPath((Join-Path $repoRoot 'build/bin')).TrimEnd([IO.Path]::DirectorySeparatorChar, [IO.Path]::AltDirectorySeparatorChar)
$binPrefix = $binRoot + [IO.Path]::DirectorySeparatorChar
$deadline = (Get-Date).AddSeconds($TimeoutSeconds)

# Windows refuses to replace a running host binary, so a build cannot clean or
# rewrite build/bin while a previous run still holds it open. Stop every process
# started from this repository's output directory first. Same-named processes
# from another location keep running: they cannot lock this build.
foreach ($name in $ProcessName) {
    foreach ($process in @(Get-Process -Name $name -ErrorAction SilentlyContinue)) {
        $path = $null
        try { $path = $process.Path } catch { $path = $null }
        if ([string]::IsNullOrWhiteSpace($path)) {
            Write-Warning "Cannot read the image path of $name (PID $($process.Id)); leaving it running"
            continue
        }
        if (-not $path.StartsWith($binPrefix, [StringComparison]::OrdinalIgnoreCase)) {
            Write-Output "Leaving $name (PID $($process.Id)) running: it was started from $path"
            continue
        }
        Write-Output "Stopping $name (PID $($process.Id)) from $path"
        try {
            Stop-Process -Id $process.Id -Force
        } catch {
            Write-Warning "Cannot stop $name (PID $($process.Id)): $($_.Exception.Message)"
        }
    }
}

foreach ($target in @((Join-Path $binRoot 'gotack-desktop.exe'), (Join-Path $binRoot 'gotack.exe'))) {
    while (Test-Path -LiteralPath $target -PathType Leaf) {
        try {
            $stream = [IO.File]::Open($target, [IO.FileMode]::Open, [IO.FileAccess]::ReadWrite, [IO.FileShare]::None)
            $stream.Dispose()
            break
        } catch [System.IO.IOException] {
            if ((Get-Date) -ge $deadline) {
                throw "Still locked after $TimeoutSeconds seconds: $target. Quit Gotack from the system tray and retry."
            }
            Start-Sleep -Milliseconds 250
        }
    }
}
Write-Output "No process holds $binRoot open"
