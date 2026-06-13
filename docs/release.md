# Release process

## Automated release

Create and push a tag:

```bash
git tag -a v0.2.0 -m v0.2.0
git push origin v0.2.0
```

This triggers `.github/workflows/release.yml`.

The workflow:

1. validates the code;
2. builds macOS, Linux, and Windows artifacts;
3. generates checksums;
4. creates/updates the GitHub Release;
5. updates `ulrich-zogo/homebrew-tap`.

## Required secrets

The `update-homebrew` job requires a GitHub secret:

```
HOMEBREW_TAP_TOKEN
```

The token must have write access to:

```
ulrich-zogo/homebrew-tap
```

If the secret is not set, the release workflow fails before updating the tap.

## Manual dispatch

The workflow can also be run manually from the Actions tab:

```
Actions → release → Run workflow → tag = v0.2.0
```

## Local release

```bash
TAG=v0.2.0 make release
```

Requires `gh` (GitHub CLI) authenticated and Go installed.

## Pre-release install smoke

Before publishing a release, build local artifacts and run the install smoke test:

```bash
scripts/build-release-artifacts.sh vX.Y.Z
scripts/smoke-release-install.sh --dist dist --version vX.Y.Z
```

On Windows, validate the Windows archive and installer:

```powershell
.\scripts\smoke-release-install.ps1 -Dist .\dist -Version "vX.Y.Z"
```

This validates checksums, archive extraction, binary execution, JSON diagnostic output, and the official model list.

## Build artifacts locally (without publishing)

```bash
TAG=v0.2.0 make build-release
```

## Update Homebrew formula locally

```bash
TAG=v0.2.0 HOMEBREW_TAP_REPO=ulrich-zogo/homebrew-tap make update-homebrew-formula
```

## Build metadata

Release binaries are built with embedded metadata:

- version
- commit
- build date

Verify a release binary with:

```bash
ocgo version
ocgo version --json
```

## Post-release smoke test

After publishing a release, run:

```bash
gh workflow run release-smoke.yml --repo ulrich-zogo/ocgo -f tag=v0.1.0
gh run watch --repo ulrich-zogo/ocgo
```

This validates:

- release assets;
- checksums;
- archive structure;
- Homebrew formula;
- Homebrew installation on macOS.

## Rollback

See [release rollback](release-rollback.md).

## Artifacts

```
ocgo_<version>_darwin_arm64.tar.gz
ocgo_<version>_darwin_x86_64.tar.gz
ocgo_<version>_linux_arm64.tar.gz
ocgo_<version>_linux_x86_64.tar.gz
ocgo_<version>_windows_arm64.zip
ocgo_<version>_windows_x86_64.zip
checksums.txt
```

## Windows packaging

Windows archives are consumed by:

- `scripts/install-windows.ps1`
- `packaging/scoop/ocgo.json`
- `packaging/winget/manifests/u/UlrichZogo/OCGO/<version>/`

The expected Windows assets are:

```text
ocgo_<version>_windows_x86_64.zip
ocgo_<version>_windows_arm64.zip
checksums.txt
```

Scoop and WinGet manifests must be updated when a new release is published. Automation for external publishing can be added in a future PR.
