# ADR 0002: One binary, many surfaces, one application layer

Status: accepted · 2026-10-21

## Decision

`mesa` is a single binary with subcommands: `serve` (HTTP API + embedded web UI), the CLI commands (`connections`, `query`, `model`, `diff`, `apply`), and `mcp`. A desktop build wraps the same web UI (ADR 0003). All of them call `internal/app`; no surface contains business logic.

## Rules

- A feature is done when it works from HTTP and from the CLI.
- Safety (environment, read-only, confirm-by-name, audit) lives in `app/changes` and `core/policy`, so no surface can skip it.
- Words and symbols match across surfaces: environment name + colour, `✓ ! ✗` in the terminal, the same SQL review before any write.

## Consequences

- `cmd/mesa` is wiring only.
- `transport/*` packages are thin and tested through their public contract.
