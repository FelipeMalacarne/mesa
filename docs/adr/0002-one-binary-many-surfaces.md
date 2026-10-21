# ADR 0002: One application layer, one binary per surface

Status: accepted · 2026-10-21

## Decision

Each surface is its own binary with its own entry point under `cmd/`, named after the surface: `cmd/server` → `mesa-server` (HTTP API + embedded web UI + the MCP tools over HTTP, the container image), `cmd/cli` → `mesa` (the CLI: `connections`, `query`, `model`, `diff`, `apply`, `audit`), `cmd/mcp` → `mesa-mcp` (MCP over stdio for agents on the same machine) and `cmd/desktop` → `mesa-desktop` (ADR 0003). All of them call `internal/app` through `internal/platform/bootstrap`; no surface contains business logic. Separate binaries keep each image and install small (the server never ships cobra, the CLI never embeds the web app) and let each surface be released and packaged on its own.

## Rules

- A feature is done when it works from HTTP and from the CLI.
- Safety (environment, read-only, confirm-by-name, audit) lives in `app/changes` and `core/policy`, so no surface can skip it.
- Words and symbols match across surfaces: environment name + colour, `✓ ! ✗` in the terminal, the same SQL review before any write.

## Consequences

- Every `cmd/*` main is wiring only; shared setup lives in `platform/bootstrap`.
- `transport/*` packages are thin and tested through their public contract.
