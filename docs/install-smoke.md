# Installation smoke tests

## Purpose

These smoke tests validate OCGO distribution artifacts and package-manager metadata before publishing a release. They work with locally built archives and do not require a published GitHub release.

## What is covered

- Local release archives for the current platform
- Checksum verification against `checksums.txt`
- Archive extraction (tar.gz on Linux/macOS, zip on Windows)
- Archive contents (README.md, LICENSE, binary)
- Binary execution with temporary HOME/USERPROFILE
- `version`, `version --json`, `--help`, `models`, `doctor --json`
- JSON output validation for diagnostic commands
- All 18 official models appearing in `models` output
- Windows installer with local archive (`-ArchivePath`)
- Scoop manifest metadata
- WinGet manifest metadata

## What is not covered

- Publishing a GitHub Release
- Publishing to Homebrew tap
- Publishing to Scoop bucket
- Publishing to WinGet community repository
- Live upstream OpenCode Go API calls
- Real daemon or proxy behavior

## Prerequisites

- Go 1.22+
- Release artifacts built locally (see below)
- `python` available for JSON validation (Linux/macOS)
- `winget` optional for WinGet validation
- `scoop` optional for local Scoop install test

## Build release artifacts locally

```bash
scripts/build-release-artifacts.sh v0.0.0-smoke
```

This creates `dist/` with archives and `checksums.txt`.

## Linux/macOS

```bash
scripts/smoke-release-install.sh --dist dist --version v0.0.0-smoke
```

Options:

- `--dist <dir>` — distribution directory (default: `dist`)
- `--version <tag>` — version tag (auto-detected if omitted)
- `--keep-temp` — keep temporary extraction directory for debugging

## Windows PowerShell

```powershell
.\scripts\smoke-release-install.ps1 -Dist .\dist -Version "v0.0.0-smoke"
```

Options:

- `-Dist <dir>` — distribution directory (default: `dist`)
- `-Version <tag>` — version tag (auto-detected if omitted)
- `-KeepTemp` — keep temporary extraction directory for debugging

## Scoop manifest

```powershell
.\scripts\smoke-scoop-manifest.ps1 -Manifest .\packaging\scoop\ocgo.json -Dist .\dist -Version "v0.0.0-smoke"
```

Options:

- `-Manifest <path>` — path to Scoop manifest (default: `packaging/scoop/ocgo.json`)
- `-Dist <dir>` — distribution directory for local archive test
- `-Version <tag>` — version tag
- `-UseLocalArchive` — attempt local Scoop install test (requires Scoop)
- `-RequireScoop` — fail if Scoop is not available

## WinGet manifest

```powershell
.\scripts\smoke-winget-manifest.ps1 -ManifestDir .\packaging\winget\manifests\u\UlrichZogo\OCGO\0.1.0
```

Options:

- `-ManifestDir <path>` — WinGet manifest directory
- `-RequireWinget` — fail if winget is not available
- `-AllowKnownRunnerWarnings` — accept known schema-header warnings from CI runners

## Makefile targets

```bash
# Run smoke test on existing dist/
make release-install-smoke

# Build artifacts then run smoke test
make release-install-smoke-build
```

## Safety

All smoke tests use temporary directories for installation and HOME/USERPROFILE. They never modify the real user environment, PATH, or configuration files.

Environment variables are restored in `try/finally` blocks in PowerShell scripts.
