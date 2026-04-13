<#
.SYNOPSIS
    Builds everything: libpiper, desktop app (GUI + CLI + installer), and DLL.

.PARAMETER SkipLibpiper
    Skip building libpiper from source (use existing build/bin/ DLLs).

.PARAMETER SkipApp
    Skip building the desktop app and installer.

.PARAMETER SkipDLL
    Skip building the C-shared DLL.

.EXAMPLE
    .\build-all.ps1
    .\build-all.ps1 -SkipLibpiper          # Skip libpiper rebuild
    .\build-all.ps1 -SkipApp               # Only build libpiper + DLL
    .\build-all.ps1 -SkipDLL               # Only build libpiper + app
#>

param(
    [switch]$SkipLibpiper,
    [switch]$SkipApp,
    [switch]$SkipDLL
)

$ErrorActionPreference = "Stop"
$ScriptsDir = Join-Path $PSScriptRoot "scripts"

Write-Host "=== Narration Studio - Full Build ===" -ForegroundColor Cyan
Write-Host ""

# Step 1: Build libpiper
if (-not $SkipLibpiper) {
    Write-Host "--- Step 1: Building libpiper ---" -ForegroundColor Yellow
    & "$ScriptsDir\build-libpiper.ps1"
    if ($LASTEXITCODE -ne 0) { Write-Error "libpiper build failed"; exit 1 }
    Write-Host ""
} else {
    Write-Host "--- Step 1: Skipping libpiper (using existing DLLs) ---" -ForegroundColor DarkGray
}

# Step 2: Build desktop app + installer
if (-not $SkipApp) {
    Write-Host "--- Step 2: Building desktop app ---" -ForegroundColor Yellow
    & "$ScriptsDir\build.ps1"
    if ($LASTEXITCODE -ne 0) { Write-Error "App build failed"; exit 1 }
    Write-Host ""
} else {
    Write-Host "--- Step 2: Skipping desktop app ---" -ForegroundColor DarkGray
}

# Step 3: Build DLL
if (-not $SkipDLL) {
    Write-Host "--- Step 3: Building DLL ---" -ForegroundColor Yellow
    & "$ScriptsDir\build-dll.ps1"
    if ($LASTEXITCODE -ne 0) { Write-Error "DLL build failed"; exit 1 }
    Write-Host ""
} else {
    Write-Host "--- Step 3: Skipping DLL ---" -ForegroundColor DarkGray
}

Write-Host "=== Full build complete ===" -ForegroundColor Green
Write-Host ""
Write-Host "Outputs:"
Write-Host "  Desktop App: build/bin/narration_studio.exe"
Write-Host "  CLI App:     build/bin/nstudio-cli.exe"
Write-Host "  DLL:         build/bin/dll/"
Write-Host "  Installer:   build/bin/"
