#!/bin/sh
# Writes checksums.txt for every file in a release directory and, when
# MESA_UPDATE_SIGNING_KEY holds an Ed25519 private key (PEM), signs it as
# checksums.txt.sig. The desktop updater installs nothing without that
# signature.
#   packaging/checksums.sh <dir>
set -eu
dir=$1
cd "$dir"
rm -f checksums.txt checksums.txt.sig
find . -maxdepth 1 -type f ! -name 'checksums.txt*' -printf '%f\n' | LC_ALL=C sort | xargs sha256sum > checksums.txt
if [ -n "${MESA_UPDATE_SIGNING_KEY:-}" ]; then
  key=$(mktemp)
  trap 'rm -f "$key"' EXIT
  printf '%s\n' "$MESA_UPDATE_SIGNING_KEY" > "$key"
  openssl pkeyutl -sign -rawin -inkey "$key" -in checksums.txt -out checksums.txt.sig
  echo "signed checksums.txt"
else
  echo "warning: MESA_UPDATE_SIGNING_KEY is not set; desktop apps will not install this release by themselves" >&2
fi
