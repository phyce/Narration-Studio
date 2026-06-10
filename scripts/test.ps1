<#
.SYNOPSIS
    Runs the Go test suite and prints a clean, flat tick/cross line per test.

.DESCRIPTION
    By default skips the API engine tests (OpenAI, ElevenLabs, Google, Gemini) —
    they make real, BILLABLE calls using the keys in your app config or the
    matching *_API_KEY environment variables, and the Gemini suite paces itself
    against rate limits so it is also slow.

    -All    Run everything, including the API engine tests.
    -Run    Pass a `go test -run` pattern to run a subset (e.g. -Run TestSettingsPage).
            The API skip is not applied, so API engine tests can be targeted directly.

.EXAMPLE
    .\scripts\test.ps1            # free tests only
.EXAMPLE
    .\scripts\test.ps1 -All      # everything (API tests cost money)
#>
param(
    [switch]$All,
    [string]$Run = ""
)

$ProjectRoot = (Resolve-Path (Join-Path $PSScriptRoot "..")).Path

# Emoji are built from code points so this file stays pure ASCII (Windows PowerShell 5.1
# mangles non-ASCII in BOM-less scripts). UTF-8 console output so they actually render.
[Console]::OutputEncoding = [System.Text.Encoding]::UTF8
$tick  = [char]::ConvertFromUtf32(0x2705) # white heavy check mark
$cross = [char]::ConvertFromUtf32(0x274C) # cross mark
$dash  = [char]::ConvertFromUtf32(0x23ED) # skip (next track)

Push-Location $ProjectRoot
try {
    $arguments = @('test', './...', '-json', '-count=1', '-timeout', '300s')
    if ($Run -ne "") {
        $arguments += @('-run', $Run)
    }
    elseif (-not $All) {
        $arguments += @('-skip', 'TestOpenAIEngine|TestElevenLabsEngine|TestGoogleEngine|TestGeminiEngine')
    }

    $results = New-Object System.Collections.Generic.List[object]
    $output = @{}
    & go @arguments | ForEach-Object {
        if (-not $_.StartsWith('{')) { return }
        try { $event = $_ | ConvertFrom-Json } catch { return }
        if (-not $event.Test) { return }
        if ($event.Action -eq 'output') {
            if (-not $output.ContainsKey($event.Test)) {
                $output[$event.Test] = New-Object System.Collections.Generic.List[string]
            }
            $output[$event.Test].Add($event.Output)
        }
        elseif ($event.Action -in @('pass', 'fail', 'skip')) {
            $results.Add([pscustomobject]@{ Name = $event.Test; Action = $event.Action })
        }
    }
    $exitCode = $LASTEXITCODE

    # Show only leaf tests; drop parent groups whose name is a prefix of a subtest.
    $names = @($results | ForEach-Object { $_.Name })
    $leaves = $results | Where-Object {
        $prefix = $_.Name + '/'
        -not ($names | Where-Object { $_.StartsWith($prefix) })
    }

    foreach ($result in ($leaves | Sort-Object Name)) {
        $name = $result.Name
        if ($result.Action -eq 'pass') {
            Write-Host "$tick $name"
            continue
        }

        if ($result.Action -eq 'fail') { $icon = $cross; $color = 'Red' }
        else { $icon = $dash; $color = 'DarkGray' }
        Write-Host "$icon $name" -ForegroundColor $color

        # Print the test's own messages (the t.Skip / t.Fatalf reason), dropping framing.
        if ($output.ContainsKey($name)) {
            foreach ($line in $output[$name]) {
                $text = $line.TrimEnd()
                $bare = $text.TrimStart()
                if ($bare -eq '') { continue }
                if ($bare -match '^(=== |--- |ok\s|\?\s|PASS$|FAIL$|FAIL\s)') { continue }
                Write-Host "      $($text.Trim())" -ForegroundColor $color
            }
        }
    }

    $passed  = @($leaves | Where-Object { $_.Action -eq 'pass' }).Count
    $failed  = @($leaves | Where-Object { $_.Action -eq 'fail' }).Count
    $skipped = @($leaves | Where-Object { $_.Action -eq 'skip' }).Count

    Write-Host ""
    if ($leaves.Count -eq 0 -and $exitCode -ne 0) {
        Write-Host "No test results - the suite failed to build. Run 'go build ./...' to see why." -ForegroundColor Red
    }
    else {
        $summary = "$passed passed, $failed failed, $skipped skipped"
        if ($exitCode -eq 0) { Write-Host $summary -ForegroundColor Green }
        else { Write-Host $summary -ForegroundColor Red }
    }
}
finally {
    Pop-Location
}

exit $exitCode
