<#
.SYNOPSIS
    Builds the Narration Studio desktop app (GUI + CLI) and NSIS installer (Wails v3).
#>

$ErrorActionPreference = "Continue"

$ProjectRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path

Push-Location $ProjectRoot
try {
    Write-Host "=== Narration Studio Build (Wails v3) ===" -ForegroundColor Cyan

    $env:Path += ";$(go env GOPATH)\bin"

    if (-not (Test-Path "build/bin")) {
        New-Item -ItemType Directory -Path "build/bin" | Out-Null
    }

    Write-Host "Building frontend..." -ForegroundColor Cyan
    Push-Location frontend
    npm install
    if ($LASTEXITCODE -ne 0) { Pop-Location; Write-Error "npm install failed"; exit 1 }
    npm run build
    if ($LASTEXITCODE -ne 0) { Pop-Location; Write-Error "frontend build failed"; exit 1 }
    Pop-Location

    Write-Host "Generating frontend bindings..." -ForegroundColor Cyan
    wails3 generate bindings -ts -silent
    if ($LASTEXITCODE -ne 0) { Write-Error "bindings generation failed"; exit 1 }

    Write-Host "Building CLI application..." -ForegroundColor Cyan
    go build -tags cli -o build/bin/nstudio-cli.exe .
    if ($LASTEXITCODE -ne 0) { Write-Error "CLI build failed"; exit 1 }
    Write-Host "  Built: build/bin/nstudio-cli.exe" -ForegroundColor Green
    Copy-Item -Path "build/bin/nstudio-cli.exe" -Destination "build/windows/installer/nstudio-cli.exe" -Force

    Write-Host "Generating Windows resources (icon + version info)..." -ForegroundColor Cyan
    $appInfo = Get-Content -Raw "wails.json" | ConvertFrom-Json
    $verParts = @($appInfo.Info.productVersion.Split('.')) + @('0', '0', '0')
    $noBomUtf8 = New-Object System.Text.UTF8Encoding($false)
    New-Item -ItemType Directory -Force -Path "build/.gen" | Out-Null
    $manifestText = (Get-Content -Raw "build/windows/wails.exe.manifest").Replace('{{.Info.ProductVersion}}', $appInfo.Info.productVersion)
    [System.IO.File]::WriteAllText((Join-Path (Get-Location) "build/.gen/app.manifest"), $manifestText, $noBomUtf8)
    $versionInfo = @"
{
  "FixedFileInfo": {
    "FileVersion":    {"Major":$($verParts[0]),"Minor":$($verParts[1]),"Patch":$($verParts[2]),"Build":0},
    "ProductVersion": {"Major":$($verParts[0]),"Minor":$($verParts[1]),"Patch":$($verParts[2]),"Build":0}
  },
  "StringFileInfo": {
    "CompanyName":"$($appInfo.Info.companyName)",
    "ProductName":"$($appInfo.Info.productName)",
    "ProductVersion":"$($appInfo.Info.productVersion)",
    "FileDescription":"$($appInfo.Info.productName)",
    "LegalCopyright":"$($appInfo.Info.copyright)",
    "Comments":"$($appInfo.Info.comments)",
    "FileVersion":"$($appInfo.Info.productVersion)",
    "InternalName":"narration_studio.exe",
    "OriginalFilename":"narration_studio.exe"
  },
  "VarFileInfo": {"Translation":{"LangID":"0409","CharsetID":"04B0"}},
  "IconPath":"build/windows/icon.ico",
  "ManifestPath":"build/.gen/app.manifest"
}
"@
    [System.IO.File]::WriteAllText((Join-Path (Get-Location) "build/.gen/versioninfo.json"), $versionInfo, $noBomUtf8)
    go run github.com/josephspurrier/goversioninfo/cmd/goversioninfo@v1.4.1 -64 -o wails_windows_amd64.syso "build/.gen/versioninfo.json"
    $sysoExit = $LASTEXITCODE
    Remove-Item -Recurse -Force "build/.gen" -ErrorAction SilentlyContinue
    if ($sysoExit -ne 0) { Write-Error "resource (syso) generation failed"; exit 1 }

    Write-Host "Building GUI application..." -ForegroundColor Cyan
    go build -tags production -ldflags "-w -s -H windowsgui" -o build/bin/narration_studio.exe .
    $guiExit = $LASTEXITCODE
    Remove-Item -Force wails_windows_amd64.syso -ErrorAction SilentlyContinue
    if ($guiExit -ne 0) { Write-Error "GUI build failed"; exit 1 }
    Write-Host "  Built: build/bin/narration_studio.exe" -ForegroundColor Green

    Write-Host "Building Installer..." -ForegroundColor Cyan
    Push-Location build/windows/installer
    makensis "-DARG_WAILS_AMD64_BINARY=..\..\bin\narration_studio.exe" "-DINFO_PRODUCTVERSION=$($appInfo.Info.productVersion)" project.nsi
    $nsisExit = $LASTEXITCODE
    Pop-Location
    if ($nsisExit -ne 0) { Write-Error "Installer build failed"; exit 1 }

    Write-Host ""
    Write-Host "=== Build complete ===" -ForegroundColor Green
    Write-Host "  GUI App:   build/bin/narration_studio.exe"
    Write-Host "  CLI App:   build/bin/nstudio-cli.exe"
    Write-Host "  Installer: build/bin/Narration Studio-amd64-installer.exe"
} finally {
    Pop-Location
}
