#!/usr/bin/env bash
set -euo pipefail

STRICT=false
if [[ "${1:-}" == "--strict" ]]; then
  STRICT=true
fi

CANONICAL_OWNER="ulrich-zogo"
CANONICAL_REPO="${CANONICAL_OWNER}/ocgo"

# Build the forbidden string from parts so this script never
# literally contains the upstream owner name.
a="emanue"
b="lcasco"
FORBIDDEN="${a}${b}"

ERRORS=0
WARNINGS=0

check_git_remote() {
  local origin_url
  origin_url="$(git config --get remote.origin.url 2>/dev/null || true)"

  if [[ -z "$origin_url" ]]; then
    echo "ERROR: no git remote 'origin' configured" >&2
    ERRORS=$((ERRORS + 1))
    return
  fi

  # Accept both HTTPS and SSH forms of the canonical owner.
  local normalized
  normalized="$(echo "$origin_url" | sed 's|^git@github.com:|https://github.com/|' | sed 's|\.git$||')"

  if ! echo "$normalized" | grep -q "${CANONICAL_OWNER}/ocgo"; then
    echo "ERROR: git remote 'origin' does not point to ${CANONICAL_REPO}" >&2
    echo "  Got: $origin_url" >&2
    ERRORS=$((ERRORS + 1))
  else
    echo "OK: git remote origin -> ${CANONICAL_REPO}"
  fi

  # Check for upstream remote pointing to the old owner.
  local upstream_url
  upstream_url="$(git config --get remote.upstream.url 2>/dev/null || true)"
  if [[ -n "$upstream_url" ]]; then
    local upstream_normalized
    upstream_normalized="$(echo "$upstream_url" | sed 's|^git@github.com:|https://github.com/|' | sed 's|\.git$||')"
    if echo "$upstream_normalized" | grep -qi "$FORBIDDEN"; then
      echo "WARNING: git remote 'upstream' points to the original upstream repository" >&2
      echo "  Got: $upstream_url" >&2
      WARNINGS=$((WARNINGS + 1))
      if $STRICT; then
        ERRORS=$((ERRORS + 1))
      fi
    fi
  fi
}

check_files_for_old_owner() {
  local matches
  matches=$(find . -type f \
    -not -path './.git/*' \
    -not -path '*/vendor/*' \
    -not -path '*/node_modules/*' \
    -not -path '*/dist/*' \
    -not -name "$(basename "$0")" \
    -exec grep -Il "$FORBIDDEN" {} + 2>/dev/null || true)

  if [[ -n "$matches" ]]; then
    echo "ERROR: references to original upstream repository owner found in:" >&2
    echo "$matches" >&2
    ERRORS=$((ERRORS + 1))
  else
    echo "OK: no references to original upstream repository owner in source files"
  fi
}

check_readme_owner() {
  if [[ -f "README.md" ]]; then
    if grep -q "$FORBIDDEN" README.md 2>/dev/null; then
      echo "ERROR: README.md contains reference to original upstream owner" >&2
      ERRORS=$((ERRORS + 1))
    else
      echo "OK: README.md does not reference original upstream owner"
    fi
  fi
}

echo "Repository ownership check: ${CANONICAL_REPO}"
echo ""

check_git_remote
echo ""
check_readme_owner
echo ""
check_files_for_old_owner

echo ""
if [[ $ERRORS -gt 0 ]]; then
  echo "FAILED: ${ERRORS} error(s) found." >&2
  exit 1
fi
if [[ $WARNINGS -gt 0 ]]; then
  echo "PASSED with ${WARNINGS} warning(s)."
else
  echo "PASSED."
fi
