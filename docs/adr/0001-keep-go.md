# ADR 0001: Keep Go for the backend

Status: accepted · 2026-10-21

## Context

Mesa must run as a web service, a CLI, an MCP server and a desktop app. A full restart of the backend was on the table, and Rust was considered.

## Decision

Keep Go. Rebuild the backend's structure, not its language.

## Reasons

- Mesa's work is I/O-bound (database and HTTP); the database is the slow part, not the language.
- The best libraries for the job are already Go: `pgx` + `sqlc`, `oapi-codegen`, cobra, the MCP Go SDK, Wails, and Kubernetes tooling.
- `modernc.org/sqlite` is pure Go, so one static binary builds for every OS and the Docker image stays small.
- Go compiles in seconds and reads plainly, which keeps AI-generated code converging quickly and reviewable.

## Consequences

- A CPU-heavy module (local analytics, large diffs) may later be written in Rust behind an interface; nothing else moves.
