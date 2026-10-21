#!/bin/sh
# Packages a built mesa-desktop for Linux as the archive the updater reads:
#   packaging/linux/package.sh <binary> <arch> <version> <outdir>
# → <outdir>/mesa-desktop_linux_<arch>.tar.gz with the binary, a desktop
# entry declaring mesa:// links, and the icon. The app also registers
# itself for mesa:// links on first run.
set -eu
bin=$1 arch=$2 version=$3 out=$4
here=$(cd "$(dirname "$0")" && pwd)
stage=$(mktemp -d)
trap 'rm -rf "$stage"' EXIT
dir="$stage/mesa-desktop_${version}_linux_${arch}"
mkdir -p "$dir"
install -m 0755 "$bin" "$dir/mesa-desktop"
install -m 0644 "$here/mesa.desktop" "$dir/mesa.desktop"
install -m 0644 "$here/../icons/mesa.png" "$dir/mesa.png"
cat > "$dir/README.txt" <<TXT
Mesa ${version} for Linux (${arch})

Run ./mesa-desktop. It needs WebKitGTK 4.1 and GTK 3
(Debian/Ubuntu: sudo apt install libwebkit2gtk-4.1-0 libgtk-3-0).
On first run it registers itself as the handler of mesa:// links.
TXT
mkdir -p "$out"
tar -C "$stage" -czf "$out/mesa-desktop_linux_${arch}.tar.gz" "$(basename "$dir")"
echo "$out/mesa-desktop_linux_${arch}.tar.gz"
