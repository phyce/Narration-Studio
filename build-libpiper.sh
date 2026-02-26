#!/usr/bin/env bash
# Builds libpiper.so from source and stages runtime files for the piper-native engine.
#
# SYNOPSIS
#   ./build-libpiper.sh [--clean] [--gpu] [--build-type TYPE]
#
# DESCRIPTION
#   Fully automated build (CPU or GPU variant):
#   1. Clones piper1-gpl at a pinned commit (if not already present)
#   2. Applies MinGW/Linux compatibility patch (libpiper-mingw.patch) and CUDA patch (libpiper-cuda.patch)
#   3. Builds CPU or GPU variant (libpiper.so)
#   4. Copies runtime files (libpiper.so, onnxruntime, espeak-ng-data) into build/bin/
#
#   Prerequisites:
#     gcc, g++, cmake, ninja, git
#
# OPTIONS
#   --clean           Remove existing build/install directories before building
#   --gpu             Build with CUDA GPU support (Linux x86-64 only)
#   --build-type TYPE CMake build type (default: Release)

set -euo pipefail

# --- ANSI colours ---
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BLUE='\033[0;34m'
NC='\033[0m'

info()    { echo -e "${CYAN}$*${NC}"; }
success() { echo -e "${GREEN}$*${NC}"; }
warn()    { echo -e "${YELLOW}$*${NC}"; }
error()   { echo -e "${RED}ERROR: $*${NC}" >&2; exit 1; }

# --- Configuration ---
PIPER_GIT_REPO="https://github.com/OHF-Voice/piper1-gpl.git"
PIPER_GIT_COMMIT="32b95f8c1f0dc0ce27a6acd1143de331f61af777"

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB_DIR="$PROJECT_ROOT/lib"
SOURCE_DIR="$LIB_DIR/libpiper-src"
MINGW_PATCH="$LIB_DIR/libpiper-mingw.patch"
CUDA_PATCH="$LIB_DIR/libpiper-cuda.patch"
RUNTIME_DIR="$PROJECT_ROOT/build/bin"
SAFE_ROOT="$LIB_DIR/build"
CMAKE_SOURCE_DIR="$SOURCE_DIR/libpiper"
BUILD_TYPE="Release"
CLEAN=false
USE_GPU=false

# --- Parse arguments ---
while [[ $# -gt 0 ]]; do
    case "$1" in
        --clean)
            CLEAN=true
            shift
            ;;
        --gpu)
            USE_GPU=true
            shift
            ;;
        --build-type)
            BUILD_TYPE="${2:?--build-type requires a value}"
            shift 2
            ;;
        *)
            error "Unknown argument: $1"
            ;;
    esac
done

if $USE_GPU; then
    BUILD_DIR="$SAFE_ROOT/build-gpu"
    INSTALL_DIR="$SAFE_ROOT/install-gpu"
else
    BUILD_DIR="$SAFE_ROOT/build-cpu"
    INSTALL_DIR="$SAFE_ROOT/install-cpu"
fi

echo ""
if $USE_GPU; then
    info "=== libpiper build script (Linux / GPU) ==="
else
    info "=== libpiper build script (Linux / CPU) ==="
fi
echo "Repository : $PIPER_GIT_REPO"
echo "Commit     : $PIPER_GIT_COMMIT"
echo "Source     : $SOURCE_DIR"
echo "Runtime    : $RUNTIME_DIR"
echo ""

# --- Check dependencies ---
info "Checking dependencies..."
for tool in gcc g++ cmake ninja git; do
    path="$(command -v "$tool" 2>/dev/null || true)"
    if [[ -z "$path" ]]; then
        error "$tool not found on PATH. Install it and retry."
    fi
    echo "  $tool: $path"
done
echo ""

# --- Clean ---
if $CLEAN; then
    warn "Cleaning..."
    for dir in "$BUILD_DIR" "$INSTALL_DIR"; do
        if [[ -d "$dir" ]]; then
            rm -rf "$dir"
            echo "  Removed $dir"
        fi
    done
    if [[ -d "$RUNTIME_DIR" ]]; then
        for item in libpiper.so libonnxruntime*.so* espeak-ng-data; do
            # Use glob expansion carefully
            for target in "$RUNTIME_DIR"/$item; do
                if [[ -e "$target" ]]; then
                    rm -rf "$target"
                    echo "  Removed $target"
                fi
            done
        done
    fi
    success "Clean complete."
    echo ""
fi

mkdir -p "$SAFE_ROOT"

# --- Clone source and apply patches (only on fresh clone) ---
FRESH_CLONE=false
if [[ ! -d "$SOURCE_DIR/.git" ]]; then
    info "Cloning piper1-gpl..."
    [[ -d "$SOURCE_DIR" ]] && rm -rf "$SOURCE_DIR"
    git clone --depth 50 "$PIPER_GIT_REPO" "$SOURCE_DIR"
    FRESH_CLONE=true
else
    success "Source already cloned at $SOURCE_DIR"
fi

if $FRESH_CLONE; then
    info "Checking out commit $PIPER_GIT_COMMIT ..."
    pushd "$SOURCE_DIR" > /dev/null

    if ! git cat-file -t "$PIPER_GIT_COMMIT" &>/dev/null; then
        echo "  Fetching commit..."
        git fetch origin "$PIPER_GIT_COMMIT" --depth 1
    fi

    git checkout "$PIPER_GIT_COMMIT" --force
    echo "  Checked out $(git rev-parse --short HEAD)"

    # Apply patches
    if [[ ! -f "$MINGW_PATCH" ]]; then
        error "Patch file not found: $MINGW_PATCH"
    fi
    info "  Applying $(basename "$MINGW_PATCH")..."
    git apply --verbose "$MINGW_PATCH"
    success "  Patch applied."

    if [[ ! -f "$CUDA_PATCH" ]]; then
        error "Patch file not found: $CUDA_PATCH"
    fi
    info "  Applying $(basename "$CUDA_PATCH")..."
    git apply --verbose "$CUDA_PATCH"
    success "  Patch applied."

    popd > /dev/null
fi
echo ""

# --- Configure ---
if $USE_GPU; then
    info "=== Configuring CMake (GPU) ==="
else
    info "=== Configuring CMake (CPU) ==="
fi
cmake -G Ninja \
    -S "$CMAKE_SOURCE_DIR" \
    -B "$BUILD_DIR" \
    -DCMAKE_BUILD_TYPE="$BUILD_TYPE" \
    -DCMAKE_INSTALL_PREFIX="$INSTALL_DIR" \
    -DCMAKE_C_COMPILER=gcc \
    -DCMAKE_CXX_COMPILER=g++ \
    ${USE_GPU:+-DPIPER_USE_GPU=ON}

# --- Build ---
if $USE_GPU; then
    info "=== Building (GPU) ==="
else
    info "=== Building (CPU) ==="
fi
cmake --build "$BUILD_DIR" --config "$BUILD_TYPE"

# --- Install ---
if $USE_GPU; then
    info "=== Installing (GPU) ==="
else
    info "=== Installing (CPU) ==="
fi
cmake --install "$BUILD_DIR" --config "$BUILD_TYPE"

if $USE_GPU; then
    success "GPU build complete."
else
    success "CPU build complete."
fi
echo ""

# --- Stage runtime files ---
info "Staging runtime files to $RUNTIME_DIR ..."
mkdir -p "$RUNTIME_DIR"

# Find libpiper.so (check root, lib/, bin/ within install dir)
PIPER_LIB=""
for search_dir in "$INSTALL_DIR" "$INSTALL_DIR/lib" "$INSTALL_DIR/bin"; do
    for name in libpiper.so piper.so; do
        candidate="$search_dir/$name"
        if [[ -f "$candidate" ]]; then
            PIPER_LIB="$candidate"
            break 2
        fi
    done
    # Also match versioned .so files (e.g. libpiper.so.1.0.0)
    for candidate in "$search_dir"/libpiper.so.* "$search_dir"/piper.so.*; do
        if [[ -f "$candidate" ]]; then
            PIPER_LIB="$candidate"
            break 2
        fi
    done
done

if [[ -z "$PIPER_LIB" ]]; then
    error "Piper shared library not found in $INSTALL_DIR"
fi

cp "$PIPER_LIB" "$RUNTIME_DIR/libpiper.so"
echo "  Copied $(basename "$PIPER_LIB") -> libpiper.so"

# Copy onnxruntime .so files
INSTALL_LIB_DIR="$INSTALL_DIR/lib"
if [[ -d "$INSTALL_LIB_DIR" ]]; then
    for f in "$INSTALL_LIB_DIR"/libonnxruntime*.so*; do
        [[ -f "$f" ]] || continue
        cp "$f" "$RUNTIME_DIR/"
        echo "  Copied $(basename "$f")"
    done
fi

# Copy espeak-ng-data
ESPEAK_DATA_SRC="$INSTALL_DIR/espeak-ng-data"
if [[ -d "$ESPEAK_DATA_SRC" ]]; then
    ESPEAK_DATA_DEST="$RUNTIME_DIR/espeak-ng-data"
    [[ -d "$ESPEAK_DATA_DEST" ]] && rm -rf "$ESPEAK_DATA_DEST"
    cp -r "$ESPEAK_DATA_SRC" "$RUNTIME_DIR/"
    echo "  Copied espeak-ng-data/"
fi

echo ""

# --- Summary ---
success "=== Build complete ==="
echo ""
echo "Contents of $RUNTIME_DIR :"
for item in "$RUNTIME_DIR"/*; do
    [[ -e "$item" ]] || continue
    name="$(basename "$item")"
    if [[ -d "$item" ]]; then
        echo -e "  ${BLUE}${name}/${NC}"
    else
        size_kb=$(( ($(stat -c%s "$item") + 512) / 1024 ))
        echo "  $name  ($size_kb KB)"
    fi
done
echo ""
warn "Next steps:"
echo "  1. libpiper.so and espeak-ng-data are in build/bin/"
echo "  2. Run ./build.sh to build the application"
echo "  3. Enable piper-native models in config (modelToggles)"
