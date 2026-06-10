#!/usr/bin/env bash
set -euo pipefail

patterns=(
  "emanuelcasco"
  "emanuelcasco/tap"
  "github.com/emanuelcasco"
)

found=false
for pattern in "${patterns[@]}"; do
  if grep -RIn \
    --exclude-dir=.git \
    --exclude-dir=vendor \
    --exclude-dir=node_modules \
    --exclude-dir=dist \
    --exclude="${0##*/}" \
    "$pattern" . >&2; then
    found=true
  fi
done

if $found; then
  echo "Found references to original upstream repository owner. Replace them with ulrich-zogo." >&2
  exit 1
fi

echo "No reference to original upstream repository owner found. OK."
