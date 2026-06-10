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

## Build artifacts locally (without publishing)

```bash
TAG=v0.2.0 make build-release
```

## Update Homebrew formula locally

```bash
TAG=v0.2.0 HOMEBREW_TAP_REPO=ulrich-zogo/homebrew-tap make update-homebrew-formula
```

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
