#!/usr/bin/env bash
set -euo pipefail

if grep -RIn \
  --exclude-dir=.git \
  --exclude-dir=vendor \
  --exclude-dir=node_modules \
  --exclude="${0##*/}" \
  "emanuelcasco" .; then
  echo "Found references to original upstream repository owner. Replace them with ulrich-zogo." >&2
  exit 1
fi

echo "No reference to original upstream repository owner found. OK."
