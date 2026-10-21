# Mesa web app

React 19 + Vite, TanStack Router (file routes) and Query, zustand for local state. Styling
comes only from the design system: tokens in `src/styles.css`, components as `.m-*` classes
in `src/ui/mesa.css`. No raw colours.

```bash
pnpm install
pnpm dev                      # http://localhost:3000, proxies /api to $MESA_API or :8080
pnpm run codegen              # regenerate src/lib/api from ../api/openapi.yaml (never edit it)
pnpm run typecheck && pnpm test
pnpm run build                # writes dist/, which mesa-server embeds
```

## Layout

```
src/app            router, routes/ (file-based), providers, theme, palette, status bar store
src/features       one folder per screen: connections, explorer (rail, tree, tabs, status bar),
                   table (grid, edits, structure), sql, monitor, designer, roles, activity,
                   changes (the review dialog every write goes through), workspace (layout)
src/ui             design-system components: Button, Code (highlighter), charts, controls,
                   primitives (dialog, popover, command palette)
src/lib            generated API client (api/), value wire format (values.ts), formatting
```

## Rules the screens follow

- Every write is reviewed. Grid edits, structure edits, the designer, roles, new databases
  and SQL that writes all open `ReviewDialog`, which shows the SQL, its rollback, the risks
  and the policy decision (unlock a read-only connection, type the database name for
  production or data loss). Nothing is applied before that.
- The SQL editor runs queries in a read-only transaction. A statement that writes is sent to
  the script review instead (`/changes/script`), so it is audited like any other write.
- Views publish facts to the status bar with `useStatusFacts` and staged work with
  `usePendingStatus`; the workspace layout renders them.
