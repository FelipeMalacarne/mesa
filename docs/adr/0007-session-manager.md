# ADR 0007: Pooled sessions through a session manager; secrets stay in the vault

Status: accepted · 2026-10-21

## Context

The previous code opened a new database pool on every request, built DSNs by hand and passed plaintext passwords through every handler.

## Decision

`session.Manager` resolves a connection id and database name into a cached `engine.Session` backed by a pool, created from the connection entity plus the secret fetched from `secret.Vault`. Sessions are evicted after idle time and on connection changes. Use cases receive a session; they never see the secret.

## Consequences

- Drivers build connection configs with their own parsers (`pgx.ParseConfig`), never string concatenation.
- TLS mode, timeouts and application name are part of `engine.Target`.
- `Ping`/`Test connection` opens a short-lived session and discards it.
