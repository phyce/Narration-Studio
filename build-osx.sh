#!/bin/bash
set -e

echo "Building GUI application..."
wails build

echo "Building CLI application..."
mkdir -p build/bin

# Build the CLI version with the 'cli' tag
go build -tags cli -o "build/bin/nstudio-cli-osx" .

echo "Embedding CLI into GUI..."
# Determine App Bundle name
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
