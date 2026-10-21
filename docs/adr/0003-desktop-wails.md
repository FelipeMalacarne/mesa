# ADR 0003: Desktop app is a Wails shell around the web UI

Status: accepted · 2026-10-21

## Context

A native desktop UI (Rust, Swift, Qt) would be a second UI to build and keep in parity with the web app, which must exist for the service anyway.

## Decision

The desktop app is a Wails shell: the Go application runs in-process, serves the same HTTP contract to the same React app through the webview, and adds only desktop concerns: SQLite store in `~/.config/mesa`, secrets in the OS keychain, native menus and dialogs, deep links, updates and signing.

## Consequences

- One UI, one API contract, one design system.
- Large result grids are solved in the web app (virtualised rows, paged results), not by going native.
- The desktop build is a build tag and an entry point, not a separate codebase: `cmd/desktop`, built with `-tags desktop,production,webkit2_41` (`make desktop`). The webview's requests, the API included, go to the same `transport/http` handler in process.
- The key that encrypts connection secrets is kept in the OS keychain (`mesa`/`app-key`); where no keychain is reachable it falls back to a key file in the config directory, as the CLI does.
- Deep links: `mesa://c/<connection>/…` opens the same route as the web app (`platform/desktop.Route` accepts plain paths only). One instance runs at a time; a second launch hands its link to the first. macOS declares the scheme in the bundle's Info.plist, the Windows installer writes it under HKCU, and on Linux (and Windows) the app registers itself on start.
- Updates: once a day, and from Help → Check for updates, the app asks GitHub for the latest release. It installs one only after verifying the archive against `checksums.txt` and that file's Ed25519 signature, made with the release key whose public half is linked into the binary. A build without that key, or a development build, never installs anything. The old executable (or app bundle) is swapped by rename; the new process waits for the old one to quit.
- Releases come from `.github/workflows/release.yml`: binaries for every surface, a signed and notarized macOS app, a signed Windows installer, Linux archives, a cosign-signed image, and checksums signed twice (the updater's key, and Sigstore keyless for people). Platform signing needs certificates the project provides as secrets; see `docs/releasing.md`.
