# ADR 0006: Every write is a ChangeSet → Plan → policy → apply → audit

Status: accepted · 2026-10-21

## Decision

Grid edits, schema designer edits, CLI `apply` and MCP writes all produce a `changeset.ChangeSet`. The engine dialect compiles it into a `changeset.Plan` (ordered SQL, down statements, risks: locks, data loss, rewrites). `core/policy` evaluates the plan against the connection's environment and read-only state, and may require typing the database name. The plan runs in one transaction and is recorded in the audit store before and after.

## Reasons

- The person always sees the SQL before it runs, on every surface.
- Production safety cannot be bypassed by a new transport or a bug in the UI.
- Up/down statements give migrations for free.

## Consequences

- `app/changes` is the only package that executes DML/DDL; `app/query` runs ad-hoc SQL read-only by default and routes writes through a plan when the connection is read-only.
- Plans are serialisable so the UI can display them and the CLI can save them as files.
