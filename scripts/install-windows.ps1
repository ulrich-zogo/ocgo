param(
    [string]$Version = "latest",
    [string]$InstallDir = "$env:LOCALAPPDATA\ocgo\bin",
    [switch]$NoPath,
    [switch]$Force,
    [switch]$DryRun
)

$ErrorActionPreference = "Stop"

if ($env:OS -ne "Windows_NT") {
    Write-Error "This script is intended for Windows only."
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

if ($Version -eq "latest") {
    $releaseUrl = "https://api.github.com/repos/ulrich-zogo/ocgo/releases/latest"
    Write-Host "Fetching latest release from $releaseUrl ..."
    $release = Invoke-RestMethod -Uri $releaseUrl -UseBasicParsing
    $tag = $release.tag_name
    Write-Host "Latest release tag: $tag"
} else {
    $tag = if ($Version -match "^v") { $Version } else { "v$Version" }
    $releaseUrl = "https://api.github.com/repos/ulrich-zogo/ocgo/releases/tags/$tag"
    Write-Host "Fetching release $tag from $releaseUrl ..."
    try {
        $release = Invoke-RestMethod -Uri $releaseUrl -UseBasicParsing
    } catch {
        Write-Error "Release $tag not found."
        exit 1
    }
}

$versionNumber = $tag -replace "^v", ""
$zipName = "ocgo_${versionNumber}_windows_${archSuffix}.zip"
$assetUrl = "https://github.com/ulrich-zogo/ocgo/releases/download/$tag/$zipName"
$checksumsUrl = "https://github.com/ulrich-zogo/ocgo/releases/download/$tag/checksums.txt"

$asset = $release.assets | Where-Object { $_.name -eq $zipName }
if (-not $asset) {
    Write-Error "Asset $zipName not found in release $tag."
    exit 1
}

if ($DryRun) {
    Write-Host "[DRY-RUN] Would download: $assetUrl"
    Write-Host "[DRY-RUN] Would install to: $InstallDir"
    Write-Host "[DRY-RUN] Architecture: $archSuffix"
    Write-Host "[DRY-RUN] No changes made."
    exit 0
}

$tempDir = Join-Path $env:TEMP ([System.Guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $tempDir -Force | Out-Null
try {
    $zipPath = Join-Path $tempDir $zipName
    $checksumsPath = Join-Path $tempDir "checksums.txt"

    Write-Host "Downloading $zipName ..."
    Invoke-WebRequest -Uri $assetUrl -OutFile $zipPath -UseBasicParsing

    Write-Host "Downloading checksums.txt ..."
    Invoke-WebRequest -Uri $checksumsUrl -OutFile $checksumsPath -UseBasicParsing

    $expectedLine = Get-Content $checksumsPath | Where-Object { $_ -match [regex]::Escape($zipName) }
    if (-not $expectedLine) {
        Write-Error "checksums.txt does not contain an entry for $zipName."
        exit 1
    }
    $expectedHash = ($expectedLine -split "\s+")[0].ToLowerInvariant()

    Write-Host "Verifying SHA256 checksum ..."
    $actualHash = (Get-FileHash -Algorithm SHA256 $zipPath).Hash.ToLowerInvariant()
    if ($actualHash -ne $expectedHash) {
        Write-Error "SHA256 checksum mismatch for $zipName."
        Write-Error "  Expected: $expectedHash"
        Write-Error "  Actual:   $actualHash"
        exit 1
    }
    Write-Host "Checksum verified."

    Write-Host "Extracting $zipName ..."
    Expand-Archive -Path $zipPath -DestinationPath $tempDir

    $exePath = Get-ChildItem -Recurse -Filter "ocgo.exe" -Path $tempDir | Select-Object -First 1 -ExpandProperty FullName
    if (-not $exePath) {
        Write-Error "ocgo.exe not found after extraction."
        exit 1
    }

    if (-not (Test-Path $InstallDir)) {
        New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
    }

    Copy-Item -Path $exePath -Destination (Join-Path $InstallDir "ocgo.exe") -Force:$Force
    Write-Host "Installed ocgo.exe to $InstallDir"

} finally {
    Remove-Item -Recurse -Force $tempDir -ErrorAction SilentlyContinue
}

if (-not $NoPath) {
    $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
    if ($userPath -split ";" -notcontains $InstallDir) {
        $newPath = "$InstallDir;$userPath"
        [Environment]::SetEnvironmentVariable("Path", $newPath, "User")
        Write-Host "Added $InstallDir to user PATH (persistent)."
    } else {
        Write-Host "$InstallDir is already in user PATH."
    }
    $env:Path = "$InstallDir;$env:Path"
}

$installedExe = Join-Path $InstallDir "ocgo.exe"
if (Test-Path $installedExe) {
    Write-Host ""
    Write-Host "OCGO installed successfully."
    Write-Host ""
    Write-Host "Next steps:"
    Write-Host "  ocgo setup"
    Write-Host "  ocgo models"
    Write-Host "  ocgo daemon start"
    Write-Host "  ocgo doctor"
    if (-not $NoPath) {
        Write-Host ""
        Write-Host "Open a new terminal if 'ocgo' is not immediately available."
    }
}
