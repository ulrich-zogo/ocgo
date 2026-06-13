param(
    [switch]$IncludeNetworkInstall
)

$ErrorActionPreference = "Stop"

$ScriptDir = $PSScriptRoot
$InstallerPath = Join-Path $ScriptDir "install-windows.ps1"
$TempRoot = Join-Path $env:TEMP ("ocgo-install-test-" + [Guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $TempRoot -Force | Out-Null

$Global:TestsPassed = 0
$Global:TestsFailed = 0

function Output-Matches {
    param([array]$Output, [string]$Pattern)
    foreach ($item in $Output) {
        if ("$item" -match $Pattern) { return $true }
    }
    return $false
}

function Invoke-Installer {
    param([string[]]$Arguments)

    # Isolate from $ErrorActionPreference = "Stop": *>&1 on a native command
    # wraps stderr as ErrorRecord objects, which become terminating errors
    # when $ErrorActionPreference is "Stop".
    $savedEAP = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try {
        $output = & powershell -ExecutionPolicy Bypass -File $InstallerPath @Arguments *>&1
        $exitCode = $LASTEXITCODE
    } finally {
        $ErrorActionPreference = $savedEAP
    }

    return [pscustomobject]@{
        ExitCode = $exitCode
        Output   = $output
    }
}

function New-MockOcgoArchive {
    param(
        [int]$ExitCodeHelp = 0,
        [int]$ExitCodeVersion = 0,
        [string]$TempDir
    )

    $mockDir = Join-Path $TempDir "mock-src"
    New-Item -ItemType Directory -Path $mockDir -Force | Out-Null

    @"
module mock-ocgo

go 1.22
"@ | Set-Content -Path (Join-Path $mockDir "go.mod")

    $versionLine = if ($ExitCodeVersion -eq 0) {
        'fmt.Println("ocgo mock version v0.0.0-test")'
    } else {
        'fmt.Println("mock version unavailable")'
    }

    @"
package main
import (
    "fmt"
    "os"
)
func main() {
    if len(os.Args) > 1 && os.Args[1] == "--help" {
        os.Exit($ExitCodeHelp)
    }
    if len(os.Args) > 1 && os.Args[1] == "version" {
        $versionLine
        os.Exit($ExitCodeVersion)
    }
    fmt.Println("mock ocgo binary")
}
"@ | Set-Content -Path (Join-Path $mockDir "main.go")

    Push-Location $mockDir
    try {
        go build -o ocgo.exe .
    } finally {
        Pop-Location
    }

    $exePath = Join-Path $mockDir "ocgo.exe"
    if (-not (Test-Path $exePath)) {
        throw "Mock compilation failed: ocgo.exe not produced."
    }

    $archiveDir = Join-Path $TempDir "archive-contents"
    New-Item -ItemType Directory -Path $archiveDir -Force | Out-Null
    Copy-Item -Path $exePath -Destination $archiveDir

    $zipPath = Join-Path $TempDir "mock-ocgo.zip"
    Compress-Archive -Path (Join-Path $archiveDir "*") -DestinationPath $zipPath -Force

    return $zipPath
}

function Find-Go {
    $goCmd = Get-Command "go" -ErrorAction SilentlyContinue
    if ($goCmd) { return $true }

    $fallbackGo = "$env:USERPROFILE\go-install\go\bin\go.exe"
    if (Test-Path $fallbackGo) {
        $env:GOROOT = "$env:USERPROFILE\go-install\go"
        $env:PATH = "$env:GOROOT\bin;$env:PATH"
        $env:GOPATH = "$env:USERPROFILE\go"
        $goCmd = Get-Command "go" -ErrorAction SilentlyContinue
        if ($goCmd) { return $true }
    }

    return $false
}

function Run-Test {
    param([string]$Name, [scriptblock]$Block)
    Write-Host "Test $Name ... " -NoNewline
    try {
        & $Block
        Write-Host "[PASS]" -ForegroundColor Green
        $Global:TestsPassed++
    } catch {
        Write-Host "[FAIL]" -ForegroundColor Red
        Write-Host "  $($_.Exception.Message)"
        $Global:TestsFailed++
    }
}

try {
    $goFound = Find-Go

    if (-not $goFound) {
        Write-Warning "Go not found. Skipping tests 1-5 (mock-based)."
    }

    # --- Test 1: Version OK ---
    if ($goFound) {
        Run-Test "1/5 Version OK" {
            $testDir = Join-Path $TempRoot "test-version-ok"
            New-Item -ItemType Directory -Path $testDir -Force | Out-Null
            $zip = New-MockOcgoArchive -ExitCodeHelp 0 -ExitCodeVersion 0 -TempDir $testDir
            $installDir = Join-Path $testDir "inst"
            $result = Invoke-Installer -Arguments @("-ArchivePath", $zip, "-InstallDir", $installDir, "-NoPath", "-Force")
            if ($result.ExitCode -ne 0) { throw "Expected exit 0, got $($result.ExitCode)" }
            if (-not (Output-Matches $result.Output "ocgo mock version v0.0.0-test")) { throw "Version output not found" }
            if (-not (Output-Matches $result.Output "OCGO installed successfully")) { throw "Success message not found" }
            if (-not (Test-Path (Join-Path $installDir "ocgo.exe"))) { throw "ocgo.exe not installed" }
        }
    }

    # --- Test 2: Version missing ---
    if ($goFound) {
        Run-Test "2/5 Version missing" {
            $testDir = Join-Path $TempRoot "test-version-missing"
            New-Item -ItemType Directory -Path $testDir -Force | Out-Null
            $zip = New-MockOcgoArchive -ExitCodeHelp 0 -ExitCodeVersion 1 -TempDir $testDir
            $installDir = Join-Path $testDir "inst"
            $result = Invoke-Installer -Arguments @("-ArchivePath", $zip, "-InstallDir", $installDir, "-NoPath", "-Force")
            if ($result.ExitCode -ne 0) { throw "Expected exit 0, got $($result.ExitCode)" }
            if (-not (Output-Matches $result.Output "did not report a version")) { throw "Warning about missing version not found" }
            if (-not (Output-Matches $result.Output "OCGO installed successfully")) { throw "Success message not found" }
            if (-not (Test-Path (Join-Path $installDir "ocgo.exe"))) { throw "ocgo.exe not installed" }
        }
    }

    # --- Test 3: Version missing with -AllowMissingVersion ---
    if ($goFound) {
        Run-Test "3/5 Version missing with -AllowMissingVersion" {
            $testDir = Join-Path $TempRoot "test-version-allow"
            New-Item -ItemType Directory -Path $testDir -Force | Out-Null
            $zip = New-MockOcgoArchive -ExitCodeHelp 0 -ExitCodeVersion 1 -TempDir $testDir
            $installDir = Join-Path $testDir "inst"
            $result = Invoke-Installer -Arguments @("-ArchivePath", $zip, 
"-InstallDir", $installDir, "-NoPath", "-Force", "-AllowMissingVersion")
            if ($result.ExitCode -ne 0) { throw "Expected exit 0, got $($result.ExitCode)" }
            if (-not (Output-Matches $result.Output "version metadata not available in this release")) { throw "AllowMissingVersion message not found" }
            if (-not (Output-Matches $result.Output "OCGO installed successfully")) { throw "Success message not found" }
            if (-not (Test-Path (Join-Path $installDir "ocgo.exe"))) { throw "ocgo.exe not installed" }
        }
    }

    # --- Test 4: --help fails ---
    if ($goFound) {
        Run-Test "4/5 --help fails" {
            $testDir = Join-Path $TempRoot "test-help-fails"
            New-Item -ItemType Directory -Path $testDir -Force | Out-Null
            $zip = New-MockOcgoArchive -ExitCodeHelp 1 -ExitCodeVersion 1 -TempDir $testDir
            $installDir = Join-Path $testDir "inst"
            $result = Invoke-Installer -Arguments @("-ArchivePath", $zip, "-InstallDir", $installDir, "-NoPath", "-Force")
            if ($result.ExitCode -eq 0) { throw "Expected non-zero exit, got 0" }
            if (-not (Output-Matches $result.Output "failed to run with --help")) { throw "Help failure message not found" }
            if (Output-Matches $result.Output "OCGO installed successfully") { throw "Unexpected success message" }
        }
    }

    # --- Test 5: Archive without ocgo.exe ---
    if ($goFound) {
        Run-Test "5/5 Archive without ocgo.exe" {
            $testDir = Join-Path $TempRoot "test-no-exe"
            New-Item -ItemType Directory -Path $testDir -Force | Out-Null
            $dummyDir = Join-Path $testDir "dummy"
            New-Item -ItemType Directory -Path $dummyDir -Force | Out-Null
            Set-Content -Path (Join-Path $dummyDir "readme.txt") -Value "no binary here"
            $badZip = Join-Path $testDir "bad-archive.zip"
            Compress-Archive -Path (Join-Path $dummyDir "*") -DestinationPath $badZip -Force
            $installDir = Join-Path $testDir "inst"
            $result = Invoke-Installer -Arguments @("-ArchivePath", $badZip, 
"-InstallDir", $installDir, "-NoPath", "-Force")
            if ($result.ExitCode -eq 0) { throw "Expected non-zero exit, got 0" }
            if (-not (Output-Matches $result.Output "not found after extraction")) { throw "Binary not found message expected" }
            if (Output-Matches $result.Output "OCGO installed successfully") { throw "Unexpected success message" }
        }
    }

    # --- Test 6: Network install (opt-in) ---
    $doNetwork = $IncludeNetworkInstall -or ($env:OCGO_TEST_NETWORK_INSTALL -eq "1")
    if (-not $doNetwork) {
        Write-Host "Test 6/6 Network install: (skipped, use -IncludeNetworkInstall or `$env:OCGO_TEST_NETWORK_INSTALL=1)"
    } else {
        Run-Test "6/6 Network install" {
            $networkDir = Join-Path $TempRoot "test-network"
            New-Item -ItemType Directory -Path $networkDir -Force | Out-Null
            $result = Invoke-Installer -Arguments @("-InstallDir", $networkDir, 
"-NoPath", "-Force")
            if ($result.ExitCode -ne 0) { throw "Network install failed with exit $($result.ExitCode)" }
            $exePath = Join-Path $networkDir "ocgo.exe"
            if (-not (Test-Path $exePath)) { throw "ocgo.exe not found after network install" }
            $helpOut = & $exePath --help 2>&1
            if ($LASTEXITCODE -ne 0) { throw "ocgo --help failed after network install" }
            $listOut = & $exePath list 2>&1
            if ($LASTEXITCODE -ne 0) { throw "ocgo list failed after network install" }
        }
    }
} finally {
    Remove-Item -Recurse -Force $TempRoot -ErrorAction SilentlyContinue
}

Write-Host ""
Write-Host "Results: $TestsPassed passed, $TestsFailed failed."
if ($TestsFailed -gt 0) { exit 1 }
exit 0
