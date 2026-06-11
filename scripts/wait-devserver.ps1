<#
.SYNOPSIS
    Blocks until the Vite dev server accepts connections, then exits.

.DESCRIPTION
    Used as a blocking step in build/config.yml's dev_mode pipeline. The Wails
    app polls the frontend dev server for a hardcoded 5 seconds before fataling
    (alpha.98, application_dev.go), which a cold Vite start (~6-8s) can lose.
    This script makes the dev exe launch only after Vite is actually up: ~50ms
    when Vite is already running, exactly-as-long-as-needed when it is cold.
#>
param(
    [int]$Port = 0,
    [int]$TimeoutSeconds = 60
)

if ($Port -eq 0) {
    $Port = 9245
    if ($env:WAILS_VITE_PORT) { $Port = [int]$env:WAILS_VITE_PORT }
}

$deadline = (Get-Date).AddSeconds($TimeoutSeconds)
while ((Get-Date) -lt $deadline) {
    try {
        (New-Object Net.Sockets.TcpClient("127.0.0.1", $Port)).Close()
        exit 0
    } catch {
        Start-Sleep -Milliseconds 200
    }
}

Write-Host "Vite dev server on port $Port did not start within $TimeoutSeconds seconds"
exit 1
