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
- Still to come: deep links, auto-update and signed installers.
