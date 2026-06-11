#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

APP_NAME="${APP_NAME:-ocgo}"
CMD_PATH="${CMD_PATH:-./cmd/ocgo}"
TAG="${1:-${TAG:-}}"
VERSION="${TAG#v}"
COMMIT="${GITHUB_SHA:-$(git rev-parse HEAD 2>/dev/null || echo unknown)}"
DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
DIST_DIR="${DIST_DIR:-dist}"

LDFLAGS="-s -w -X main.version=$VERSION -X ocgo/internal/buildinfo.Version=$VERSION -X ocgo/internal/buildinfo.Commit=$COMMIT -X ocgo/internal/buildinfo.Date=$DATE"

if [[ -z "$TAG" ]]; then
  echo "Usage: $0 v0.1.0"
  echo "   or: TAG=v0.1.0 make build-release"
  exit 1
fi

if [[ -z "$VERSION" || "$VERSION" == "$TAG" ]]; then
  echo "Invalid tag format: $TAG (must start with 'v')"
  exit 1
fi

rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"

build_one() {
  local goos="$1"
  local goarch="$2"
  local arch_name="$goarch"

  if [[ "$goarch" == "amd64" ]]; then
    arch_name="x86_64"
  fi

  local dir="$DIST_DIR/${APP_NAME}_${VERSION}_${goos}_${arch_name}"
  mkdir -p "$dir"

  local bin="$APP_NAME"
  if [[ "$goos" == "windows" ]]; then
    bin="$APP_NAME.exe"
  fi

  echo "Building $goos/$goarch..."
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath -ldflags "$LDFLAGS" -o "$dir/$bin" "$CMD_PATH"

  cp README.md "$dir/" 2>/dev/null || true
  cp LICENSE "$dir/" 2>/dev/null || true

  if [[ "$goos" == "windows" ]]; then
    (cd "$DIST_DIR" && zip -qr "${APP_NAME}_${VERSION}_${goos}_${arch_name}.zip" "${APP_NAME}_${VERSION}_${goos}_${arch_name}")
  else
    tar -C "$DIST_DIR" -czf "${dir}.tar.gz" "$(basename "$dir")"
  fi

  rm -rf "$dir"
}

build_one darwin amd64
build_one darwin arm64
build_one linux amd64
build_one linux arm64
build_one windows amd64
build_one windows arm64

(
  cd "$DIST_DIR"
  shasum -a 256 *.tar.gz *.zip 2>/dev/null > checksums.txt
)

echo "Release artifacts built in $DIST_DIR:"
ls -la "$DIST_DIR"

# Verify artifacts immediately.
"$SCRIPT_DIR/verify-release-artifacts.sh" "$TAG" "$DIST_DIR"
