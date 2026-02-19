#!/bin/bash
set -e

WAILS_TAGS=""
if pkg-config --exists webkit2gtk-4.1 2>/dev/null; then
    echo "Detected webkit2gtk-4.1 (newer distro)"
    WAILS_TAGS="-tags webkit2_41"
elif pkg-config --exists webkit2gtk-4.0 2>/dev/null; then
    echo "Detected webkit2gtk-4.0 (older distro)"
else
    echo "Warning: Neither webkit2gtk-4.1 nor webkit2gtk-4.0 found via pkg-config."
    echo "On Debian/Ubuntu, install one of:"
    echo "  Newer (Debian 13+, Ubuntu 24.04+): sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev"
    echo "  Older (Debian 12, Ubuntu 22.04):   sudo apt install libgtk-3-dev libwebkit2gtk-4.0-dev"
    exit 1
fi

if ! pkg-config --exists alsa 2>/dev/null; then
    echo "Warning: ALSA development library not found."
    echo "Install it with: sudo apt install libasound2-dev"
    exit 1
fi
echo "Detected ALSA"

echo "Building CLI application..."
mkdir -p build/bin

# Build the CLI version with the 'cli' tag
go build -tags cli -o build/bin/nstudio-cli .

echo "Building GUI application..."
wails build $WAILS_TAGS

echo ""
echo "Build complete!"
echo "GUI App: build/bin/narration_studio"
echo "CLI App: build/bin/nstudio-cli"