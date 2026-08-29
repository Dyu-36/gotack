param(
    [Parameter(Mandatory = $false)]
    [string]$Output = ''
)

$ErrorActionPreference = 'Stop'
$repoRoot = Split-Path -Parent $PSScriptRoot
$bundleDir = Join-Path $repoRoot 'build/bin/resources'

if (-not $Output) {
    New-Item -ItemType Directory -Force -Path $bundleDir | Out-Null
    $Output = Join-Path $bundleDir 'office.exe'
}

# -H windowsgui turns off the console subsystem so MCP stdio spawns no longer
# flash a conhost window on Windows. stdout/stderr from MCP traffic is still
# piped through os.Stdout/os.Stderr; the GUI flag only detaches the host tty,
# it does not silence MCP stdio. Logs must therefore go through the JSON-RPC
# channel or a file, not fmt.Fprintln(os.Stderr, ...).
$ldflags = '-H windowsgui -s -w'
Write-Host "Building office MCP server (windowsgui) -> $Output"
go build -trimpath -ldflags $ldflags -o $Output ./cmd/office
if ($LASTEXITCODE -ne 0) {
    throw "office build failed with exit code $LASTEXITCODE"
}
Write-Host "Office MCP built: $Output"
