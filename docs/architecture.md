# Mesa architecture

Mesa is one Go application layer shipped as one binary per surface, one directory each under `cmd/`: `cmd/server` builds `mesa-server` (HTTP API with the embedded React UI and the MCP tools over HTTP; the Docker/Kubernetes image), `cmd/cli` builds `mesa` (the CLI), `cmd/mcp` builds `mesa-mcp` (the MCP server on stdio for local agents) and `cmd/desktop` builds `mesa-desktop` (a Wails window serving the same handler in process; `make desktop`). Every main is wiring over `internal/platform/bootstrap`. The product design lives in the Mesa design system and screen canvas; this document describes how the code is organised to deliver it.

## Principles

1. **One application, many transports.** Transports translate; they never contain logic. A feature exists when it works from the HTTP API and the CLI.
2. **Safe by default.** Every write is a reviewed change: staged → planned as SQL → checked by policy (environment, read-only, confirm-by-name) → applied in a transaction → audited. There is no other path.
3. **One schema model.** Live catalog, DBML, the designer, the diff and the DDL generator all speak `core/schema`.
4. **Engines are plug-ins with declared capabilities.** PostgreSQL and MySQL pass the same conformance suite (`engine/enginetest`); an engine implements what it can, and the UI shows only what is supported. MySQL has no schemas inside a database, so its catalog reports one schema named after the database; the tree skips that level and the designer folds a model's schemas into it.
5. **Mesa works with nothing but itself.** Connections, history and models live in Mesa's store (SQLite on desktop/CLI, PostgreSQL in server mode). A `.mesa/` project folder is an optional overlay.

## Packages

### `internal/core` — pure, no I/O

| Package | Holds |
| --- | --- |
| `ident` | Identifiers and qualified names, quoting rules, validation |
| `schema` | Engine-neutral model (`Database`, `Schema`, `Table`, `Column`, `Index`, `ForeignKey`, `Enum`) and `Diff(from, to) → []Change` |
| `value` | Typed cell values (`Null`, `Bool`, `Int`, `Float`, `Decimal`, `Text`, `Bytes`, `UUID`, `JSON`, `Time`, `Array`) and their JSON wire form |
| `changeset` | `ChangeSet` (row inserts/updates/deletes + schema changes) and `Plan` (ordered statements, down statements, risks) |
| `policy` | `Environment`, read-only rules, confirmation rules → `Decision` |
| `connection` | The `Connection` entity (never holds secrets) |
| `account` | People, roles (viewer, editor, admin) and the `Principal` a request runs as; `Require` checks a permission |
| `errs` | Error kinds: `NotFound`, `Invalid`, `Forbidden`, `Conflict`, `Unavailable`, `Unsupported`, `Unauthenticated` |

### `internal/engine` — driver ports

```go
type Engine interface {
    Name() string
    Capabilities() Capabilities
    Open(ctx, Target) (Session, error)   // Target = host, port, database, user, secret, TLS
}
type Session interface {
    Catalog  // Database(ctx) (schema.Database, error); Databases(ctx); TableStats…
    Exec     // Query(ctx, sql, args) (Rows, error); Rows paged; Tx(ctx, func(Tx) error)
    Monitor  // Overview, Metrics, Sessions, Kill, EngineMetrics (engine-specific panel)
    Dialect  // Compile(changeset.ChangeSet) (changeset.Plan, error)
    Ping(ctx) error; Close()
}
```

`engine/postgres` implements all four on `pgxpool`; catalog SQL is embedded. `engine/enginetest` is a conformance suite every engine must pass.

### `internal/session`

`Manager` turns a `connection.Connection` plus its secret into a pooled `engine.Session` per (connection, database), evicts idle sessions, and never exposes the secret to callers.

### `internal/app` — use cases

One package per feature: `connections`, `explore`, `changes`, `query`, `monitor`, `designer`, `accounts` (sign-in through an OpenID Connect provider, sessions, API tokens; ADR 0011), `provision` (an app's database on a CloudNativePG cluster, created now and committed to the GitOps repository; ADR 0012). Use cases that change something call `account.Require` with the principal in the context; transports set it (the signed-in person, or the local principal on desktop, CLI and stdio MCP). Inputs and outputs are plain structs. `changes` is the single write path; `designer` builds on `schema.Diff`, `dbml` and `Dialect.Compile`.

### `internal/store`

Mesa's own data behind ports: `Connections`, `Secrets`, `Audit`, `Layouts` (diagram positions), `History` (queries). `store/sqlite` for desktop and CLI (`~/.config/mesa/mesa.db`), `store/postgres` for server mode, `store/project` reads an optional `.mesa/` folder and overlays it on the store and vault: its connections (secrets only as `env:`/`keychain:` references, stable IDs from project name and environment), layouts as `models/*.layout.json`, saved queries as `queries/*.sql`. The CLI finds it from the working directory; the server takes `MESA_PROJECT`. `store/declared` overlays the connections a deployment declares in a file (`MESA_CONNECTIONS`, ADR 0009) the same way: read-only, passwords as references.

### Adapters for the platform

`internal/oidc` implements the identity provider port of `app/accounts` on go-oidc. `internal/github` implements the repository port of `app/provision` on GitHub's REST API. `internal/sops` encrypts YAML for age recipients in the sops format, and `internal/cnpg` renders the Kustomize component Mesa owns in a repository (CNPG `Database`, managed roles, Secrets); both are pure.

### `internal/transport`

- `http`: OpenAPI strict server (oapi-codegen) on chi, SSE for streamed results, embedded `web/dist`.
- `cli`: cobra commands sharing the app layer (`connections`, `query`, `model pull`, `diff`, `apply`, `init`).
- `platform/bootstrap` (not a transport): loads config, opens the store and vault, builds the `app.App`; every `cmd/*` main calls it.
- `mcp`: MCP tools over stdio, read-only unless `--allow-writes`; writes still go through review.

## The write path

Ad-hoc queries (`app/query`) always run in a read-only transaction. Anything that writes is
a change: row edits and schema changes arrive as a `ChangeSet`, hand-written SQL arrives as a
script (`changeset.FromScript`, which splits it safely and reads risks from the text).

```
grid edit / structure / designer / SQL script / CLI / MCP
        │ changeset.ChangeSet
        ▼
engine.Dialect.Compile  ──► changeset.Plan {Up, Down, Risks}
        │
policy.Evaluate(connection.Environment, plan) ──► Allow | NeedsConfirmation | Deny
        │
store.Audit.Begin ──► engine.Exec.Tx(plan.Up) ──► store.Audit.Finish
```

## Data flow for reads

`transport` → `app/explore` → `session.Manager.Session(connID, db)` → `engine.Catalog/Exec` → `value.Row` → transport encodes with `value` JSON form.

## Testing

- Unit tests for every `core` package, golden files for `dbml`, `schema.Diff` and `Dialect.Compile`.
- `enginetest` runs against a real PostgreSQL when `MESA_TEST_PG` is set; CI starts one.
- HTTP handlers tested through the generated client; CLI through cobra with an in-memory store.
- Web: Vitest for logic and components; typecheck and build in CI.
