# Mesa — working notes for agents

Mesa is an open-source SQL control plane: one Go application layer shipped as one binary per surface (web service for Docker/Kubernetes, desktop app, CLI, MCP server), with a React web UI. PostgreSQL first; other engines behind the same model.

Read `docs/architecture.md` before changing structure, and the ADRs in `docs/adr/` before revisiting a decision.

## Layout

```
cmd/server          the service: HTTP API + embedded web UI + MCP over HTTP (the Docker image) → mesa-server
cmd/cli             the CLI → mesa
cmd/mcp             the MCP server on stdio → mesa-mcp
cmd/desktop         the desktop app (Wails; build tag desktop, `make desktop`) → mesa-desktop
                    each main is wiring only, over internal/platform/bootstrap
api/openapi.yaml    the HTTP contract (generated code lives next to its adapter)
internal/core       pure Go, no I/O: ident, schema, value, changeset, policy, connection, account, errs
internal/engine     driver ports (catalog, exec, monitor, dialect) + postgres, mysql + enginetest
internal/session    connection + secret → pooled Session per (connection, database)
internal/secret     vault: local AES-GCM, env: references
internal/dbml       DBML parse/print ↔ schema.Database
internal/app        use cases, one package per feature; plain structs in and out
                    (accounts: sign-in via OIDC; provision: CNPG databases committed to Git)
internal/oidc       OpenID Connect provider (go-oidc) for app/accounts
internal/github     GitHub repository (REST) for app/provision
internal/sops       sops-format encryption for age recipients (pure)
internal/cnpg       the Kustomize component Mesa writes for CloudNativePG (pure)
internal/store      Mesa's own data: ports + sqlite (desktop/CLI) + postgres (server) + project (optional .mesa/ overlay)
                    + declared (connections a deployment declares in MESA_CONNECTIONS)
internal/transport  thin adapters: http, cli, mcp
internal/platform   config, logging, paths, version
web/                React app: src/app (shell), src/features/*, src/ui (design system), src/lib
```

## Rules

- Dependencies point inward: `core` imports nothing internal; `engine`, `dbml`, `secret` import only `core`; `app` imports those and `session`/`store`; `transport` imports only `app` (and `core` for types); `cmd/*` imports `platform/bootstrap` and one transport. `golangci-lint` (depguard) enforces this; do not loosen it.
- Packages are named for what they provide, never by layer. Keep them small.
- Secrets never reach a use case or a transport. `session.Manager` hands out a ready `engine.Session`.
- Every request runs as an `account.Principal` in its context; use cases that change something call `account.Require`. Transports set the principal (the signed-in person, or `account.Local()` on local surfaces); a context without one is refused.
- Every write goes through `app/changes`: ChangeSet → Plan → policy → apply → audit. No transport runs SQL on its own.
- Engine-specific behaviour lives in the engine package. Use cases ask `Capabilities()` and never type-switch on the engine.
- `core/value` is the only cell representation. Never pass `any` from a driver to a transport.
- Generated code stays beside its adapter (`transport/http/gen`, `store/sqlite/gen`) and is regenerated with `make generate`, never edited.
- Errors: wrap with `errs.*` kinds; transports map kinds to status codes. Never leak driver errors to clients unwrapped.
- Tests: `go test ./...` must pass offline. Engine integration tests run when `MESA_TEST_PG` (a Postgres URL) or `MESA_TEST_MYSQL` (a go-sql-driver DSN such as `root:mesa@tcp(127.0.0.1:3306)/`) is set, otherwise they skip. Keep all green.

## Conventions

- Go 1.26, `gofmt`, `go vet`, `golangci-lint run`. Comments and identifiers in English.
- Context first argument everywhere; no package-level state; constructors return concrete types, accept interfaces.
- SQL that reads the catalog lives in `.sql` files embedded next to the driver.
- Web: TypeScript strict, Vitest, generated client via `orval` from `api/openapi.yaml`. Styling only through the tokens in `web/src/styles.css` (generated from the design system); no raw colours.
- Commits: conventional (`feat(engine): …`), one concern per commit.

## Running

```
make dev-db        # local Postgres for development/tests (see Makefile)
make generate      # openapi → Go + TS
make test          # go test ./... (+ integration if MESA_TEST_PG)
make lint
make web           # build the web app into web/dist (embedded by mesa-server)
go run ./cmd/server
go run ./cmd/cli --help
go run ./cmd/mcp
```
