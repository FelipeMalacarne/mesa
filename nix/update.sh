#!/bin/sh
# Points the flake at a release: writes nix/release.json with the version and
# the SRI hash of every file the Nix packages download, taken from the
# release's checksums.txt.
#   nix/update.sh 0.1.1
set -eu
version=${1#v}
here=$(cd "$(dirname "$0")" && pwd)
sums=$(curl -fsSL "https://github.com/FelipeMalacarne/mesa/releases/download/v$version/checksums.txt")
VERSION="$version" SUMS="$sums" python3 - "$here/release.json" <<'PY'
import base64, json, os, sys
version = os.environ["VERSION"]
wanted = [f"mesa_{version}_{os_}_{arch}.tar.gz" for os_ in ("linux", "darwin") for arch in ("amd64", "arm64")]
wanted += ["mesa-desktop_linux_amd64.tar.gz", "mesa-desktop_linux_arm64.tar.gz", "Mesa_darwin_universal.zip"]
hashes = {}
for line in os.environ["SUMS"].splitlines():
    digest, name = line.split()
    if name in wanted:
        hashes[name] = "sha256-" + base64.b64encode(bytes.fromhex(digest)).decode()
missing = sorted(set(wanted) - set(hashes))
if missing:
    sys.exit(f"checksums.txt lacks {missing}")
with open(sys.argv[1], "w") as f:
    json.dump({"version": version, "hashes": dict(sorted(hashes.items()))}, f, indent=2)
    f.write("\n")
PY
echo "nix/release.json now points at v$version"
