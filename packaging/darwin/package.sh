#!/bin/sh
# Builds Mesa.app from a universal mesa-desktop binary, signs and notarizes
# it when the signing secrets are present, and writes the updater's zip and
# a disk image:
#   packaging/darwin/package.sh <binary> <version> <outdir>
# Signing (optional): MACOS_SIGNING_IDENTITY names a Developer ID
# Application identity already in the keychain; notarizing (optional):
# APPLE_ID, APPLE_TEAM_ID, APPLE_APP_PASSWORD.
set -eu
bin=$1 version=$2 out=$3
here=$(cd "$(dirname "$0")" && pwd)
stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT
app="$stage/Mesa.app"
mkdir -p "$app/Contents/MacOS" "$app/Contents/Resources"
install -m 0755 "$bin" "$app/Contents/MacOS/mesa-desktop"
sed "s/@VERSION@/$version/g" "$here/Info.plist" > "$app/Contents/Info.plist"

iconset="$stage/mesa.iconset"
mkdir -p "$iconset"
for size in 16 32 128 256 512; do
  sips -z $size $size "$here/../icons/mesa.png" --out "$iconset/icon_${size}x${size}.png" >/dev/null
  double=$((size * 2))
  sips -z $double $double "$here/../icons/mesa.png" --out "$iconset/icon_${size}x${size}@2x.png" >/dev/null
done
iconutil -c icns "$iconset" -o "$app/Contents/Resources/mesa.icns"

if [ -n "${MACOS_SIGNING_IDENTITY:-}" ]; then
  codesign --force --deep --options runtime --timestamp --sign "$MACOS_SIGNING_IDENTITY" "$app"
  codesign --verify --strict --verbose=2 "$app"
else
  echo "warning: MACOS_SIGNING_IDENTITY is not set; Mesa.app is ad-hoc signed and Gatekeeper will warn" >&2
  codesign --force --deep --sign - "$app"
fi

mkdir -p "$out"
zip="$out/Mesa_darwin_universal.zip"
ditto -c -k --keepParent "$app" "$zip"
if [ -n "${MACOS_SIGNING_IDENTITY:-}" ] && [ -n "${APPLE_ID:-}" ] && [ -n "${APPLE_TEAM_ID:-}" ] && [ -n "${APPLE_APP_PASSWORD:-}" ]; then
  xcrun notarytool submit "$zip" --apple-id "$APPLE_ID" --team-id "$APPLE_TEAM_ID" --password "$APPLE_APP_PASSWORD" --wait
  xcrun stapler staple "$app"
  rm "$zip"
  ditto -c -k --keepParent "$app" "$zip"
fi

dmg="$out/Mesa_${version}_darwin_universal.dmg"
ln -s /Applications "$stage/Applications"
hdiutil create -volname "Mesa $version" -srcfolder "$app" -srcfolder "$stage/Applications" -ov -format UDZO "$dmg" >/dev/null
if [ -n "${MACOS_SIGNING_IDENTITY:-}" ]; then
  codesign --force --timestamp --sign "$MACOS_SIGNING_IDENTITY" "$dmg"
fi
echo "$zip"
echo "$dmg"
