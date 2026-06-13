#!/usr/bin/env bash
set -euo pipefail

APP_NAME="${APP_NAME:-ocgo}"
DIST_DIR=""
VERSION=""
KEEP_TEMP=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --dist) DIST_DIR="$2"; shift 2 ;;
    --version) VERSION="$2"; shift 2 ;;
    --keep-temp) KEEP_TEMP="1"; shift ;;
    *) echo "Usage: $0 --dist <dir> [--version <tag>] [--keep-temp]"; exit 1 ;;
  esac
done

if [[ -z "$DIST_DIR" ]]; then
  DIST_DIR="dist"
fi

if [[ ! -d "$DIST_DIR" ]]; then
  echo "ERROR: dist directory not found: $DIST_DIR" >&2
  echo "Run 'scripts/build-release-artifacts.sh v0.0.0-smoke' first." >&2
  exit 1
fi

OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
  Darwin) OS_LOOKUP="darwin" ;;
  Linux)  OS_LOOKUP="linux" ;;
  *)      echo "ERROR: unsupported OS: $OS" >&2; exit 1 ;;
esac

case "$ARCH" in
  x86_64|amd64) ARCH_LOOKUP="x86_64" ;;
  arm64|aarch64) ARCH_LOOKUP="arm64" ;;
  *) echo "ERROR: unsupported architecture: $ARCH" >&2; exit 1 ;;
esac

echo "Platform: ${OS_LOOKUP}/${ARCH_LOOKUP}"

ARCHIVE_PATTERNS=()
if [[ "$OS_LOOKUP" == "darwin" || "$OS_LOOKUP" == "linux" ]]; then
  EXT="tar.gz"
  for os_var in "$OS_LOOKUP" "$(echo "$OS_LOOKUP" | sed 's/.*/\u&/')"; do
    for arch_var in "$ARCH_LOOKUP" "amd64" "ARM64"; do
      if [[ -n "$VERSION" ]]; then
        ARCHIVE_PATTERNS+=("${APP_NAME}_${VERSION#v}_${os_var}_${arch_var}.${EXT}")
      else
        ARCHIVE_PATTERNS+=("*${os_var}*${arch_var}*.${EXT}")
      fi
    done
  done
fi

MATCHES=()
for pattern in "${ARCHIVE_PATTERNS[@]}"; do
  for f in "$DIST_DIR"/$pattern; do
    if [[ -f "$f" ]]; then
      MATCHES+=("$f")
    fi
  done
done

if [[ ${#MATCHES[@]} -eq 0 ]]; then
  echo "ERROR: no archive found for ${OS_LOOKUP}/${ARCH_LOOKUP} in $DIST_DIR" >&2
  echo "Searched patterns:" >&2
  for p in "${ARCHIVE_PATTERNS[@]}"; do echo "  $p" >&2; done
  echo "Available files:" >&2
  ls -1 "$DIST_DIR" 2>/dev/null || true
  exit 1
fi

UNIQ=()
for m in "${MATCHES[@]}"; do
  already=0
  for u in "${UNIQ[@]}"; do
    if [[ "$u" == "$m" ]]; then already=1; break; fi
  done
  if [[ $already -eq 0 ]]; then
    UNIQ+=("$m")
  fi
done
MATCHES=("${UNIQ[@]}")

if [[ ${#MATCHES[@]} -gt 1 ]]; then
  echo "ERROR: multiple archives matched for ${OS_LOOKUP}/${ARCH_LOOKUP}" >&2
  for m in "${MATCHES[@]}"; do echo "  $m" >&2; done
  exit 1
fi

ARCHIVE="${MATCHES[0]}"
ARCHIVE_NAME="$(basename "$ARCHIVE")"
echo "Found archive: $ARCHIVE_NAME"

if [[ ! -f "$DIST_DIR/checksums.txt" ]]; then
  echo "ERROR: checksums.txt not found in $DIST_DIR" >&2
  exit 1
fi

echo "Verifying checksums ..."
(cd "$DIST_DIR" && shasum -a 256 -c checksums.txt)

TMP_DIR="$(mktemp -d)"
if [[ -z "$KEEP_TEMP" ]]; then
  trap 'rm -rf "$TMP_DIR"' EXIT
else
  echo "Temp dir: $TMP_DIR"
fi

EXTRACT_DIR="$TMP_DIR/extract"
mkdir -p "$EXTRACT_DIR"

if [[ "$ARCHIVE" == *.tar.gz ]]; then
  tar -xzf "$ARCHIVE" -C "$EXTRACT_DIR"
else
  echo "ERROR: unsupported archive format: $ARCHIVE" >&2
  exit 1
fi

echo "Checking archive contents ..."
if ! find "$EXTRACT_DIR" -type f -name "README.md" | grep -q .; then
  echo "ERROR: README.md not found in archive" >&2
  exit 1
fi
echo "  README.md found"

if ! find "$EXTRACT_DIR" -type f -name "LICENSE" | grep -q .; then
  echo "ERROR: LICENSE not found in archive" >&2
  exit 1
fi
echo "  LICENSE found"

BIN=""
for candidate in "$APP_NAME" "$APP_NAME.exe"; do
  found=$(find "$EXTRACT_DIR" -type f -name "$candidate" | head -1)
  if [[ -n "$found" ]]; then
    BIN="$found"
    break
  fi
done

if [[ -z "$BIN" ]]; then
  echo "ERROR: $APP_NAME binary not found in archive" >&2
  exit 1
fi
echo "  Binary found: $BIN"

chmod +x "$BIN"

TMP_HOME="$TMP_DIR/home"
mkdir -p "$TMP_HOME"

validate_json() {
  local input="$1"
  local desc="$2"
  if ! echo "$input" | python -m json.tool >/dev/null 2>&1; then
    echo "ERROR: $desc is not valid JSON" >&2
    echo "Output:" >&2
    echo "$input" >&2
    exit 1
  fi
  echo "  $desc: valid JSON"
}

echo ""
echo "=== smoke: version ==="
OUTPUT="$(HOME="$TMP_HOME" OCGO_API_KEY="test-key" "$BIN" version 2>&1)"
if [[ $? -ne 0 ]]; then
  echo "ERROR: ocgo version failed" >&2
  echo "$OUTPUT" >&2
  exit 1
fi
echo "  OK"

echo ""
echo "=== smoke: version --json ==="
JSON_OUTPUT="$(HOME="$TMP_HOME" OCGO_API_KEY="test-key" "$BIN" version --json 2>&1)"
if [[ $? -ne 0 ]]; then
  echo "ERROR: ocgo version --json failed" >&2
  echo "$JSON_OUTPUT" >&2
  exit 1
fi
validate_json "$JSON_OUTPUT" "version --json"

echo ""
echo "=== smoke: --help ==="
HELP_OUTPUT="$(HOME="$TMP_HOME" OCGO_API_KEY="test-key" "$BIN" --help 2>&1)"
echo "  OK"

echo ""
echo "=== smoke: models ==="
MODELS_OUTPUT="$(HOME="$TMP_HOME" OCGO_API_KEY="test-key" "$BIN" models 2>&1)"
if [[ $? -ne 0 ]]; then
  echo "ERROR: ocgo models failed" >&2
  echo "$MODELS_OUTPUT" >&2
  exit 1
fi

OFFICIAL_MODELS=(
  "minimax-m3"
  "minimax-m2.7"
  "minimax-m2.5"
  "kimi-k2.6"
  "kimi-k2.5"
  "glm-5.1"
  "glm-5"
  "deepseek-v4-pro"
  "deepseek-v4-flash"
  "qwen3.7-max"
  "qwen3.7-plus"
  "qwen3.6-plus"
  "qwen3.5-plus"
  "mimo-v2-pro"
  "mimo-v2-omni"
  "mimo-v2.5-pro"
  "mimo-v2.5"
  "hy3-preview"
)

missing=0
for model in "${OFFICIAL_MODELS[@]}"; do
  if ! echo "$MODELS_OUTPUT" | grep -q "$model"; then
    echo "ERROR: model $model not found in models output" >&2
    missing=$((missing + 1))
  fi
done
if [[ $missing -gt 0 ]]; then
  echo "ERROR: $missing official model(s) missing" >&2
  exit 1
fi
echo "  All 18 official models present"

echo ""
echo "=== smoke: doctor --json ==="
set +e
DOCTOR_OUTPUT="$(HOME="$TMP_HOME" OCGO_API_KEY="test-key" "$BIN" doctor --json 2>/dev/null)"
DOCTOR_STATUS=$?
set -e
validate_json "$DOCTOR_OUTPUT" "doctor --json"
echo "  doctor exit code: $DOCTOR_STATUS (non-zero is acceptable)"

echo ""
echo "All smoke tests passed for $ARCHIVE_NAME."