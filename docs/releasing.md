# Releasing

Pushing a tag `vX.Y.Z` (or `vX.Y.Z-rc.N` for a prerelease) runs
`.github/workflows/release.yml`, which builds everything, signs what it has keys
for and publishes a GitHub release. Running the workflow by hand with a version
builds the same files as a dry run and keeps them as an artifact.

## What a release contains

| File | What |
| --- | --- |
| `mesa_<v>_<os>_<arch>.tar.gz` / `.zip` | `mesa`, `mesa-server`, `mesa-mcp` for Linux, macOS and Windows on amd64 and arm64 |
| `Mesa_<v>_darwin_universal.dmg` | the desktop app for macOS (Apple Silicon and Intel) |
| `Mesa_darwin_universal.zip` | the same app bundle, read by the updater |
| `Mesa_<v>_windows_amd64_setup.exe` | per-user installer: Start menu entry, `mesa://` links, uninstaller |
| `mesa-desktop_windows_amd64.zip` | the Windows app alone, read by the updater |
| `mesa-desktop_linux_<arch>.tar.gz` | the Linux app, its desktop entry and icon |
| `checksums.txt` | SHA-256 of every file above |
| `checksums.txt.sig` | Ed25519 signature of `checksums.txt` by the release key |
| `checksums.txt.sigstore.json` | Sigstore (cosign keyless) bundle for `checksums.txt` |

The image is pushed to `ghcr.io/felipemalacarne/mesa:<v>` (and `:latest` for a
final release) for amd64 and arm64, and signed with cosign keyless.

## Keys and certificates

Without any of these the release still builds; what is missing is reported in
the job logs and the corresponding files are unsigned.

| Name | Kind | Used for |
| --- | --- | --- |
| `MESA_UPDATE_SIGNING_KEY` | secret | Ed25519 private key (PEM) that signs `checksums.txt` |
| `MESA_UPDATE_PUBLIC_KEY` | variable | its public key, base64, linked into the desktop app. Desktop builds without it never install updates |
| `MACOS_CERTIFICATE` | secret | Developer ID Application certificate, `.p12` as base64 |
| `MACOS_CERTIFICATE_PASSWORD` | secret | its password |
| `MACOS_SIGNING_IDENTITY` | secret | e.g. `Developer ID Application: Name (TEAMID)` |
| `APPLE_ID`, `APPLE_TEAM_ID`, `APPLE_APP_PASSWORD` | secrets | notarization with `notarytool` (an app-specific password) |
| `WINDOWS_CERTIFICATE` | secret | Authenticode code-signing certificate, `.pfx` as base64 |
| `WINDOWS_CERTIFICATE_PASSWORD` | secret | its password |

Create the update key once and keep the PEM file offline:

```bash
packaging/release-key.sh mesa-release.pem
# prints MESA_UPDATE_PUBLIC_KEY=…; store it as a repository variable,
# and the contents of mesa-release.pem as the MESA_UPDATE_SIGNING_KEY secret
```

Rotating the key means shipping one release signed with the old key whose
binaries carry the new public key, then switching the secret.

## Verifying a download by hand

```bash
sha256sum --check --ignore-missing checksums.txt
cosign verify-blob checksums.txt --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp 'https://github.com/felipemalacarne/mesa/.github/workflows/release.yml@refs/tags/v.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
cosign verify ghcr.io/felipemalacarne/mesa:<v> \
  --certificate-identity-regexp 'https://github.com/felipemalacarne/mesa/.github/workflows/release.yml@refs/tags/v.*' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```

## Building the packages locally

```bash
make web desktop
packaging/linux/package.sh bin/mesa-desktop amd64 0.0.0-dev dist      # Linux archive
(cd packaging/windows && makensis -DVERSION=0.0.0 -DSOURCE=/path/mesa-desktop.exe installer.nsi)
packaging/darwin/package.sh bin/mesa-desktop 0.0.0-dev dist           # on macOS
MESA_UPDATE_SIGNING_KEY="$(cat mesa-release.pem)" packaging/checksums.sh dist
```
