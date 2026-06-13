param(
    [string]$Version = "latest",
    [string]$InstallDir = "$env:LOCALAPPDATA\ocgo\bin",
    [string]$ArchivePath = "",
    [string]$DistDir = "",
    [switch]$NoPath,
    [switch]$Force,
    [switch]$DryRun,
    [switch]$AllowMissingVersion
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

$usingArchive = [System.IO.File]::Exists($ArchivePath)

if ($usingArchive) {
    $zipPath = Resolve-Path -LiteralPath $ArchivePath
    $zipName = Split-Path -Leaf $zipPath

    Write-Host "Using local archive: $zipPath"

    if ($DryRun) {
        Write-Host "[DRY-RUN] Would install from archive: $zipPath"
        Write-Host "[DRY-RUN] Would install to: $InstallDir"
        Write-Host "[DRY-RUN] Architecture: $archSuffix"
        Write-Host "[DRY-RUN] No changes made."
        exit 0
    }
} else {
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
}

$tempDir = Join-Path $env:TEMP ([System.Guid]::NewGuid().ToString())
New-Item -ItemType Directory -Path $tempDir -Force | Out-Null
try {
    if (-not $usingArchive) {
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
    } else {
        $archiveDir = Split-Path -Parent $zipPath
        $checksumsPath = Join-Path $archiveDir "checksums.txt"
        if (-not (Test-Path $checksumsPath) -and $DistDir) {
            $checksumsPath = Join-Path $DistDir "checksums.txt"
        }

        if (Test-Path $checksumsPath) {
            Write-Host "Verifying SHA256 checksum from $checksumsPath ..."
            $expectedLine = Get-Content $checksumsPath | Where-Object { $_ -match [regex]::Escape($zipName) }
            if ($expectedLine) {
                $expectedHash = ($expectedLine -split "\s+")[0].ToLowerInvariant()
                $actualHash = (Get-FileHash -Algorithm SHA256 $zipPath).Hash.ToLowerInvariant()
                if ($actualHash -ne $expectedHash) {
                    Write-Error "SHA256 checksum mismatch for $zipName."
                    Write-Error "  Expected: $expectedHash"
                    Write-Error "  Actual:   $actualHash"
                    exit 1
                }
                Write-Host "Checksum verified."
            } else {
                Write-Host "WARNING: checksums.txt does not contain an entry for $zipName. Skipping checksum verification."
            }
        } else {
            Write-Host "WARNING: checksums.txt not found alongside archive. Skipping checksum verification."
        }
    }

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

    $destExe = Join-Path $InstallDir "ocgo.exe"

    if ((Test-Path $destExe) -and -not $Force) {
        Write-Error "ocgo.exe already exists at $destExe. Use -Force to reinstall."
        exit 1
    }

    Copy-Item -Path $exePath -Destination $destExe -Force
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

if (-not (Test-Path $installedExe)) {
    Write-Error "Installed executable not found at $installedExe."
    exit 1
}

Write-Host "Verifying installed binary ..."
& $installedExe --help | Out-Null
if ($LASTEXITCODE -ne 0) {
    Write-Error "Installed ocgo.exe failed to run with --help."
    exit 1
}

Write-Host ""
Write-Host "Installed version:"
$versionOutput = & $installedExe version 2>&1
if ($LASTEXITCODE -eq 0) {
    Write-Host $versionOutput
} elseif ($AllowMissingVersion) {
    Write-Host "(version metadata not available in this release)"
} else {
    Write-Error "Installed ocgo.exe failed to report version."
    exit 1
}

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
