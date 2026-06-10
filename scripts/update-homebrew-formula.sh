#!/usr/bin/env bash
set -euo pipefail

APP_NAME="${APP_NAME:-ocgo}"
REPO="${GITHUB_REPOSITORY:-ulrich-zogo/ocgo}"
HOMEBREW_TAP_REPO="${HOMEBREW_TAP_REPO:-ulrich-zogo/homebrew-tap}"
TAG="${1:-${TAG:-}}"
CHECKSUMS_FILE="${2:-dist/checksums.txt}"

VERSION="${TAG#v}"

if [[ -z "$TAG" ]]; then
  echo "Usage: $0 v0.1.0 [dist/checksums.txt]"
  echo "   or: TAG=v0.1.0 make update-homebrew-formula"
  exit 1
fi

if [[ ! -f "$CHECKSUMS_FILE" ]]; then
  echo "Checksums file not found: $CHECKSUMS_FILE"
  echo "Run scripts/build-release-artifacts.sh first."
  exit 1
fi

DARWIN_ARM_SHA="$(grep "darwin_arm64" "$CHECKSUMS_FILE" | awk '{print $1}')"
DARWIN_AMD_SHA="$(grep "darwin_x86_64" "$CHECKSUMS_FILE" | awk '{print $1}')"

if [[ -z "$DARWIN_ARM_SHA" || -z "$DARWIN_AMD_SHA" ]]; then
  echo "Could not find darwin checksums in $CHECKSUMS_FILE"
  exit 1
fi

echo "Cloning $HOMEBREW_TAP_REPO..."
TAP_TMP="$(mktemp -d)"
trap 'rm -rf "$TAP_TMP"' EXIT

if [[ -n "${HOMEBREW_TAP_TOKEN:-}" ]]; then
  AUTH_HEADER="$(printf "x-access-token:%s" "$HOMEBREW_TAP_TOKEN" | base64 | tr -d '\n')"
  git -c "http.https://github.com/.extraheader=AUTHORIZATION: basic ${AUTH_HEADER}" \
    clone "https://github.com/${HOMEBREW_TAP_REPO}.git" "$TAP_TMP" --quiet
elif command -v gh >/dev/null 2>&1; then
  gh repo clone "$HOMEBREW_TAP_REPO" "$TAP_TMP" -- --quiet
else
  git clone "https://github.com/${HOMEBREW_TAP_REPO}.git" "$TAP_TMP" --quiet
fi

mkdir -p "$TAP_TMP/Formula"

cat > "$TAP_TMP/Formula/${APP_NAME}.rb" <<EOF_FORMULA
class Ocgo < Formula
  desc "Use OpenCode Go with Claude Code, Codex CLI, and Codex Desktop"
  homepage "https://github.com/${REPO}"
  version "${VERSION}"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/${REPO}/releases/download/${TAG}/${APP_NAME}_${VERSION}_darwin_arm64.tar.gz"
      sha256 "${DARWIN_ARM_SHA}"
    else
      url "https://github.com/${REPO}/releases/download/${TAG}/${APP_NAME}_${VERSION}_darwin_x86_64.tar.gz"
      sha256 "${DARWIN_AMD_SHA}"
    end
  end

  def install
    bin.install "${APP_NAME}"
  end

  test do
    system "\#{bin}/${APP_NAME}", "--help"
  end
end
EOF_FORMULA

(
  cd "$TAP_TMP"

  git config user.name "github-actions[bot]"
  git config user.email "41898282+github-actions[bot]@users.noreply.github.com"

  git add "Formula/${APP_NAME}.rb"
  if git diff --cached --quiet; then
    echo "Homebrew formula is already up to date."
  else
    git commit -m "Update ${APP_NAME} to ${TAG}"
    if [[ -n "${HOMEBREW_TAP_TOKEN:-}" ]]; then
      AUTH_HEADER="$(printf "x-access-token:%s" "$HOMEBREW_TAP_TOKEN" | base64 | tr -d '\n')"
      git -c "http.https://github.com/.extraheader=AUTHORIZATION: basic ${AUTH_HEADER}" \
        push origin HEAD:main --quiet
    else
      git push
    fi
    echo "Homebrew formula updated."
  fi
)

TAP_OWNER="${HOMEBREW_TAP_REPO%%/*}"
TAP_REPO_NAME="${HOMEBREW_TAP_REPO#*/}"
TAP_NAME="${TAP_REPO_NAME#homebrew-}"

echo "Install with: brew install ${TAP_OWNER}/${TAP_NAME}/${APP_NAME}"
