$ErrorActionPreference = "Stop"

Write-Host "Building CLI application..."
if (-not (Test-Path "build/bin")) {
    New-Item -ItemType Directory -Path "build/bin" | Out-Null
}

# Build the CLI version with the 'cli' tag
go build -tags cli -o build/bin/nstudio-cli.exe .

Write-Host "Preparing CLI for installer..."
# Copy CLI to installer directory so NSIS can include it
Copy-Item -Path "build/bin/nstudio-cli.exe" -Destination "build/windows/installer/nstudio-cli.exe" -Force

Write-Host "Building GUI application..."
wails build

Write-Host "Build complete!"
Write-Host "GUI App: build/bin/Narration Studio.exe"
Write-Host "CLI App: build/bin/nstudio-cli.exe"
Write-Host ""
Write-Host "Note: If building with NSIS (wails build -nsis), the CLI will be included in the installer."
