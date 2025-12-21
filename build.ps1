$ErrorActionPreference = "Stop"

Write-Host "Building GUI application..."
wails build

Write-Host "Building CLI application..."
if (-not (Test-Path "build/bin")) {
    New-Item -ItemType Directory -Path "build/bin" | Out-Null
}

# Build the CLI version with the 'cli' tag
go build -tags cli -o build/bin/nstudio-cli.exe .

Write-Host "Build complete!"
Write-Host "GUI App: build/bin/Narration Studio.exe"
Write-Host "CLI App: build/bin/nstudio-cli.exe"
