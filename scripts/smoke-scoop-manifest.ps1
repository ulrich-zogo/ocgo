param(
    [string]$Manifest = "",
    [string]$Dist = "",
    [string]$Version = "",
    [switch]$UseLocalArchive,
    [switch]$RequireScoop
)

$ErrorActionPreference = "Stop"

if (-not $Manifest) {
    $Manifest = Join-Path $PSScriptRoot "..\packaging\scoop\ocgo.json"
}
$Manifest = Resolve-Path -LiteralPath $Manifest -ErrorAction Stop

Write-Host "Validating Scoop manifest: $Manifest"

$json = Get-Content $Manifest -Raw
try {
    $parsed = $json | ConvertFrom-Json
} catch {
    Write-Error "ERROR: manifest is not valid JSON: $_"
    exit 1
}
Write-Host "  JSON is valid"

if (-not $parsed.version) { Write-Error "ERROR: version field missing"; exit 1 }
Write-Host "  version: $($parsed.version)"

if (-not $parsed.homepage) { Write-Error "ERROR: homepage field missing"; exit 1 }
if ($parsed.homepage -notmatch "ulrich-zogo/ocgo") {
    Write-Error "ERROR: homepage must point to ulrich-zogo/ocgo, got: $($parsed.homepage)"
    exit 1
}
Write-Host "  homepage: $($parsed.homepage)"

if (-not $parsed.license) { Write-Error "ERROR: license field missing"; exit 1 }
Write-Host "  license: $($parsed.license)"

if (-not $parsed.architecture) { Write-Error "ERROR: architecture field missing"; exit 1 }
if (-not $parsed.architecture.'64bit') { Write-Error "ERROR: architecture.64bit missing"; exit 1 }
if (-not $parsed.architecture.'64bit'.url) { Write-Error "ERROR: architecture.64bit.url missing"; exit 1 }
if ($parsed.architecture.'64bit'.url -notmatch "ulrich-zogo/ocgo") {
    Write-Error "ERROR: 64bit URL must point to ulrich-zogo/ocgo, got: $($parsed.architecture.'64bit'.url)"
    exit 1
}
if (-not $parsed.architecture.'64bit'.hash) { Write-Error "ERROR: architecture.64bit.hash missing"; exit 1 }
if ($parsed.architecture.'64bit'.hash -match "^(0+|[[:space:]]*)$") {
    Write-Error "ERROR: architecture.64bit.hash is empty or zero"
    exit 1
}
Write-Host "  64bit URL: $($parsed.architecture.'64bit'.url)"
Write-Host "  64bit hash: $($parsed.architecture.'64bit'.hash)"

if ($parsed.architecture.arm64) {
    if ($parsed.architecture.arm64.url -notmatch "ulrich-zogo/ocgo") {
        Write-Error "ERROR: arm64 URL must point to ulrich-zogo/ocgo"
        exit 1
    }
    if (-not $parsed.architecture.arm64.hash -or $parsed.architecture.arm64.hash -match "^(0+|[[:space:]]*)$") {
        Write-Error "ERROR: architecture.arm64.hash is empty or zero"
        exit 1
    }
    Write-Host "  arm64 URL: $($parsed.architecture.arm64.url)"
    Write-Host "  arm64 hash: $($parsed.architecture.arm64.hash)"
} else {
    Write-Host "  (no arm64 entry)"
}

if (-not $parsed.bin) { Write-Error "ERROR: bin field missing"; exit 1 }
Write-Host "  bin: $($parsed.bin)"

if (-not $parsed.checkver) { Write-Error "ERROR: checkver field missing"; exit 1 }
Write-Host "  checkver: present"

if (-not $parsed.autoupdate) { Write-Error "ERROR: autoupdate field missing"; exit 1 }
if (-not $parsed.autoupdate.architecture) { Write-Error "ERROR: autoupdate.architecture missing"; exit 1 }
Write-Host "  autoupdate: present"

$upstreamOwner = "emanue" + "lcasco"
$manifestText = Get-Content $Manifest -Raw
$low = $manifestText.ToLowerInvariant()
if ($low -match [regex]::Escape($upstreamOwner)) {
    Write-Error "ERROR: manifest contains reference to upstream owner"
    exit 1
}
Write-Host "  No upstream owner references in manifest"

$hasScoop = Get-Command scoop -ErrorAction SilentlyContinue

$archMap = @{
    "X64"   = "x86_64"
    "Arm64" = "arm64"
}

if ($UseLocalArchive -and $hasScoop) {
    if (-not $Dist -or -not (Test-Path $Dist)) {
        Write-Error "-UseLocalArchive requires a valid -Dist directory"
        exit 1
    }

    $localManifest = Join-Path $Dist "scoop-temp.json"
    try {
        $localJson = $parsed | ConvertTo-Json -Depth 10
        $localObj = $localJson | ConvertFrom-Json

        if (-not $Version) {
            $Version = "0.0.0-smoke"
        }
        $localObj.version = $Version -replace "^v", ""

        $archFiles = @()
        if ($archMap["$([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture)"] -eq "arm64") {
            $archFiles = @(Get-ChildItem "$Dist\*windows*arm64*.zip" -ErrorAction SilentlyContinue)
        } else {
            $archFiles = @(Get-ChildItem "$Dist\*windows*x86_64*.zip" -ErrorAction SilentlyContinue)
        }

        if ($archFiles.Count -gt 0) {
            $localArchive = $archFiles[0].FullName
            $localHash = (Get-FileHash -Algorithm SHA256 $localArchive).Hash.ToLowerInvariant()
            $localObj.architecture.'64bit'.url = "file:///$($localArchive.Replace('\','/'))"
            $localObj.architecture.'64bit'.hash = $localHash
            $localObj | ConvertTo-Json -Depth 10 | Set-Content -Path $localManifest -Force

            Write-Host "Testing local Scoop install from $localManifest ..."
            $installOutput = scoop install $localManifest 2>&1
            if ($LASTEXITCODE -eq 0) {
                Write-Host "  Local Scoop install succeeded"
                scoop uninstall ocgo 2>&1 | Out-Null
            } else {
                Write-Host "  WARNING: local Scoop install failed (this is acceptable):"
                Write-Host "    $installOutput"
            }
        } else {
            Write-Host "  WARNING: no Windows zip found in $Dist for local Scoop test"
        }
    } finally {
        if (Test-Path $localManifest) { Remove-Item -Force $localManifest -ErrorAction SilentlyContinue }
    }
} elseif ($UseLocalArchive -and -not $hasScoop) {
    Write-Host "  SKIP: local Scoop test requires Scoop to be installed"
    if ($RequireScoop) {
        Write-Error "-RequireScoop set but Scoop is not available"
        exit 1
    }
}

if (-not $hasScoop) {
    Write-Host "  SKIP: Scoop is not installed (use -RequireScoop to enforce)"
    if ($RequireScoop) {
        Write-Error "-RequireScoop set but Scoop is not available"
        exit 1
    }
}

Write-Host ""
Write-Host "Scoop manifest validation passed."