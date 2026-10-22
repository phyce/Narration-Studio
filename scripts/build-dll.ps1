<#
.SYNOPSIS
    Builds nstudio.dll (c-shared DLL) for embedding Narration Studio in other applications.

.DESCRIPTION
    Compiles the Go codebase with the "clib" build tag using -buildmode=c-shared,
    producing nstudio.dll and nstudio.h. Copies runtime dependencies (libpiper, onnxruntime,
    espeak-ng-data) into the output directory.

    Prerequisites:
    - Go 1.24+ with CGO enabled
    - GCC toolchain (MSYS2 UCRT64 recommended)
    - libpiper.dll and onnxruntime DLLs built (run build-libpiper.ps1 first)

.PARAMETER OutputDir
    Directory for build output. Default: build/dll

.PARAMETER Clean
    Remove existing output directory before building.

.PARAMETER SkipDeps
    Skip copying runtime dependencies (DLLs, espeak-ng-data).

.EXAMPLE
    .\build-dll.ps1
    .\build-dll.ps1 -Clean
    .\build-dll.ps1 -OutputDir "C:\my\output"
#>

param(
    [string]$OutputDir = "build\bin\dll",
    [switch]$Clean,
    [switch]$SkipDeps
)

$ErrorActionPreference = "Stop"

$ProjectRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path
if (-not [System.IO.Path]::IsPathRooted($OutputDir)) {
    $OutputDir = Join-Path $ProjectRoot $OutputDir
}

Write-Host "=== Narration Studio DLL Build ===" -ForegroundColor Cyan
Write-Host "Project:  $ProjectRoot"
Write-Host "Output:   $OutputDir"
Write-Host ""

$GoPath = (Get-Command go -ErrorAction SilentlyContinue).Source
$GccPath = (Get-Command gcc -ErrorAction SilentlyContinue).Source

if (-not $GoPath) {
    Write-Error "Go not found on PATH."
    exit 1
}
if (-not $GccPath) {
    Write-Error "GCC not found on PATH. CGO requires a C compiler (install MSYS2 UCRT64)."
    exit 1
}

Write-Host "  go:  $GoPath"
Write-Host "  gcc: $GccPath"

$cgoEnabled = & go env CGO_ENABLED 2>&1
if ($cgoEnabled -ne "1") {
    Write-Host "  CGO_ENABLED is '$cgoEnabled', setting to 1" -ForegroundColor Yellow
    $env:CGO_ENABLED = "1"
}
Write-Host ""

if ($Clean -and (Test-Path $OutputDir)) {
    Write-Host "Cleaning $OutputDir..." -ForegroundColor Yellow
    Remove-Item $OutputDir -Recurse -Force
    Write-Host "Clean complete." -ForegroundColor Green
    Write-Host ""
}

if (-not (Test-Path $OutputDir)) {
    New-Item -ItemType Directory -Path $OutputDir | Out-Null
}

Write-Host "Building nstudio.dll..." -ForegroundColor Cyan

$dllPath = Join-Path $OutputDir "nstudio.dll"

Push-Location $ProjectRoot
try {
    & go build -tags clib -buildmode=c-shared -ldflags="-s -w" -o $dllPath .
    if ($LASTEXITCODE -ne 0) {
        Write-Error "Go build failed with exit code $LASTEXITCODE"
        exit 1
    }
} finally {
    Pop-Location
}

Write-Host "  Built: $dllPath" -ForegroundColor Green

$headerPath = Join-Path $OutputDir "nstudio.h"
if (Test-Path $headerPath) {
    Write-Host "  Header: $headerPath" -ForegroundColor Green
}
Write-Host ""

if (-not $SkipDeps) {
    Write-Host "Copying runtime dependencies..." -ForegroundColor Cyan

    $searchPaths = @(
        $ProjectRoot,
        (Join-Path $ProjectRoot "build\bin"),
        (Join-Path $ProjectRoot "build\windows\installer\engines\piper"),
        (Join-Path $ProjectRoot "lib\libpiper-src\libpiper\install"),
        (Join-Path $ProjectRoot "lib\libpiper-src\libpiper\install\lib")
    )

    $runtimeDlls = @(
        "libpiper.dll",
        "onnxruntime.dll",
        "onnxruntime_providers_shared.dll"
    )

    $piperFiles = @(
        "espeak-ng.dll",
        "piper_phonemize.dll",
        "libtashkeel_model.ort"
    )

    $optionalDlls = @(
        "libpiper_directml.dll",
        "DirectML.dll"
    )

    foreach ($dll in $runtimeDlls) {
        $found = $false
        foreach ($searchPath in $searchPaths) {
            $candidate = Join-Path $searchPath $dll
            if (Test-Path $candidate) {
                Copy-Item $candidate -Destination $OutputDir -Force
                Write-Host "  Copied $dll" -ForegroundColor Green
                $found = $true
                break
            }
        }
        if (-not $found) {
            Write-Warning "Required DLL not found: $dll (searched: $($searchPaths -join ', '))"
        }
    }

    foreach ($dll in $optionalDlls) {
        foreach ($searchPath in $searchPaths) {
            $candidate = Join-Path $searchPath $dll
            if (Test-Path $candidate) {
                Copy-Item $candidate -Destination $OutputDir -Force
                Write-Host "  Copied $dll (optional)" -ForegroundColor Green
                break
            }
        }
    }

    foreach ($file in $piperFiles) {
        $found = $false
        foreach ($searchPath in $searchPaths) {
            $candidate = Join-Path $searchPath $file
            if (Test-Path $candidate) {
                Copy-Item $candidate -Destination $OutputDir -Force
                Write-Host "  Copied $file" -ForegroundColor Green
                $found = $true
                break
            }
        }
        if (-not $found) {
            Write-Warning "Piper file not found: $file"
        }
    }

    $espeakSearchPaths = @(
        (Join-Path $ProjectRoot "build\bin\espeak-ng-data"),
        (Join-Path $ProjectRoot "build\windows\installer\engines\piper\espeak-ng-data"),
        (Join-Path $ProjectRoot "lib\libpiper-src\libpiper\install\espeak-ng-data")
    )

    $espeakFound = $false
    foreach ($espeakDir in $espeakSearchPaths) {
        if (Test-Path $espeakDir) {
            $destEspeak = Join-Path $OutputDir "espeak-ng-data"
            if (Test-Path $destEspeak) {
                Remove-Item $destEspeak -Recurse -Force
            }
            Copy-Item $espeakDir -Destination $OutputDir -Recurse -Force
            Write-Host "  Copied espeak-ng-data/" -ForegroundColor Green
            $espeakFound = $true
            break
        }
    }

    if (-not $espeakFound) {
        Write-Warning "espeak-ng-data not found. Piper models that use espeak will not work."
    }

    Write-Host ""
}

Write-Host "=== Build complete ===" -ForegroundColor Green
Write-Host ""
Write-Host "Contents of $OutputDir :"
Get-ChildItem $OutputDir | ForEach-Object {
    if ($_.PSIsContainer) {
        Write-Host "  $($_.Name)/" -ForegroundColor Blue
    } else {
        $sizeKB = [math]::Round($_.Length / 1KB, 1)
        $sizeMB = [math]::Round($_.Length / 1MB, 1)
        if ($sizeMB -ge 1) {
            Write-Host "  $($_.Name)  ($sizeMB MB)"
        } else {
            Write-Host "  $($_.Name)  ($sizeKB KB)"
        }
    }
}
Write-Host ""
Write-Host "Usage:" -ForegroundColor Yellow
Write-Host "  1. Copy the contents of $OutputDir to your application"
Write-Host "  2. Load nstudio.dll and call NStudioInit() to initialize"
Write-Host "  3. See nstudio.h for the full C API"
