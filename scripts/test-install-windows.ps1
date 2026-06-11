$ErrorActionPreference = "Stop"

$TempRoot = Join-Path $env:TEMP ("ocgo-install-test-" + [Guid]::NewGuid().ToString())
$InstallDir = Join-Path $TempRoot "bin"

try {
    $scriptPath = Join-Path $PSScriptRoot "install-windows.ps1"
    if (-not (Test-Path $scriptPath)) {
        Write-Error "install-windows.ps1 not found at $scriptPath"
        exit 1
    }

    Write-Host "Installing ocgo to $InstallDir ..."
    & $scriptPath -InstallDir $InstallDir -NoPath -Force

    $exePath = Join-Path $InstallDir "ocgo.exe"
    if (-not (Test-Path $exePath)) {
        Write-Error "ocgo.exe not found at $exePath after installation."
        exit 1
    }
    Write-Host "ocgo.exe found at $exePath"

    Write-Host "Running ocgo --help ..."
    $helpOutput = & $exePath --help 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Error "ocgo --help exited with code $LASTEXITCODE"
        exit 1
    }
    Write-Host "  OK"

    Write-Host "Running ocgo models ..."
    $modelsOutput = & $exePath models 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Error "ocgo models exited with code $LASTEXITCODE"
        exit 1
    }
    Write-Host "  OK"

    Write-Host "Running ocgo version ..."
    $versionOutput = & $exePath version 2>&1
    if ($LASTEXITCODE -eq 0) {
        Write-Host $versionOutput
        Write-Host "  OK"

        Write-Host "Running ocgo version --json ..."
        $jsonOutput = & $exePath version --json 2>&1
        if ($LASTEXITCODE -eq 0) {
            $jsonOutput | ConvertFrom-Json | Out-Null
            Write-Host "  OK"
        } else {
            Write-Host "  (version --json not available in this release)"
        }
    } else {
        Write-Host "  (version command not available in this release)"
    }

    Write-Host ""
    Write-Host "All tests passed."
} finally {
    Write-Host ""
    Write-Host "Cleaning up $TempRoot ..."
    Remove-Item -Recurse -Force $TempRoot -ErrorAction SilentlyContinue
}

exit 0
