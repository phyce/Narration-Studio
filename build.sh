#!/bin/bash
set -e

echo "Building CLI application..."
mkdir -p build/bin

# Build the CLI version with the 'cli' tag
go build -tags cli -o build/bin/nstudio-cli .

echo "Building GUI application..."
wails build

echo ""
echo "Build complete!"
echo "GUI App: build/bin/narration_studio"
echo "CLI App: build/bin/nstudio-cli"
