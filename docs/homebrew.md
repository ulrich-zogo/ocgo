# Homebrew tap

OCGO uses a dedicated Homebrew tap under the same GitHub owner as this fork.

Expected tap repository:

```
ulrich-zogo/homebrew-tap
```

## Install

```bash
brew tap ulrich-zogo/tap
brew install ocgo
```

Direct install:

```bash
brew install ulrich-zogo/tap/ocgo
```

## Create the tap repository

Create a new GitHub repository named `ulrich-zogo/homebrew-tap` with the following structure:

```
Formula/
  ocgo.rb
```

## Release source

The formula must download release artifacts from:

```
https://github.com/ulrich-zogo/ocgo/releases
```

Do not use the original upstream repository.

## Release script integration

The OCGO release script publishes formula updates to `ulrich-zogo/homebrew-tap` using the variable:

```bash
HOMEBREW_TAP_REPO=ulrich-zogo/homebrew-tap
```

## Automated updates

The release workflow updates this tap automatically when a `v*` tag is pushed.

Required secret in `ulrich-zogo/ocgo`:

```text
HOMEBREW_TAP_TOKEN
```

The token must have write access to `ulrich-zogo/homebrew-tap`.

See [docs/release.md](release.md) for the full release process.

## Manual validation

```bash
brew tap ulrich-zogo/tap https://github.com/ulrich-zogo/homebrew-tap
brew install ocgo
ocgo --help
```
