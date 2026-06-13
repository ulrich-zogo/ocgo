param(
    [string]$ManifestDir = "",
    [switch]$RequireWinget,
    [switch]$AllowKnownRunnerWarnings
)

$ErrorActionPreference = "Stop"

if (-not $ManifestDir) {
    $ManifestDir = Join-Path $PSScriptRoot "..\packaging\winget\manifests\u\UlrichZogo\OCGO\0.1.0"
}
$ManifestDir = Resolve-Path -LiteralPath $ManifestDir -ErrorAction Stop

Write-Host "Validating WinGet manifests in: $ManifestDir"

$versionYaml = Join-Path $ManifestDir "UlrichZogo.OCGO.yaml"
$installerYaml = Join-Path $ManifestDir "UlrichZogo.OCGO.installer.yaml"
$localeYaml = Join-Path $ManifestDir "UlrichZogo.OCGO.locale.en-US.yaml"

$expectedFiles = @($versionYaml, $installerYaml, $localeYaml)
foreach ($f in $expectedFiles) {
    if (-not (Test-Path $f)) {
        Write-Error "ERROR: missing required manifest file: $f"
        exit 1
    }
    Write-Host "  Found: $(Split-Path -Leaf $f)"
}

function Check-Field($path, $field, $fileDesc) {
    $content = Get-Content $path -Raw
    $match = [regex]::Match($content, "$field\s*:\s*(.+)")
    if (-not $match.Success) {
        Write-Error "ERROR: $field not found in $fileDesc"
        return $null
    }
    return $match.Groups[1].Value.Trim()
}

function Check-NoEmanuelCasco($path, $fileDesc) {
    $content = Get-Content $path -Raw
    if ($content.ToLowerInvariant() -match "emanuelcasco") {
        Write-Error "ERROR: $fileDesc contains reference to emanuelcasco"
        exit 1
    }
}

Write-Host ""
Write-Host "Checking UlrichZogo.OCGO.yaml ..."
$id = Check-Field $versionYaml "PackageIdentifier" "version manifest"
if ($id -ne "UlrichZogo.OCGO") {
    Write-Error "ERROR: PackageIdentifier should be UlrichZogo.OCGO, got: $id"
    exit 1
}
Write-Host "  PackageIdentifier: $id"

$verVersion = Check-Field $versionYaml "PackageVersion" "version manifest"
if (-not $verVersion) { exit 1 }
Write-Host "  PackageVersion: $verVersion"

$manifestType = Check-Field $versionYaml "ManifestType" "version manifest"
if ($manifestType -ne "version") {
    Write-Error "ERROR: version manifest ManifestType should be 'version', got: $manifestType"
    exit 1
}
Check-NoEmanuelCasco $versionYaml "version manifest"

Write-Host ""
Write-Host "Checking UlrichZogo.OCGO.installer.yaml ..."
$instId = Check-Field $installerYaml "PackageIdentifier" "installer manifest"
if ($instId -ne "UlrichZogo.OCGO") {
    Write-Error "ERROR: installer PackageIdentifier should be UlrichZogo.OCGO, got: $instId"
    exit 1
}

$instVersion = Check-Field $installerYaml "PackageVersion" "installer manifest"
if ($instVersion -ne $verVersion) {
    Write-Error "ERROR: installer PackageVersion ($instVersion) differs from version manifest ($verVersion)"
    exit 1
}
Write-Host "  PackageVersion matches: $instVersion"

$installerContent = Get-Content $installerYaml -Raw
if ($installerContent -notmatch "InstallerType:\s*zip") {
    Write-Error "ERROR: installer manifest should use InstallerType: zip"
    exit 1
}
Write-Host "  InstallerType: zip"

$x64Match = [regex]::Match($installerContent, "- Architecture:\s*x64")
if (-not $x64Match.Success) {
    Write-Error "ERROR: installer manifest missing x64 architecture entry"
    exit 1
}

$urlMatches = [regex]::Matches($installerContent, "InstallerUrl:\s*(\S+)")
if ($urlMatches.Count -eq 0) {
    Write-Error "ERROR: no InstallerUrl entries found"
    exit 1
}
foreach ($urlMatch in $urlMatches) {
    $url = $urlMatch.Groups[1].Value
    if ($url -notmatch "ulrich-zogo/ocgo") {
        Write-Error "ERROR: InstallerUrl must point to ulrich-zogo/ocgo, got: $url"
        exit 1
    }
    Write-Host "  InstallerUrl: $url"
}

$shaMatches = [regex]::Matches($installerContent, "InstallerSha256:\s*(\S+)")
if ($shaMatches.Count -eq 0) {
    Write-Error "ERROR: no InstallerSha256 entries found"
    exit 1
}
foreach ($shaMatch in $shaMatches) {
    $sha = $shaMatch.Groups[1].Value
    if ($sha -match "^(0+|[[:space:]]*)$") {
        Write-Error "ERROR: InstallerSha256 is empty or zero"
        exit 1
    }
    Write-Host "  InstallerSha256: $sha"
}

$nestedMatches = [regex]::Matches($installerContent, "RelativeFilePath:\s*(\S+)")
if ($nestedMatches.Count -eq 0) {
    Write-Error "ERROR: no NestedInstallerFiles.RelativeFilePath entries found"
    exit 1
}
foreach ($nestedMatch in $nestedMatches) {
    $relPath = $nestedMatch.Groups[1].Value
    Write-Host "  NestedInstallerFile: $relPath"
}

Check-NoEmanuelCasco $installerYaml "installer manifest"

Write-Host ""
Write-Host "Checking UlrichZogo.OCGO.locale.en-US.yaml ..."
$locId = Check-Field $localeYaml "PackageIdentifier" "locale manifest"
if ($locId -ne "UlrichZogo.OCGO") {
    Write-Error "ERROR: locale PackageIdentifier should be UlrichZogo.OCGO, got: $locId"
    exit 1
}

$locVersion = Check-Field $localeYaml "PackageVersion" "locale manifest"
if ($locVersion -ne $verVersion) {
    Write-Error "ERROR: locale PackageVersion ($locVersion) differs from version manifest ($verVersion)"
    exit 1
}
Write-Host "  PackageVersion matches: $locVersion"

$publisher = Check-Field $localeYaml "Publisher" "locale manifest"
Write-Host "  Publisher: $publisher"

$publisherUrl = Check-Field $localeYaml "PublisherUrl" "locale manifest"
if ($publisherUrl -notmatch "ulrich-zogo") {
    Write-Error "ERROR: PublisherUrl should point to ulrich-zogo, got: $publisherUrl"
    exit 1
}
Write-Host "  PublisherUrl: $publisherUrl"

$packageUrl = Check-Field $localeYaml "PackageUrl" "locale manifest"
if ($packageUrl -notmatch "ulrich-zogo/ocgo") {
    Write-Error "ERROR: PackageUrl should point to ulrich-zogo/ocgo, got: $packageUrl"
    exit 1
}
Write-Host "  PackageUrl: $packageUrl"

Check-NoEmanuelCasco $localeYaml "locale manifest"

$hasWinget = Get-Command winget -ErrorAction SilentlyContinue

if ($hasWinget) {
    Write-Host ""
    Write-Host "Running winget validate ..."
    $out = winget validate $ManifestDir 2>&1
    $text = ($out | Out-String)
    Write-Host $text
    $ec = $LASTEXITCODE

    if ($ec -eq 0) {
        Write-Host "winget validate completed."
    } else {
        $hasKnownSchemaWarning = $text -match "Schema header not found"
        $hasHardError = $text -match "(?i)\berror\b" -or $text -match "(?i)\bfailed\b" -or $text -match "(?i)\binvalid\b" -or $text -match "erreur" -or $text -match "échec"

        if ($AllowKnownRunnerWarnings -and $hasKnownSchemaWarning -and -not $hasHardError) {
            Write-Host "winget validate returned non-zero but only known schema-header warning was detected. Accepting with -AllowKnownRunnerWarnings."
        } else {
            Write-Error "winget validate failed with exit code $ec."
            Write-Error $text
            exit $ec
        }
    }
} else {
    Write-Host ""
    Write-Host "SKIP: winget is not installed (use -RequireWinget to enforce)"
    if ($RequireWinget) {
        Write-Error "-RequireWinget set but winget is not available"
        exit 1
    }
}

Write-Host ""
Write-Host "WinGet manifest validation passed."