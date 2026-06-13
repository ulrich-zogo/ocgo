param(
    [string]$Dist = "dist",
    [string]$Version = "",
    [switch]$KeepTemp
)

$ErrorActionPreference = "Stop"

if (-not (Test-Path $Dist)) {
    Write-Error "ERROR: dist directory not found: $Dist"
    Write-Error "Run '.\scripts\build-release-artifacts.ps1 v0.0.0-smoke' first, or build with bash."
    exit 1
}

$arch = [System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture
$archMap = @{
    "X64"   = "x86_64"
    "Arm64" = "arm64"
}
if (-not $archMap.ContainsKey("$arch")) {
    Write-Error "Unsupported architecture: $arch. Supported: X64, Arm64."
    exit 1
}
$archSuffix = $archMap["$arch"]
$archAlt = if ($archSuffix -eq "x86_64") { "amd64" } else { "ARM64" }

$osVar = "windows"
$osVarAlt = "Windows"

$candidates = @()
if ($Version) {
    $v = $Version -replace "^v", ""
    foreach ($ov in @($osVar, $osVarAlt)) {
        foreach ($av in @($archSuffix, $archAlt)) {
            $candidates += "ocgo_${v}_${ov}_${av}.zip"
        }
    }
} else {
    foreach ($ov in @($osVar, $osVarAlt)) {
        foreach ($av in @($archSuffix, $archAlt)) {
            $candidates += "ocgo_*_${ov}_${av}.zip"
        }
    }
}

$matches = @()
foreach ($pattern in $candidates) {
    $found = @(Get-ChildItem -Path "$Dist\$pattern" -ErrorAction SilentlyContinue)
    foreach ($f in $found) {
        if ($matches -notcontains $f.FullName) {
            $matches += $f.FullName
        }
    }
}

if ($matches.Count -eq 0) {
    Write-Error "ERROR: no archive found for windows/${archSuffix} in $Dist"
    Write-Error "Searched patterns:"
    foreach ($p in $candidates) { Write-Error "  $p" }
    Write-Error "Available files:"
    Get-ChildItem $Dist -ErrorAction SilentlyContinue | ForEach-Object { Write-Error "  $($_.Name)" }
    exit 1
}

if ($matches.Count -gt 1) {
    Write-Error "ERROR: multiple archives matched for windows/${archSuffix}:"
    foreach ($m in $matches) { Write-Error "  $m" }
    exit 1
}

$archive = $matches[0]
$archiveName = Split-Path -Leaf $archive
Write-Host "Found archive: $archiveName"

$checksumsPath = Join-Path $Dist "checksums.txt"
if (-not (Test-Path $checksumsPath)) {
    Write-Error "ERROR: checksums.txt not found in $Dist"
    exit 1
}

Write-Host "Verifying SHA256 checksum ..."
$expectedLine = Get-Content $checksumsPath | Where-Object { $_ -match [regex]::Escape($archiveName) }
if (-not $expectedLine) {
    Write-Error "checksums.txt does not contain an entry for $archiveName"
    exit 1
}
$expectedHash = ($expectedLine -split "\s+")[0].ToLowerInvariant()
$actualHash = (Get-FileHash -Algorithm SHA256 $archive).Hash.ToLowerInvariant()
if ($actualHash -ne $expectedHash) {
    Write-Error "SHA256 checksum mismatch for $archiveName"
    Write-Error "  Expected: $expectedHash"
    Write-Error "  Actual:   $actualHash"
    exit 1
}
Write-Host "Checksum verified."

$tmpRoot = Join-Path $env:TEMP ("ocgo-smoke-" + [Guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $tmpRoot -Force | Out-Null
if (-not $KeepTemp) {
    Register-EngineEvent -SourceIdentifier PowerShell.Exiting -Action { Remove-Item -Recurse -Force $tmpRoot -ErrorAction SilentlyContinue } | Out-Null
}

try {
    $extractDir = Join-Path $tmpRoot "extract"
    Write-Host "Extracting $archiveName ..."
    Expand-Archive -Path $archive -DestinationPath $extractDir

    Write-Host "Checking archive contents ..."
    $readme = Get-ChildItem -Recurse -Filter "README.md" -Path $extractDir | Select-Object -First 1
    if (-not $readme) { Write-Error "ERROR: README.md not found in archive"; exit 1 }
    Write-Host "  README.md found"

    $license = Get-ChildItem -Recurse -Filter "LICENSE" -Path $extractDir | Select-Object -First 1
    if (-not $license) { Write-Error "ERROR: LICENSE not found in archive"; exit 1 }
    Write-Host "  LICENSE found"

    $bin = Get-ChildItem -Recurse -Filter "ocgo.exe" -Path $extractDir | Select-Object -First 1 -ExpandProperty FullName
    if (-not $bin) { Write-Error "ERROR: ocgo.exe not found in archive"; exit 1 }
    Write-Host "  Binary found: $bin"

    $tmpHome = Join-Path $tmpRoot "home"
    New-Item -ItemType Directory -Path $tmpHome -Force | Out-Null

    $oldHome = $env:HOME
    $oldUserProfile = $env:USERPROFILE
    $oldApiKey = $env:OCGO_API_KEY
    $oldHomeDrive = $env:HOMEDRIVE
    $oldHomePath = $env:HOMEPATH

    try {
        $env:HOME = $tmpHome
        $env:USERPROFILE = $tmpHome
        $env:HOMEDRIVE = ""
        $env:HOMEPATH = $tmpHome
        $env:OCGO_API_KEY = "test-key"

        function Validate-Json($json, $desc) {
            try {
                $json | ConvertFrom-Json | Out-Null
                Write-Host "  $desc : valid JSON"
            } catch {
                Write-Error "ERROR: $desc is not valid JSON"
                Write-Error "Output: $json"
                exit 1
            }
        }

        Write-Host ""
        Write-Host "=== smoke: version ==="
        $output = & $bin version 2>&1
        if ($LASTEXITCODE -ne 0) { Write-Error "ERROR: version failed`n$output"; exit 1 }
        Write-Host "  OK"

        Write-Host ""
        Write-Host "=== smoke: version --json ==="
        $jsonOutput = & $bin version --json 2>&1
        if ($LASTEXITCODE -ne 0) { Write-Error "ERROR: version --json failed`n$jsonOutput"; exit 1 }
        Validate-Json $jsonOutput "version --json"

        Write-Host ""
        Write-Host "=== smoke: --help ==="
        $helpOutput = & $bin --help 2>&1
        Write-Host "  OK"

        Write-Host ""
        Write-Host "=== smoke: models ==="
        $modelsOutput = & $bin models 2>&1
        if ($LASTEXITCODE -ne 0) { Write-Error "ERROR: models failed`n$modelsOutput"; exit 1 }

        $officialModels = @(
            "minimax-m3", "minimax-m2.7", "minimax-m2.5",
            "kimi-k2.6", "kimi-k2.5",
            "glm-5.1", "glm-5",
            "deepseek-v4-pro", "deepseek-v4-flash",
            "qwen3.7-max", "qwen3.7-plus", "qwen3.6-plus", "qwen3.5-plus",
            "mimo-v2-pro", "mimo-v2-omni", "mimo-v2.5-pro", "mimo-v2.5",
            "hy3-preview"
        )

        $missing = 0
        foreach ($model in $officialModels) {
            if ($modelsOutput -notmatch [regex]::Escape($model)) {
                Write-Host "  ERROR: model '$model' not found in models output"
                $missing++
            }
        }
        if ($missing -gt 0) {
            Write-Error "ERROR: $missing official model(s) missing"
            exit 1
        }
        Write-Host "  All 18 official models present"

        Write-Host ""
        Write-Host "=== smoke: doctor --json ==="
        $doctorErr = Join-Path $tmpRoot "doctor-err.txt"
        $doctorOutput = & $bin doctor --json 2>$doctorErr
        $doctorExit = $LASTEXITCODE
        Validate-Json $doctorOutput "doctor --json"
        Write-Host "  doctor exit code: $doctorExit (non-zero is acceptable)"

        Write-Host ""
        Write-Host "=== smoke: install-windows.ps1 -DryRun ==="
        $installer = Join-Path $PSScriptRoot "install-windows.ps1"
        $dryRunOutput = & $installer -ArchivePath $archive -InstallDir (Join-Path $tmpRoot "install-dry") -NoPath -DryRun 2>&1
        if ($LASTEXITCODE -ne 0) { Write-Error "ERROR: install-windows.ps1 -DryRun failed`n$dryRunOutput"; exit 1 }
        $dryInstallDir = Join-Path $tmpRoot "install-dry"
        if (Test-Path $dryInstallDir) {
            Write-Error "ERROR: -DryRun created install directory"
            exit 1
        }
        Write-Host "  Dry-run OK (no files created)"

        Write-Host ""
        Write-Host "=== smoke: install-windows.ps1 -ArchivePath ==="
        $installDir = Join-Path $tmpRoot "install"
        $installOutput = & $installer -ArchivePath $archive -InstallDir $installDir -NoPath -Force 2>&1
        if ($LASTEXITCODE -ne 0) { Write-Error "ERROR: install-windows.ps1 failed`n$installOutput"; exit 1 }
        $installedExe = Join-Path $installDir "ocgo.exe"
        if (-not (Test-Path $installedExe)) {
            Write-Error "ERROR: ocgo.exe not found at $installDir after installation"
            exit 1
        }
        Write-Host "  Installed to $installDir"

        Write-Host ""
        Write-Host "=== smoke: installed binary ==="
        $installedVersion = & $installedExe version 2>&1
        if ($LASTEXITCODE -ne 0) { Write-Error "ERROR: installed version failed`n$installedVersion"; exit 1 }
        Write-Host "  OK"

        $installedHelp = & $installedExe --help 2>&1
        if ($LASTEXITCODE -ne 0) { Write-Error "ERROR: installed --help failed`n$installedHelp"; exit 1 }
        Write-Host "  Help OK"

    } finally {
        $env:HOME = $oldHome
        $env:USERPROFILE = $oldUserProfile
        $env:OCGO_API_KEY = $oldApiKey
        $env:HOMEDRIVE = $oldHomeDrive
        $env:HOMEPATH = $oldHomePath
    }
} finally {
    if (-not $KeepTemp) {
        Remove-Item -Recurse -Force $tmpRoot -ErrorAction SilentlyContinue
    } else {
        Write-Host "Temp dir: $tmpRoot"
    }
}

Write-Host ""
Write-Host "All smoke tests passed for $archiveName."