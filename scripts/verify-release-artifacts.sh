#!/usr/bin/env bash
set -euo pipefail

APP_NAME="${APP_NAME:-ocgo}"
TAG="${1:-${TAG:-}}"
DIST_DIR="${2:-${DIST_DIR:-dist}}"

if [[ -z "$TAG" ]]; then
  echo "Usage: $0 v0.1.0 [dist]" >&2
  exit 1
fi

VERSION="${TAG#v}"

if [[ -z "$VERSION" || "$VERSION" == "$TAG" ]]; then
  echo "Invalid tag format: $TAG (must start with 'v')" >&2
  exit 1
fi

if [[ ! -d "$DIST_DIR" ]]; then
  echo "Dist directory not found: $DIST_DIR" >&2
  exit 1
fi

expected=(
  "${APP_NAME}_${VERSION}_darwin_arm64.tar.gz"
  "${APP_NAME}_${VERSION}_darwin_x86_64.tar.gz"
  "${APP_NAME}_${VERSION}_linux_arm64.tar.gz"
  "${APP_NAME}_${VERSION}_linux_x86_64.tar.gz"
  "${APP_NAME}_${VERSION}_windows_arm64.zip"
  "${APP_NAME}_${VERSION}_windows_x86_64.zip"
)

echo "Checking for ${#expected[@]} expected artifacts..."

for file in "${expected[@]}"; do
  if [[ ! -f "$DIST_DIR/$file" ]]; then
    echo "Missing release artifact: $DIST_DIR/$file" >&2
    exit 1
  fi
  echo "  Found: $file"
done

if [[ ! -f "$DIST_DIR/checksums.txt" ]]; then
  echo "Missing checksums file: $DIST_DIR/checksums.txt" >&2
  exit 1
fi
echo "  Found: checksums.txt"

for file in "${expected[@]}"; do
  if ! grep -q "  ${file}$" "$DIST_DIR/checksums.txt"; then
    echo "Missing checksum entry for: $file" >&2
    exit 1
  fi
done

checksum_count="$(wc -l < "$DIST_DIR/checksums.txt" | tr -d ' ')"
if [[ "$checksum_count" != "${#expected[@]}" ]]; then
  echo "Expected ${#expected[@]} checksum entries, got $checksum_count" >&2
  exit 1
fi
echo "  checksums.txt entries: $checksum_count (expected ${#expected[@]})"

echo "Verifying checksums..."
(
  cd "$DIST_DIR"
  shasum -a 256 -c checksums.txt
)

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

check_version() {
  local binary="$1"
  if ! "$binary" version > /dev/null 2>&1; then
    echo "Binary $binary failed to run 'version'" >&2
    exit 1
  fi
  if ! "$binary" version --json | grep -q '"version":'; then
    echo "Binary $binary 'version --json' missing version field" >&2
    exit 1
  fi
}

verify_extracted_dir() {
  local root="$1"
  local binary="$2"

  if ! find "$root" -type f -name "$binary" | grep -q .; then
    echo "Missing binary $binary in extracted archive" >&2
    exit 1
  fi

  if ! find "$root" -type f -name "README.md" | grep -q .; then
    echo "Missing README.md in extracted archive" >&2
    exit 1
  fi

  if ! find "$root" -type f -name "LICENSE" | grep -q .; then
    echo "Missing LICENSE in extracted archive" >&2
    exit 1
  fi
}

for file in "${expected[@]}"; do
  target="$TMP_DIR/$file"
  mkdir -p "$target"

  case "$file" in
    *.tar.gz)
      tar -xzf "$DIST_DIR/$file" -C "$target"
      verify_extracted_dir "$target" "$APP_NAME"
      local bin_path="$(find "$target" -type f -name "$APP_NAME" | head -1)"
      if [[ -n "$bin_path" ]]; then
        check_version "$bin_path" || true
      fi
      echo "  Extracted and verified: $file"
      ;;
    *.zip)
      unzip -q "$DIST_DIR/$file" -d "$target"
      verify_extracted_dir "$target" "${APP_NAME}.exe"
      echo "  Extracted and verified: $file"
      ;;
    *)
      echo "Unsupported artifact type: $file" >&2
      exit 1
      ;;
  esac
done

echo "Release artifacts verified successfully for $TAG."
