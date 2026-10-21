# Mesa

**The open-source SQL control plane.** One Go application you can run as a
service (Docker, Kubernetes), from the terminal, or hand to an agent over MCP,
for exploring, changing and watching your databases. PostgreSQL first; the
engine layer is a port.

Every write Mesa makes is a reviewed change: it is planned as SQL you can read,
checked by policy (read-only connections, production needs the database name
typed back), applied in one transaction and written to an audit log. Grid edits,
the schema designer, the CLI and the MCP tools all go through that one path.

## What it does

- **Connections** with environments (local, development, staging, production).
  Production is read-only until you unlock it for a while.
- **Explorer**: a nested tree of databases, schemas, tables, views, enums,
  functions; a grid with filters, sort, pending edits and a commit review
  showing the exact SQL.
- **Structure**: columns, indexes, foreign keys, checks, with changes planned
  as DDL and the matching rollback.
- **SQL editor** with results, EXPLAIN (and ANALYZE, rolled back), history and saved
  queries. Queries run in a read-only transaction; a statement that writes is reviewed
  and audited like any other change.
- **Monitor**: general metrics (connections, transactions per second, cache
  hit, locks, longest transaction) plus an engine panel; PostgreSQL shows
  replication, vacuum, bloat, slow queries. Active sessions, cancel or
  terminate.
- **Designer**: an ERD and DBML side by side, kept in sync. Open an existing
  database and the diagram is drawn from the live schema; create a new
  database and start from an empty canvas. Review the diff, apply.
- **Roles** and databases, created through the same reviewed path.
- **Surfaces**: `mesa-server` (web app + HTTP API), `mesa` (CLI), `mesa-mcp`
  (Model Context Protocol on stdio). A desktop shell is planned.

## Run it

### Docker

```bash
docker run -d --name mesa -p 8080:8080 \
  -e MESA_APP_KEY="$(openssl rand -base64 32)" \
  -v mesa:/data ghcr.io/felipemalacarne/mesa:latest
```

Open http://localhost:8080 and add a connection. `docker compose up --build`
starts Mesa with a PostgreSQL 17 beside it.

### Kubernetes

```bash
helm install mesa ./charts/mesa --set config.appKey="$(openssl rand -base64 32)"
```

Set `config.store=postgres` and `config.storeDSN` to run several replicas with
Mesa's own data in PostgreSQL.

### CLI

```bash
mesa connections add --name shop --env production --host db.internal --user app --database shop
mesa query shop "select count(*) from orders"   # read-only; writes: mesa apply --sql
mesa model pull shop > shop.dbml           # the live schema as DBML
mesa diff shop --model shop.dbml           # SQL + risks for the changes in the file
mesa apply shop --model shop.dbml --confirm shop --save-migration ./migrations
mesa audit shop
```

The CLI keeps its data in `$MESA_HOME` or the OS config directory. The
`MESA_PASSWORD` variable or `--secret env:NAME` keeps passwords out of shell
history.

### MCP

Local agents talk to the stdio binary; agents that reach the service use the
server's `/mcp` endpoint (streamable HTTP).

```json
{ "mcpServers": { "mesa": { "command": "mesa-mcp" } } }
{ "mcpServers": { "mesa": { "url": "http://localhost:8080/mcp" } } }
```

Tools: `list_connections`, `list_databases`, `get_schema` (DBML), `query`,
`table_rows`, `explain`, `monitor`, `sessions`, `plan_sql`. `mesa-mcp
--allow-writes` (or `MESA_MCP_WRITES=true` on the server) adds `apply_sql`;
it still goes through policy and audit.

## Configuration

| Variable | Default | Meaning |
| --- | --- | --- |
| `MESA_APP_KEY` | required in server mode | Key that encrypts connection secrets at rest (`mesa key` generates one) |
| `MESA_STORE` | `sqlite` | Where Mesa keeps its own data: `sqlite` or `postgres` |
| `MESA_STORE_DSN` | `<config dir>/mesa.db` | SQLite path or PostgreSQL URL |
| `MESA_LISTEN` | `:8080` | Server address |
| `MESA_LOG_LEVEL` / `MESA_LOG_FORMAT` | `info` / `text` | Logging |
| `MESA_SESSION_IDLE` | `10m` | Idle database sessions are closed after this |
| `MESA_MCP_WRITES` | `false` | Expose `apply_sql` on the server's `/mcp` endpoint |
| `MESA_HOME` | OS config dir + `/mesa` | CLI and MCP data directory |

Connection passwords can be stored encrypted or given as `env:NAME`, which
Mesa resolves from its own environment at connect time.

## Develop

```bash
make dev-db            # throwaway PostgreSQL on :5499 (needs postgres binaries)
make generate          # OpenAPI server (Go) and client (TypeScript)
make web && make build # web/dist, then bin/mesa-server, bin/mesa, bin/mesa-mcp
make test-integration  # go test ./... against the dev database
```

`cmd/server`, `cmd/cli` and `cmd/mcp` are the entry points (`cmd/desktop` comes later). `docs/architecture.md` explains the layout: `core` (pure model, diff, plans,
policy) → `engine` (ports, PostgreSQL driver) → `session` → `app` (use cases)
→ `transport` (HTTP, CLI, MCP). `docs/adr/` records the decisions. The product
design lives in the Mesa design system and screen canvas linked from
`CLAUDE.md`.

## License

MIT
