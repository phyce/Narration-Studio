#!/bin/bash
# Usage: scripts/build.sh [gtk4|gtk3]   (default: whichever is installed, GTK 4 first)
set -e

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_ROOT"

GTK_VERSION="${1:-}"
if [ -z "$GTK_VERSION" ]; then
    if pkg-config --exists gtk4 webkitgtk-6.0 2>/dev/null; then
        GTK_VERSION="gtk4"
    elif pkg-config --exists gtk+-3.0 webkit2gtk-4.1 2>/dev/null; then
        GTK_VERSION="gtk3"
    fi
fi

case "$GTK_VERSION" in
    gtk4)
        echo "Building for GTK 4 + WebKitGTK 6.0"
        GUI_TAGS="production"
        ;;
    gtk3)
        echo "Building for GTK 3 + WebKit2GTK 4.1"
        GUI_TAGS="production,gtk3"
        ;;
    "")
        echo "Error: neither GTK 4 + WebKitGTK 6.0 nor GTK 3 + WebKit2GTK 4.1 found via pkg-config."
        echo "On Debian/Ubuntu, install one of:"
        echo "  GTK 4: sudo apt install libgtk-4-dev libwebkitgtk-6.0-dev"
        echo "  GTK 3: sudo apt install libgtk-3-dev libwebkit2gtk-4.1-dev"
        exit 1
        ;;
    *)
        echo "Error: unknown GTK version '$GTK_VERSION' (use gtk4 or gtk3)."
        exit 1
        ;;
esac

if ! pkg-config --exists alsa 2>/dev/null; then
    echo "Error: ALSA development library not found."
    echo "Install it with: sudo apt install libasound2-dev"
    exit 1
fi

mkdir -p build/bin

echo "Building frontend..."
(cd frontend && npm install && npm run build)

echo "Building CLI application..."
go build -tags cli -o build/bin/nstudio-cli .

echo "Building GUI application..."
CGO_ENABLED=1 go build -tags "$GUI_TAGS" -ldflags "-w -s" -o build/bin/narration_studio .

echo ""
echo "Build complete!"
echo "GUI App: build/bin/narration_studio"
echo "CLI App: build/bin/nstudio-cli"
