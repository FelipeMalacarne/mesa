#!/bin/sh
# Creates the Ed25519 key pair that signs release checksums:
#   packaging/release-key.sh mesa-release.pem
# Store the PEM file's contents as the MESA_UPDATE_SIGNING_KEY secret and
# the printed public key as the MESA_UPDATE_PUBLIC_KEY variable.
set -eu
out=${1:-mesa-release.pem}
[ -e "$out" ] && { echo "$out exists" >&2; exit 1; }
umask 077
openssl genpkey -algorithm ed25519 -out "$out"
printf 'MESA_UPDATE_PUBLIC_KEY=%s\n' "$(openssl pkey -in "$out" -pubout -outform DER | tail -c 32 | base64)"
