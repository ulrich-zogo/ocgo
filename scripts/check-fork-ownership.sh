#!/usr/bin/env bash
set -euo pipefail

# Build the forbidden string from parts so this script never
# literally contains the upstream owner name.
a="emanue"
b="lcasco"
forbidden="${a}${b}"

exclude_file="check-fork-ownership.sh"

matches=$(find . -type f \
  -not -path './.git/*' \
  -not -path '*/vendor/*' \
  -not -path '*/node_modules/*' \
  -not -path '*/dist/*' \
  -not -name "$exclude_file" \
  -exec grep -Il "$forbidden" {} + 2>/dev/null || true)

if [[ -n "$matches" ]]; then
  echo "Found references to original upstream repository owner in:" >&2
  echo "$matches" >&2
  exit 1
fi

echo "No reference to original upstream repository owner found. OK."
