#!/bin/bash
set -e

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_ROOT"

echo "Building GUI application..."
wails build

echo "Building CLI application..."
mkdir -p build/bin

go build -tags cli -o "build/bin/nstudio-cli-osx" .

echo "Embedding CLI into GUI..."
APP_PATH="build/bin/Narration Studio.app"
if [ ! -d "$APP_PATH" ]; then
    APP_PATH="build/bin/narration_studio.app"
fi

if [ -d "$APP_PATH" ]; then
    cp "build/bin/nstudio-cli-osx" "$APP_PATH/Contents/MacOS/"
    echo "CLI embedded into $APP_PATH/Contents/MacOS/"
else
    echo "Error: Could not find App Bundle to embed CLI."
    exit 1
fi

echo ""
echo "Build complete!"
echo "GUI App: $APP_PATH"
echo "CLI App: build/bin/nstudio-cli-osx"
