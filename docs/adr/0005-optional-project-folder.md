# ADR 0005: Mesa needs no folder; `.mesa/` is an optional overlay

Status: accepted · 2026-10-21

## Decision

By default Mesa keeps connections, saved queries, query history, diagram layouts and migration history in its own store: SQLite under `~/.config/mesa` for desktop and CLI, PostgreSQL in server mode. A project folder `.mesa/` (`mesa init`, or linked in the app) is an optional overlay for teams that want schema files next to their code.

## `.mesa/` layout

| Path | Holds |
| --- | --- |
| `mesa.toml` | Project name, default environment, connections per environment. Never secrets: passwords are `env:NAME` or `keychain:…` references |
| `models/*.dbml` | Schema models, one per database |
| `models/*.layout.json` | Diagram positions |
| `migrations/` | `<timestamp>_<name>.up.sql` / `.down.sql` for applied changes |
| `queries/*.sql` | Saved queries |

## Consequences

- The schema designer always shows a real database: existing ones open drawn from the live schema; new ones open empty. DBML and migration files are exports, never requirements.
- The CLI works from any directory with Mesa's own connections; inside a folder with `.mesa/` it also picks up that project.
