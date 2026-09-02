param(
    [Parameter(Mandatory = $false)]
    [string]$StackRoot = ''
)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'

$repoRoot = Split-Path -Parent $PSScriptRoot
$resourcesBin = Join-Path $repoRoot 'resources/bin'
$sitePackagesDir = Join-Path $resourcesBin 'Lib/site-packages'
$pythonExe = Join-Path $resourcesBin 'python.exe'
$python3Exe = Join-Path $resourcesBin 'python3.exe'
$pythonPth = Join-Path $resourcesBin 'python312._pth'
$uvExe = Join-Path $resourcesBin 'uv.exe'
$officeCliExe = Join-Path $resourcesBin 'officecli.exe'

New-Item -ItemType Directory -Force -Path $resourcesBin | Out-Null

if (-not (Test-Path $officeCliExe)) {
    $localOfficeCli = Join-Path $env:LOCALAPPDATA 'OfficeCLI/officecli.exe'
    if (Test-Path $localOfficeCli) {
        Copy-Item $localOfficeCli $officeCliExe -Force
    } else {
        Invoke-WebRequest -Uri 'https://github.com/iOfficeAI/OfficeCLI/releases/latest/download/officecli-win-x64.exe' -OutFile $officeCliExe
    }
}

# Prefer the already-prepared runtime from Stack when the companion checkout
# is available. Clean checkouts remain reproducible through the pinned
# download fallback below.
$stackCandidates = @()
if ($StackRoot) {
    $stackCandidates += $StackRoot
}
if ($env:GOTACK_STACK_ROOT) {
    $stackCandidates += $env:GOTACK_STACK_ROOT
}
$stackCandidates += 'C:/stack'

if (-not (Test-Path $pythonExe)) {
    foreach ($candidate in $stackCandidates) {
        $sourceBin = Join-Path $candidate 'resources/bin'
        $sourcePython = Join-Path $sourceBin 'python.exe'
        $sourceOrtools = Join-Path $sourceBin 'Lib/site-packages/ortools'
        $sourceOpenpyxl = Join-Path $sourceBin 'Lib/site-packages/openpyxl'
        if ((Test-Path $sourcePython) -and (Test-Path $sourceOrtools) -and (Test-Path $sourceOpenpyxl)) {
            Get-ChildItem -LiteralPath $sourceBin -Force |
                Where-Object { $_.Name -notin @('officecli.exe', 'officecli.exe.update', '.gitkeep') } |
                Copy-Item -Destination $resourcesBin -Recurse -Force
            break
        }
    }
}

if (-not (Test-Path $pythonExe)) {
    $tmpDir = Join-Path $repoRoot 'tmp'
    $pythonZip = Join-Path $tmpDir 'python-3.12.8-embed-amd64.zip'
    New-Item -ItemType Directory -Force -Path $tmpDir | Out-Null
    if (-not (Test-Path $pythonZip)) {
        Invoke-WebRequest -Uri 'https://www.python.org/ftp/python/3.12.8/python-3.12.8-embed-amd64.zip' -OutFile $pythonZip
    }
    Expand-Archive -Path $pythonZip -DestinationPath $resourcesBin -Force
}

Copy-Item $pythonExe $python3Exe -Force

@'
python312.zip
.
Lib/site-packages
import site
'@ | Set-Content -Path $pythonPth -Encoding Ascii

if (-not (Test-Path $uvExe)) {
    $localUv = Join-Path $env:USERPROFILE '.local/bin/uv.exe'
    if (Test-Path $localUv) {
        Copy-Item $localUv $uvExe -Force
    } else {
        $tmpDir = Join-Path $repoRoot 'tmp'
        $uvZip = Join-Path $tmpDir 'uv-x86_64-pc-windows-msvc.zip'
        $uvExtract = Join-Path $tmpDir 'uv_extract'
        New-Item -ItemType Directory -Force -Path $tmpDir | Out-Null
        Invoke-WebRequest -Uri 'https://github.com/astral-sh/uv/releases/latest/download/uv-x86_64-pc-windows-msvc.zip' -OutFile $uvZip
        Expand-Archive -Path $uvZip -DestinationPath $uvExtract -Force
        Copy-Item (Join-Path $uvExtract 'uv-x86_64-pc-windows-msvc/uv.exe') $uvExe -Force
    }
}

foreach ($library in @('openpyxl', 'ortools')) {
    if (-not (Test-Path (Join-Path $sitePackagesDir $library))) {
        & $uvExe pip install $library --target $sitePackagesDir --python $pythonExe
        if ($LASTEXITCODE -ne 0) {
            throw "failed to install bundled Python library: $library"
        }
    }
}

& $pythonExe -c "import openpyxl, ortools, sys; print('Timetable Python ready:', sys.version.split()[0], openpyxl.__version__, ortools.__version__)"
if ($LASTEXITCODE -ne 0) {
    throw 'bundled timetable Python verification failed'
}
