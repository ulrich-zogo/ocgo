#!/usr/bin/env bash
set -euo pipefail

APP_NAME="${APP_NAME:-ocgo}"
REPO="${GITHUB_REPOSITORY:-ulrich-zogo/ocgo}"
TAG="${1:-${TAG:-}}"
FORMULA_PATH="${2:-${FORMULA_PATH:-Formula/ocgo.rb}}"

if [[ -z "$TAG" ]]; then
  echo "Usage: $0 v0.1.0 [Formula/ocgo.rb]" >&2
  exit 1
fi

VERSION="${TAG#v}"

if [[ -z "$VERSION" || "$VERSION" == "$TAG" ]]; then
  echo "Invalid tag format: $TAG (must start with 'v')" >&2
  exit 1
fi

if [[ ! -f "$FORMULA_PATH" ]]; then
  echo "Formula not found: $FORMULA_PATH" >&2
  exit 1
fi

assert_contains() {
  local needle="$1"
  if ! grep -Fq "$needle" "$FORMULA_PATH"; then
    echo "Formula is missing expected content: $needle" >&2
    exit 1
  fi
}

assert_not_contains() {
  local needle="$1"
  if grep -Fq "$needle" "$FORMULA_PATH"; then
    echo "Formula contains forbidden content: $needle" >&2
    exit 1
  fi
}

assert_contains "class Ocgo < Formula"
assert_contains "homepage \"https://github.com/${REPO}\""
assert_contains "version \"${VERSION}\""
assert_contains "https://github.com/${REPO}/releases/download/${TAG}/${APP_NAME}_${VERSION}_darwin_arm64.tar.gz"
assert_contains "https://github.com/${REPO}/releases/download/${TAG}/${APP_NAME}_${VERSION}_darwin_x86_64.tar.gz"
assert_contains "sha256"

sha_count="$(grep -c "sha256" "$FORMULA_PATH" | tr -d ' ')"
if [[ "$sha_count" -lt 2 ]]; then
  echo "Expected at least 2 sha256 entries, got $sha_count" >&2
  exit 1
fi

a="emanue"
b="lcasco"
old_owner="${a}${b}"
assert_not_contains "$old_owner"
assert_not_contains "github.com/${old_owner}"
assert_not_contains "${old_owner}/tap"

echo "Homebrew formula verified successfully for $TAG."
