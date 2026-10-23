#!/bin/bash
set -e

PROJECT_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$PROJECT_ROOT"

BINARY_NAME="narration_studio"
CLI_NAME="nstudio-cli-osx"
APP_PATH="build/bin/Narration Studio.app"
VERSION="$(sed -nE 's/.*"productVersion": *"([^"]+)".*/\1/p' wails.json)"

export CGO_ENABLED=1
export MACOSX_DEPLOYMENT_TARGET="12.0"
export CGO_CFLAGS="-mmacosx-version-min=$MACOSX_DEPLOYMENT_TARGET"
export CGO_LDFLAGS="-mmacosx-version-min=$MACOSX_DEPLOYMENT_TARGET"

mkdir -p build/bin

echo "Building frontend..."
(cd frontend && npm install && npm run build)

for ARCH in arm64 amd64; do
    echo "Building GUI application ($ARCH)..."
    GOARCH=$ARCH go build -tags production -ldflags "-w -s" -o "build/bin/$BINARY_NAME-$ARCH" .
    echo "Building CLI application ($ARCH)..."
    GOARCH=$ARCH go build -tags cli -o "build/bin/$CLI_NAME-$ARCH" .
done

echo "Creating universal binaries..."
lipo -create -output "build/bin/$BINARY_NAME" "build/bin/$BINARY_NAME-arm64" "build/bin/$BINARY_NAME-amd64"
lipo -create -output "build/bin/$CLI_NAME" "build/bin/$CLI_NAME-arm64" "build/bin/$CLI_NAME-amd64"
rm -f "build/bin/$BINARY_NAME-arm64" "build/bin/$BINARY_NAME-amd64" "build/bin/$CLI_NAME-arm64" "build/bin/$CLI_NAME-amd64"

echo "Creating app bundle..."
rm -rf "$APP_PATH"
mkdir -p "$APP_PATH/Contents/MacOS" "$APP_PATH/Contents/Resources"
cp "build/bin/$BINARY_NAME" "build/bin/$CLI_NAME" "$APP_PATH/Contents/MacOS/"
cp build/darwin/icons.icns "$APP_PATH/Contents/Resources/"
sed "s/{{.Info.ProductVersion}}/$VERSION/g" build/darwin/Info.plist > "$APP_PATH/Contents/Info.plist"
codesign --force --deep --sign - "$APP_PATH"

echo ""
echo "Build complete!"
echo "GUI App: $APP_PATH"
echo "CLI App: build/bin/$CLI_NAME"
