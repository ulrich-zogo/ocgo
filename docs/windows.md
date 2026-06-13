# Windows installation

OCGO supports three Windows installation paths:

1. PowerShell installer
2. Scoop
3. WinGet

## PowerShell installer

Install the latest OCGO release:

```powershell
irm https://raw.githubusercontent.com/ulrich-zogo/ocgo/main/scripts/install-windows.ps1 | iex
```

Or run explicitly:

```powershell
powershell -ExecutionPolicy Bypass -File .\scripts\install-windows.ps1
```

## Install a specific version

```powershell
.\scripts\install-windows.ps1 -Version v0.1.0
```

## Install directory

Default:

```text
%LOCALAPPDATA%\ocgo\bin
```

Custom:

```powershell
.\scripts\install-windows.ps1 -InstallDir "$env:USERPROFILE\Tools\ocgo"
```

## PATH

The installer adds the install directory to the user PATH by default.

Use:

```powershell
.\scripts\install-windows.ps1 -NoPath
```

to skip PATH modification.

## Test the installer from a local archive

The PowerShell installer supports `-ArchivePath` to install from a local zip file without downloading from GitHub:

```powershell
.\scripts\install-windows.ps1 `
  -ArchivePath .\dist\ocgo_0.1.0_windows_x86_64.zip `
  -InstallDir $env:TEMP\ocgo-install-smoke `
  -NoPath `
  -Force
```

With `-DryRun`, the installer shows what it would do without modifying the system:

```powershell
.\scripts\install-windows.ps1 `
  -ArchivePath .\dist\ocgo_0.1.0_windows_x86_64.zip `
  -InstallDir $env:TEMP\ocgo-install-smoke `
  -NoPath `
  -DryRun
```

For a full Windows install smoke that also validates checksums, Scoop, and WinGet manifests:

```powershell
.\scripts\smoke-release-install.ps1 -Dist .\dist -Version "v0.1.0"
```

## Scoop

Future Scoop install path:

```powershell
scoop bucket add ocgo https://github.com/ulrich-zogo/scoop-ocgo
scoop install ocgo
```

Current manifest location:

```text
packaging/scoop/ocgo.json
```

Local test:

```powershell
scoop install .\packaging\scoop\ocgo.json
ocgo --help
scoop uninstall ocgo
```

## WinGet

Future WinGet install path:

```powershell
winget install UlrichZogo.OCGO
```

Current draft manifests:

```text
packaging/winget/manifests/u/UlrichZogo/OCGO/0.1.0/
```

Local validation:

```powershell
winget validate .\packaging\winget\manifests\u\UlrichZogo\OCGO\0.1.0
```

Or validate the manifests with a content check:

```powershell
Get-ChildItem .\packaging\winget\manifests\u\UlrichZogo\OCGO\0.1.0 -Filter *.yaml | ForEach-Object { Write-Host "$($_.Name): $(Get-Content $_.FullName -Raw | Select-String -Pattern 'PackageIdentifier')" }
```

## Verify installation

```powershell
ocgo --help
ocgo version
ocgo version --json
ocgo models
ocgo doctor
ocgo support bundle --output "$env:TEMP\ocgo-support.zip" --force
```

## Setup

```powershell
ocgo setup
ocgo opencode model set-default minimax-m3
ocgo daemon start
ocgo doctor
```

## Uninstall PowerShell installation

Remove:

```text
%LOCALAPPDATA%\ocgo\bin\ocgo.exe
```

Optionally remove the PATH entry manually from:

```text
System Properties → Environment Variables → User variables → Path
```

This does not remove OCGO configuration files.

## Reset Windows configuration

```powershell
ocgo config paths
ocgo config inspect
ocgo config backup
ocgo config reset --scope ocgo --dry-run
```

## Configuration files

OCGO configuration is stored under:

```text
%USERPROFILE%\.config\ocgo
%USERPROFILE%\.codex
```

## JSON diagnostics in PowerShell

JSON output can be piped to `ConvertFrom-Json` for scripting:

```powershell
ocgo version --json | ConvertFrom-Json
ocgo daemon status --json | ConvertFrom-Json
ocgo config inspect --json | ConvertFrom-Json
ocgo support bundle --json | ConvertFrom-Json
```

## Troubleshooting

If `ocgo` is not found after installation:

1. Open a new terminal.
2. Check:

```powershell
$env:Path
```

3. Run directly:

```powershell
& "$env:LOCALAPPDATA\ocgo\bin\ocgo.exe" --help
```

## Security

The PowerShell installer, Scoop manifest, and WinGet manifests download release assets from:

```text
https://github.com/ulrich-zogo/ocgo/releases
```

Release checksums are verified before installation where supported.
