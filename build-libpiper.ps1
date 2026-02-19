<#
.SYNOPSIS
    Builds libpiper from source and stages runtime files for piper-native engine.

.DESCRIPTION
    Fully automated build:
    1. Clones piper1-gpl at a pinned commit (if not already present)
    2. Applies MinGW/Windows compatibility patch
    3. Builds CPU variant (libpiper.dll)
    4. Applies DirectML patch and builds GPU variant (libpiper_directml.dll)
    5. Copies runtime files (DLLs, espeak-ng-data) into build/bin/

    Prerequisites:
    - MSYS2 UCRT64 toolchain: gcc, g++, cmake, ninja
      Install: pacman -S mingw-w64-ucrt-x86_64-gcc mingw-w64-ucrt-x86_64-cmake mingw-w64-ucrt-x86_64-ninja
    - Git

    NOTE: Build artifacts are placed in lib/build/ within the project directory.

.PARAMETER Clean
    Remove existing build, install, and source directories before building.

.PARAMETER BuildType
    CMake build type. Default: Release

.PARAMETER SkipCPU
    Skip building the CPU variant.

.PARAMETER SkipDirectML
    Skip building the DirectML variant.

.EXAMPLE
    .\build-libpiper.ps1
    .\build-libpiper.ps1 -Clean
    .\build-libpiper.ps1 -SkipCPU        # Only build DirectML variant
    .\build-libpiper.ps1 -SkipDirectML    # Only build CPU variant
#>

param(
    [switch]$Clean,
    [string]$BuildType = "Release",
    [switch]$SkipCPU,
    [switch]$SkipDirectML
)

$ErrorActionPreference = "Stop"

# --- Configuration ---
$PiperGitRepo   = "https://github.com/OHF-Voice/piper1-gpl.git"
$PiperGitCommit = "32b95f8c1f0dc0ce27a6acd1143de331f61af777"

$ProjectRoot  = $PSScriptRoot
$LibDir       = Join-Path $ProjectRoot "lib"
$SourceDir    = Join-Path $LibDir "libpiper-src"
$MingwPatch   = Join-Path $LibDir "libpiper-mingw.patch"
$DirectMLPatch = Join-Path $LibDir "libpiper-directml.patch"
$RuntimeDir   = Join-Path $ProjectRoot "build\bin"

# Build artifacts go into lib/build-* subdirectories
$SafeRoot = Join-Path $LibDir "build"

# Where CMake source lives (the libpiper subfolder within the repo)
$CmakeSourceDir = Join-Path $SourceDir "libpiper"

Write-Host "=== libpiper build script ===" -ForegroundColor Cyan
Write-Host "Repository: $PiperGitRepo"
Write-Host "Commit:     $PiperGitCommit"
Write-Host "Source:     $SourceDir"
Write-Host "Runtime:    $RuntimeDir"
Write-Host ""

# --- Ensure MSYS2 MinGW-w64 toolchain ---
$Msys2Bin = "C:\msys64\ucrt64\bin"
if (Test-Path $Msys2Bin) {
    $env:PATH = "$Msys2Bin;$env:PATH"
    Write-Host "Using MSYS2 UCRT64 toolchain from $Msys2Bin" -ForegroundColor Green
} else {
    Write-Error @"
MSYS2 UCRT64 not found at $Msys2Bin.

Install MSYS2 from https://www.msys2.org/ then run in MSYS2 UCRT64 terminal:
  pacman -S mingw-w64-ucrt-x86_64-gcc mingw-w64-ucrt-x86_64-cmake mingw-w64-ucrt-x86_64-ninja
"@
    exit 1
}

# Verify toolchain
$GccPath = (Get-Command gcc -ErrorAction SilentlyContinue).Source
$CmakePath = (Get-Command cmake -ErrorAction SilentlyContinue).Source
$NinjaPath = (Get-Command ninja -ErrorAction SilentlyContinue).Source
$GitPath = (Get-Command git -ErrorAction SilentlyContinue).Source

Write-Host "  gcc:   $GccPath"
Write-Host "  cmake: $CmakePath"
Write-Host "  ninja: $NinjaPath"
Write-Host "  git:   $GitPath"

foreach ($tool in @(@("gcc", $GccPath), @("cmake", $CmakePath), @("ninja", $NinjaPath), @("git", $GitPath))) {
    if (-not $tool[1]) {
        Write-Error "$($tool[0]) not found on PATH."
        exit 1
    }
}

if ($GccPath -like "*cygwin*") {
    Write-Error "Cygwin GCC detected. Ensure C:\msys64\ucrt64\bin is before C:\cygwin64\bin in PATH."
    exit 1
}
Write-Host ""

# --- Clean ---
if ($Clean) {
    Write-Host "Cleaning..." -ForegroundColor Yellow
    foreach ($dir in @("$SafeRoot\build-cpu", "$SafeRoot\build-directml", "$SafeRoot\install-cpu", "$SafeRoot\install-directml")) {
        if (Test-Path $dir) {
            Remove-Item $dir -Recurse -Force
            Write-Host "  Removed $dir"
        }
    }
    if (Test-Path $RuntimeDir) {
        foreach ($item in @("libpiper.dll", "libpiper_directml.dll", "onnxruntime.dll", "onnxruntime_providers_shared.dll", "DirectML.dll", "espeak-ng-data")) {
            $target = Join-Path $RuntimeDir $item
            if (Test-Path $target) {
                Remove-Item $target -Recurse -Force
                Write-Host "  Removed $target"
            }
        }
    }
    Write-Host "Clean complete." -ForegroundColor Green
    Write-Host ""
}

if (-not (Test-Path $SafeRoot)) {
    New-Item -ItemType Directory -Path $SafeRoot | Out-Null
}

# --- Clone source and apply patches (only on fresh clone) ---
$freshClone = $false
if (-not (Test-Path (Join-Path $SourceDir ".git"))) {
    Write-Host "Cloning piper1-gpl..." -ForegroundColor Cyan
    if (Test-Path $SourceDir) {
        Remove-Item $SourceDir -Recurse -Force
    }
    git clone --depth 50 $PiperGitRepo $SourceDir
    if ($LASTEXITCODE -ne 0) {
        Write-Error "Git clone failed."
        exit 1
    }
    $freshClone = $true
} else {
    Write-Host "Source already cloned at $SourceDir" -ForegroundColor Green
}

# Checkout pinned commit (only on fresh clone)
if ($freshClone) {
    Write-Host "Checking out commit $PiperGitCommit ..." -ForegroundColor Cyan
    Push-Location $SourceDir
    try {
        $commitExists = git cat-file -t $PiperGitCommit 2>&1
        if ($LASTEXITCODE -ne 0) {
            Write-Host "  Fetching commit..."
            git fetch origin $PiperGitCommit --depth 1
        }

        git checkout $PiperGitCommit --force
        if ($LASTEXITCODE -ne 0) {
            Write-Error "Failed to checkout commit $PiperGitCommit"
            exit 1
        }
        Write-Host "  Checked out $(git rev-parse --short HEAD)"

        # Apply patches
        foreach ($patch in @($MingwPatch, $DirectMLPatch)) {
            if (-not (Test-Path $patch)) {
                Write-Error "Patch file not found: $patch"
                exit 1
            }
            Write-Host "  Applying $(Split-Path $patch -Leaf)..."
            git apply --verbose $patch
            if ($LASTEXITCODE -ne 0) {
                Write-Error "Failed to apply patch: $patch"
                exit 1
            }
        }
        Write-Host "  Patches applied." -ForegroundColor Green
    } finally {
        Pop-Location
    }
}
Write-Host ""

# --- Helper: Configure, build, install ---
function Build-Variant {
    param(
        [string]$VariantName,
        [string]$BuildDir,
        [string]$InstallDir,
        [string[]]$ExtraCMakeArgs
    )

    Write-Host "=== Building $VariantName variant ===" -ForegroundColor Cyan

    # Configure
    Write-Host "Configuring CMake ($VariantName)..." -ForegroundColor Cyan
    $cmakeArgs = @(
        "-G", "Ninja",
        "-S", $CmakeSourceDir,
        "-B", $BuildDir,
        "-DCMAKE_BUILD_TYPE=$BuildType",
        "-DCMAKE_INSTALL_PREFIX=$InstallDir",
        "-DCMAKE_C_COMPILER=gcc",
        "-DCMAKE_CXX_COMPILER=g++"
    ) + $ExtraCMakeArgs

    & cmake @cmakeArgs
    if ($LASTEXITCODE -ne 0) {
        Write-Error "CMake configure failed ($VariantName)."
        exit 1
    }

    # Build
    Write-Host "Building ($VariantName)..." -ForegroundColor Cyan
    cmake --build $BuildDir --config $BuildType
    if ($LASTEXITCODE -ne 0) {
        Write-Error "CMake build failed ($VariantName)."
        exit 1
    }

    # Install
    Write-Host "Installing ($VariantName)..." -ForegroundColor Cyan
    cmake --install $BuildDir --config $BuildType
    if ($LASTEXITCODE -ne 0) {
        Write-Error "CMake install failed ($VariantName)."
        exit 1
    }

    Write-Host "$VariantName build complete." -ForegroundColor Green
    Write-Host ""
}

# --- Helper: Find and stage runtime files ---
function Stage-Runtime {
    param(
        [string]$InstallDir,
        [string]$DllOutputName,  # e.g. "libpiper.dll" or "libpiper_directml.dll"
        [switch]$CopyEspeakData  # Only copy espeak-ng-data once
    )

    Write-Host "Staging $DllOutputName to $RuntimeDir ..." -ForegroundColor Cyan

    if (-not (Test-Path $RuntimeDir)) {
        New-Item -ItemType Directory -Path $RuntimeDir | Out-Null
    }

    # Find piper shared library
    $PiperLib = $null
    foreach ($name in @("libpiper.dll", "piper.dll")) {
        foreach ($searchDir in @($InstallDir, (Join-Path $InstallDir "lib"), (Join-Path $InstallDir "bin"))) {
            $candidate = Join-Path $searchDir $name
            if (Test-Path $candidate) {
                $PiperLib = $candidate
                break
            }
        }
        if ($PiperLib) { break }
    }

    if (-not $PiperLib) {
        Write-Error "Piper shared library not found in $InstallDir."
        exit 1
    }

    # Copy as the requested output name
    Copy-Item $PiperLib -Destination (Join-Path $RuntimeDir $DllOutputName) -Force
    Write-Host "  Copied $(Split-Path $PiperLib -Leaf) -> $DllOutputName"

    # Copy onnxruntime DLLs
    $InstallLibDir = Join-Path $InstallDir "lib"
    if (Test-Path $InstallLibDir) {
        Get-ChildItem $InstallLibDir -Filter "onnxruntime*" -File | Where-Object {
            $_.Extension -in @(".dll", ".so", ".dylib")
        } | ForEach-Object {
            Copy-Item $_.FullName -Destination $RuntimeDir -Force
            Write-Host "  Copied $($_.Name)"
        }

        # Copy DirectML.dll if present
        $directmlDll = Join-Path $InstallLibDir "DirectML.dll"
        if (Test-Path $directmlDll) {
            Copy-Item $directmlDll -Destination $RuntimeDir -Force
            Write-Host "  Copied DirectML.dll"
        }
    }

    # Copy espeak-ng-data (only once)
    if ($CopyEspeakData) {
        $EspeakDataDir = Join-Path $InstallDir "espeak-ng-data"
        if (Test-Path $EspeakDataDir) {
            $DestEspeakDir = Join-Path $RuntimeDir "espeak-ng-data"
            if (Test-Path $DestEspeakDir) {
                Remove-Item $DestEspeakDir -Recurse -Force
            }
            Copy-Item $EspeakDataDir -Destination $RuntimeDir -Recurse -Force
            Write-Host "  Copied espeak-ng-data/"
        }
    }

    Write-Host ""
}

# --- Build CPU variant ---
if (-not $SkipCPU) {
    Build-Variant `
        -VariantName "CPU" `
        -BuildDir (Join-Path $SafeRoot "build-cpu") `
        -InstallDir (Join-Path $SafeRoot "install-cpu") `
        -ExtraCMakeArgs @()

    Stage-Runtime `
        -InstallDir (Join-Path $SafeRoot "install-cpu") `
        -DllOutputName "libpiper.dll" `
        -CopyEspeakData
}

# --- Build DirectML variant ---
if (-not $SkipDirectML) {
    Build-Variant `
        -VariantName "DirectML" `
        -BuildDir (Join-Path $SafeRoot "build-directml") `
        -InstallDir (Join-Path $SafeRoot "install-directml") `
        -ExtraCMakeArgs @("-DPIPER_USE_DIRECTML=ON")

    Stage-Runtime `
        -InstallDir (Join-Path $SafeRoot "install-directml") `
        -DllOutputName "libpiper_directml.dll"
}

# --- Summary ---
Write-Host "=== Build complete ===" -ForegroundColor Green
Write-Host ""
Write-Host "Contents of $RuntimeDir :"
Get-ChildItem $RuntimeDir | ForEach-Object {
    if ($_.PSIsContainer) {
        Write-Host "  $($_.Name)/" -ForegroundColor Blue
    } else {
        Write-Host "  $($_.Name)  ($([math]::Round($_.Length / 1KB, 1)) KB)"
    }
}
Write-Host ""
Write-Host "Next steps:" -ForegroundColor Yellow
Write-Host "  1. DLLs and espeak-ng-data are in build/bin/"
Write-Host "  2. Run .\dev.ps1 to start development"
Write-Host "  3. Enable piper-native models in config (modelToggles)"
