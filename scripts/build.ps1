<#
.SYNOPSIS
    Builds the Narration Studio desktop app (GUI + CLI) and NSIS installer.
#>

$ErrorActionPreference = "Stop"

$ProjectRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path

Push-Location $ProjectRoot
try {
    Write-Host "=== Narration Studio Build ===" -ForegroundColor Cyan

    if (-not (Test-Path "build/bin")) {
        New-Item -ItemType Directory -Path "build/bin" | Out-Null
    }

    Write-Host "Building CLI application..." -ForegroundColor Cyan
    go build -tags cli -o build/bin/nstudio-cli.exe .
    if ($LASTEXITCODE -ne 0) { Write-Error "CLI build failed"; exit 1 }
    Write-Host "  Built: build/bin/nstudio-cli.exe" -ForegroundColor Green

    Write-Host "Preparing CLI for installer..."
    Copy-Item -Path "build/bin/nstudio-cli.exe" -Destination "build/windows/installer/nstudio-cli.exe" -Force

    Write-Host "Building GUI application..." -ForegroundColor Cyan
    wails build
    if ($LASTEXITCODE -ne 0) { Write-Error "GUI build failed"; exit 1 }
    Write-Host "  Built: build/bin/narration_studio.exe" -ForegroundColor Green

    Write-Host "Building Installer..." -ForegroundColor Cyan
    wails build -nsis
    if ($LASTEXITCODE -ne 0) { Write-Error "Installer build failed"; exit 1 }

    Write-Host ""
    Write-Host "=== Build complete ===" -ForegroundColor Green
    Write-Host "  GUI App:   build/bin/narration_studio.exe"
    Write-Host "  CLI App:   build/bin/nstudio-cli.exe"
    Write-Host "  Installer: build/bin/"
} finally {
    Pop-Location
}
