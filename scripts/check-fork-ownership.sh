#!/usr/bin/env bash
set -euo pipefail

# Each forbidden string is built from parts so this script
# never literally contains the upstream owner name.
a="emanue"
b="lcasco"
owner="${a}${b}"

patterns=(
  "$owner"
  "$owner/tap"
  "github.com/$owner"
)

exclude_file="check-fork-ownership.sh"

found=false
for pattern in "${patterns[@]}"; do
  matches=$(find . -type f \
    -not -path './.git/*' \
    -not -path '*/vendor/*' \
    -not -path '*/node_modules/*' \
    -not -path '*/dist/*' \
    -not -name "$exclude_file" \
    -exec grep -In "$pattern" {} + 2>/dev/null || true)
  if [[ -n "$matches" ]]; then
    echo "$matches" >&2
    found=true
  fi
done

if $found; then
  echo "Found references to original upstream repository owner. Replace them with ulrich-zogo." >&2
  exit 1
fi

echo "No reference to original upstream repository owner found. OK."
