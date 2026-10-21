#!/bin/sh
# Installs Mesa on macOS from a GitHub release, for people running the
# unsigned app on their own machine:
#   curl -fsSL https://raw.githubusercontent.com/FelipeMalacarne/mesa/main/packaging/darwin/install.sh | sh
# MESA_VERSION picks a release (default: the latest); MESA_INSTALL_DIR
# the destination (default: /Applications); MESA_NO_OPEN=1 skips opening it. The download is checked
# against the release's checksums.txt before anything is installed.
set -eu
repo=FelipeMalacarne/mesa
version=${MESA_VERSION:-latest}
dest=${MESA_INSTALL_DIR:-/Applications}
if [ "$version" = latest ]; then
  base="https://github.com/$repo/releases/latest/download"
else
  base="https://github.com/$repo/releases/download/v${version#v}"
fi
[ "$(uname -s)" = Darwin ] || { echo "this installs the macOS app; see the release page for other systems" >&2; exit 1; }

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
echo "Downloading Mesa ($version)…"
curl -fL --progress-bar -o "$tmp/Mesa_darwin_universal.zip" "$base/Mesa_darwin_universal.zip"
curl -fsSL -o "$tmp/checksums.txt" "$base/checksums.txt"
want=$(awk '$2 == "Mesa_darwin_universal.zip" { print $1 }' "$tmp/checksums.txt")
got=$(shasum -a 256 "$tmp/Mesa_darwin_universal.zip" | awk '{ print $1 }')
if [ -z "$want" ] || [ "$want" != "$got" ]; then
  echo "the download does not match the release checksum; nothing was installed" >&2
  exit 1
fi

ditto -x -k "$tmp/Mesa_darwin_universal.zip" "$tmp/app"
mkdir -p "$dest"
if [ -d "$dest/Mesa.app" ]; then
  osascript -e 'quit app "Mesa"' >/dev/null 2>&1 || true
  rm -rf "$dest/Mesa.app"
fi
mv "$tmp/app/Mesa.app" "$dest/Mesa.app"
# The app is not notarized; drop the quarantine flag so macOS opens it.
xattr -dr com.apple.quarantine "$dest/Mesa.app" 2>/dev/null || true
echo "Installed $dest/Mesa.app"
[ -n "${MESA_NO_OPEN:-}" ] || open "$dest/Mesa.app"
