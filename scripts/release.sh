#!/usr/bin/env bash
set -euo pipefail

APP_NAME="${APP_NAME:-ocgo}"
CMD_PATH="${CMD_PATH:-./cmd/ocgo}"
TAG="${1:-${TAG:-}}"

if [[ -z "$TAG" ]]; then
  echo "Usage: $0 v0.1.0"
  echo "   or: TAG=v0.1.0 make release"
  exit 1
fi

VERSION="${TAG#v}"
REPO="${GITHUB_REPOSITORY:-ulrich-zogo/ocgo}"
if [[ -z "$REPO" ]]; then
  origin_url="$(git config --get remote.origin.url || true)"
  if [[ "$origin_url" =~ github.com[:/]([^/]+)/([^/.]+)(\.git)?$ ]]; then
    REPO="${BASH_REMATCH[1]}/${BASH_REMATCH[2]}"
  else
    echo "Set GITHUB_REPOSITORY=owner/repo, or configure a GitHub origin remote."
    exit 1
  fi
fi

HOMEBREW_TAP_REPO="${HOMEBREW_TAP_REPO:-ulrich-zogo/homebrew-tap}"

if ! command -v gh >/dev/null 2>&1; then
  echo "GitHub CLI is required: brew install gh && gh auth login"
  exit 1
fi

if ! gh auth status >/dev/null 2>&1; then
  echo "GitHub CLI is not authenticated. Run: gh auth login"
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  echo "Go is required."
  exit 1
fi

if ! git diff --quiet || ! git diff --cached --quiet; then
  echo "Working tree has uncommitted changes. Commit or stash them first."
  exit 1
fi

# Verify the project builds/tests before tagging.
go test ./...

if ! git rev-parse "$TAG" >/dev/null 2>&1; then
  git tag -a "$TAG" -m "$TAG"
fi

git push origin "$TAG"

# Build release artifacts using the reusable script.
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
"$SCRIPT_DIR/build-release-artifacts.sh" "$TAG"

# Create or update the GitHub Release.
if gh release view "$TAG" --repo "$REPO" >/dev/null 2>&1; then
  echo "GitHub release $TAG already exists; uploading artifacts with --clobber."
  gh release upload "$TAG" dist/* --repo "$REPO" --clobber
else
  gh release create "$TAG" dist/* \
    --repo "$REPO" \
    --title "$TAG" \
    --generate-notes
fi

# Update Homebrew tap formula.
"$SCRIPT_DIR/update-homebrew-formula.sh" "$TAG" dist/checksums.txt

echo "Release complete: https://github.com/${REPO}/releases/tag/${TAG}"
