# ADR 0008: Package layout by concept, dependencies pointing inward, enforced by lint

Status: accepted · 2026-10-21

## Decision

Packages are named for what they provide (`schema`, `changeset`, `policy`), never by layer. `core` imports nothing internal; `engine`, `dbml`, `secret` import only `core`; `app` imports `core`, `engine`, `session`, `store`, `dbml`; `transport` imports `app` and `core`. `golangci-lint` with `depguard` fails the build on a violation.

## Reasons

- The old single `domain/connection` package held every concept and every change touched it.
- A linter keeps the rule true when most code is generated.

## Consequences

- Interfaces live with their consumers, except the engine ports, which every driver implements.
- Generated code sits beside its adapter and is excluded from lint.
