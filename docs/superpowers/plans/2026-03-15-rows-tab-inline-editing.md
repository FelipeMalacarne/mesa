# Rows Tab — Inline Editing Implementation Plan

> **For agentic workers:** REQUIRED: Use superpowers:subagent-driven-development (if subagents available) or superpowers:executing-plans to implement this plan. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rename "Sample data" tab to "Rows", add inline double-click cell editing with pending-change yellow highlighting, and a confirm/discard flow backed by a new PUT endpoint.

**Architecture:** Backend adds `PUT /rows` to the OpenAPI spec, regenerates Go contract and frontend client, then wires up a new `UpdateTableRow` command through the existing gateway/command/handler pattern. Frontend adds a `--pending` design token, a thin `getRowClassName` prop to `DataTable`, and a new `EditableDataTable` wrapper that owns all edit state.

**Tech Stack:** Go (chi, oapi-codegen), React 18, TanStack Table v8, TanStack Query v5, Tailwind CSS v4, sonner (toasts), orval (frontend codegen)

---

## File Map

| File | Action | Responsibility |
|---|---|---|
| `openapi.yaml` | Modify | Add PUT /rows endpoint + `UpdateTableRowRequest` schema |
| `internal/transport/rest/contract/api.gen.go` | Regenerated | Do not edit — run `make codegen` |
| `web/src/api/connections/connections.ts` | Regenerated | Do not edit — run `make codegen` |
| `web/src/api/mesaAPI.schemas.ts` | Regenerated | Do not edit — run `make codegen` |
| `internal/domain/connection/gateway.go` | Modify | Add `UpdateTableRow` to `Administrator` interface |
| `internal/application/commands/update_table_row.go` | Create | `UpdateTableRowCmd` struct + handler |
| `internal/infrastructure/postgres/gateway.go` | Modify | Implement `UpdateTableRow` |
| `internal/application/app.go` | Modify | Register `UpdateTableRow` in `Commands` |
| `internal/transport/rest/handlers.go` | Modify | Add `UpdateTableRow` REST handler |
| `web/src/styles.css` | Modify | Add `--pending` + `--pending-foreground` design tokens |
| `web/src/components/data-table.tsx` | Modify | Add optional `getRowClassName` prop |
| `web/src/components/editable-data-table.tsx` | Create | `EditableDataTable` component + `RowChange` type |
| `web/src/routes/.../tables/_components/rows-tab.tsx` | Create | `RowsTab` — uses `EditableDataTable` |
| `web/src/routes/.../tables/_components/sample-tab.tsx` | Delete | Replaced by `rows-tab.tsx` |
| `web/src/routes/.../tables/$tableName.tsx` | Modify | Update import + tab values |

---

## Chunk 1: Backend

### Task 1: Add PUT /rows to the OpenAPI spec

**Files:**
- Modify: `openapi.yaml`

- [ ] **Step 1: Add the PUT operation under the existing `/rows` path**

In `openapi.yaml`, find the path `/connections/{connectionID}/databases/{databaseName}/tables/{tableName}/rows` (currently has only a `get`). Add the following `put` block directly after the `get` block at the same indentation level:

```yaml
    put:
      operationId: UpdateTableRow
      summary: Update a row in a table
      tags:
        - Connections
      parameters:
        - $ref: "#/components/parameters/ConnectionId"
        - $ref: "#/components/parameters/DatabaseName"
        - $ref: "#/components/parameters/TableName"
      requestBody:
        required: true
        content:
          application/json:
            schema:
              $ref: "#/components/schemas/UpdateTableRowRequest"
      responses:
        "204":
          description: Updated (No Content)
        "400":
          description: Bad Request
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Error"
        "500":
          description: Internal Server Error
          content:
            application/json:
              schema:
                $ref: "#/components/schemas/Error"
```

- [ ] **Step 2: Add the `UpdateTableRowRequest` schema**

In the `components.schemas` section (at the very end of the file, after `CreateTableIndex`), add:

```yaml
    UpdateTableRowRequest:
      type: object
      required: [where, set]
      properties:
        where:
          type: object
          additionalProperties: true
          description: Primary key column(s) and their values to identify the row
        set:
          type: object
          additionalProperties: true
          description: Column(s) and new values to apply
```

- [ ] **Step 3: Validate the YAML is well-formed**

```bash
cd /home/felipemalacarne/repos/mesa
python3 -c "import yaml; yaml.safe_load(open('openapi.yaml'))" && echo "OK"
```

Expected: `OK`

- [ ] **Step 4: Commit**

```bash
git add openapi.yaml
git commit -m "feat(api): add PUT /rows endpoint for inline row editing"
```

---

### Task 2: Regenerate Go contract and frontend client

**Files:**
- Regenerated: `internal/transport/rest/contract/api.gen.go`
- Regenerated: `web/src/api/connections/connections.ts`
- Regenerated: `web/src/api/mesaAPI.schemas.ts`

- [ ] **Step 1: Run codegen**

```bash
cd /home/felipemalacarne/repos/mesa
make codegen
```

Expected: no errors. The command runs:
1. `oapi-codegen -generate chi,types -package contract -o internal/transport/rest/contract/api.gen.go openapi.yaml`
2. `cd web && pnpm orval --config orval.config.ts`

- [ ] **Step 2: Verify the Go contract has the new interface method**

```bash
grep -n "UpdateTableRow" internal/transport/rest/contract/api.gen.go
```

Expected: lines showing `UpdateTableRow` in the `StrictServerInterface` or `ServerInterface`.

- [ ] **Step 3: Verify the frontend client has the new hook**

```bash
grep -n "UpdateTableRow\|updateTableRow" web/src/api/connections/connections.ts | head -10
```

Expected: lines for the mutation hook and URL helper.

- [ ] **Step 4: Check the Go project still compiles (will fail at this point — expected)**

```bash
cd /home/felipemalacarne/repos/mesa
go build ./...
```

Expected: compile error like `Server does not implement ServerInterface: missing UpdateTableRow method`. This is expected — we haven't wired up the handler yet. Move to Task 3.

- [ ] **Step 5: Commit generated files**

```bash
git add internal/transport/rest/contract/api.gen.go \
        web/src/api/connections/connections.ts \
        web/src/api/mesaAPI.schemas.ts
git commit -m "chore: regenerate api contract and frontend client for PUT /rows"
```

---

### Task 3: Extend the Administrator interface

**Files:**
- Modify: `internal/domain/connection/gateway.go`

- [ ] **Step 1: Add `UpdateTableRow` to the `Administrator` interface**

In `internal/domain/connection/gateway.go`, find the `Administrator` interface and add the new method:

```go
// Administrator handles user management and database creation.
type Administrator interface {
	KillSession(ctx context.Context, conn Connection, password string, pid int) error
	ListUsers(ctx context.Context, conn Connection, password string) ([]DBUser, error)
	CreateUser(ctx context.Context, conn Connection, password string, user DBUser, secret string) error
	DropUser(ctx context.Context, conn Connection, password string, username Identifier) error
	CreateDatabase(ctx context.Context, conn Connection, password string, dbName, owner Identifier) error
	UpdateTableRow(ctx context.Context, conn Connection, password string, dbName, tableName Identifier, where, set map[string]any) error
}
```

- [ ] **Step 2: Verify the project still does not compile (expected — gateway not yet implemented)**

```bash
cd /home/felipemalacarne/repos/mesa && go build ./... 2>&1 | head -5
```

Expected: error about `*Gateway` not implementing `connection.Administrator` (missing `UpdateTableRow`).

- [ ] **Step 3: Commit**

```bash
git add internal/domain/connection/gateway.go
git commit -m "feat(domain): add UpdateTableRow to Administrator interface"
```

---

### Task 4: Implement UpdateTableRow in the Postgres gateway

**Files:**
- Modify: `internal/infrastructure/postgres/gateway.go`

- [ ] **Step 1: Add the `UpdateTableRow` method to the Gateway**

At the end of the `// --- Administrator Implementation ---` section in `internal/infrastructure/postgres/gateway.go` (after `CreateDatabase`), add:

```go
func (h *Gateway) UpdateTableRow(ctx context.Context, conn connection.Connection, password string, dbName, tableName connection.Identifier, where, set map[string]any) error {
	if len(where) == 0 {
		return fmt.Errorf("%w: where clause cannot be empty", connection.ErrInvalidConfiguration)
	}
	if len(set) == 0 {
		return fmt.Errorf("%w: set clause cannot be empty", connection.ErrInvalidConfiguration)
	}

	db, err := h.connect(conn, password, dbName)
	if err != nil {
		return err
	}
	defer db.Close()

	args := make([]any, 0, len(set)+len(where))
	setClauses := make([]string, 0, len(set))

	i := 1
	for col, val := range set {
		colIdent, err := connection.NewIdentifier(col)
		if err != nil {
			return fmt.Errorf("%w: invalid set column name '%s': %v", connection.ErrInvalidConfiguration, col, err)
		}
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", colIdent.Quoted(), i))
		args = append(args, val)
		i++
	}

	whereClauses := make([]string, 0, len(where))
	for col, val := range where {
		colIdent, err := connection.NewIdentifier(col)
		if err != nil {
			return fmt.Errorf("%w: invalid where column name '%s': %v", connection.ErrInvalidConfiguration, col, err)
		}
		whereClauses = append(whereClauses, fmt.Sprintf("%s = $%d", colIdent.Quoted(), i))
		args = append(args, val)
		i++
	}

	query := fmt.Sprintf(
		`UPDATE "public".%s SET %s WHERE %s`,
		tableName.Quoted(),
		strings.Join(setClauses, ", "),
		strings.Join(whereClauses, " AND "),
	)

	result, err := db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("%w: updating row: %v", connection.ErrQueryFailed, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%w: checking rows affected: %v", connection.ErrQueryFailed, err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("%w: no row matched the WHERE clause", connection.ErrResourceNotFound)
	}

	return nil
}
```

- [ ] **Step 2: Verify the project compiles (Gateway now satisfies the interface)**

```bash
cd /home/felipemalacarne/repos/mesa && go build ./...
```

Expected: compile error about `Server does not implement ServerInterface` (REST handler still missing). The gateway error is gone.

- [ ] **Step 3: Commit**

```bash
git add internal/infrastructure/postgres/gateway.go
git commit -m "feat(postgres): implement UpdateTableRow in gateway"
```

---

### Task 5: Create the UpdateTableRow command handler

**Files:**
- Create: `internal/application/commands/update_table_row.go`

- [ ] **Step 1: Create the file**

```go
package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/felipemalacarne/mesa/internal/domain"
	"github.com/felipemalacarne/mesa/internal/domain/connection"
	"github.com/google/uuid"
)

type UpdateTableRowCmd struct {
	ConnectionID uuid.UUID
	DatabaseName connection.Identifier
	TableName    connection.Identifier
	Where        map[string]any
	Set          map[string]any
}

type UpdateTableRowHandler struct {
	repo     connection.Repository
	crypto   domain.Cryptographer
	gateways connection.GatewayFactory
}

func NewUpdateTableRowHandler(
	repo connection.Repository,
	crypto domain.Cryptographer,
	gateways connection.GatewayFactory,
) *UpdateTableRowHandler {
	return &UpdateTableRowHandler{repo: repo, crypto: crypto, gateways: gateways}
}

func (h *UpdateTableRowHandler) Handle(ctx context.Context, cmd UpdateTableRowCmd) error {
	if len(cmd.Where) == 0 {
		return fmt.Errorf("%w: where clause is required", ErrInvalidInput)
	}
	if len(cmd.Set) == 0 {
		return fmt.Errorf("%w: set clause is required", ErrInvalidInput)
	}

	conn, err := h.repo.FindByID(ctx, cmd.ConnectionID)
	if err != nil {
		return err
	}
	if conn == nil {
		return ErrConnectionNotFound
	}

	password, err := h.crypto.Decrypt(conn.Password)
	if err != nil {
		return err
	}

	gateway, err := h.gateways.ForDriver(conn.Driver)
	if err != nil {
		return err
	}

	timedCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	return gateway.UpdateTableRow(timedCtx, *conn, password, cmd.DatabaseName, cmd.TableName, cmd.Where, cmd.Set)
}
```

- [ ] **Step 2: Add `ErrInvalidInput` to the errors file if it doesn't exist**

Check `internal/application/commands/errors.go`:

```bash
cat internal/application/commands/errors.go
```

If `ErrInvalidInput` is not there, add it:

```go
var ErrInvalidInput = errors.New("invalid input")
```

- [ ] **Step 3: Verify the file compiles**

```bash
cd /home/felipemalacarne/repos/mesa && go build ./internal/application/commands/...
```

Expected: no errors.

- [ ] **Step 4: Commit**

```bash
git add internal/application/commands/update_table_row.go \
        internal/application/commands/errors.go
git commit -m "feat(commands): add UpdateTableRow command handler"
```

---

### Task 6: Register the command and wire the REST handler

**Files:**
- Modify: `internal/application/app.go`
- Modify: `internal/transport/rest/handlers.go`

- [ ] **Step 1: Add `UpdateTableRow` to the `Commands` struct in `app.go`**

In `internal/application/app.go`, add the field to `Commands`:

```go
type Commands struct {
	CreateConnection *commands.CreateConnectionHandler
	KillSession      *commands.KillSessionHandler
	CreateUser       *commands.CreateUserHandler
	CreateDatabase   *commands.CreateDatabaseHandler
	CreateTable      *commands.CreateTableHandler
	UpdateTableRow   *commands.UpdateTableRowHandler
}
```

And register it in `NewApp`:

```go
Commands: Commands{
    CreateConnection: commands.NewCreateConnectionHandler(repos.Connection, crypto),
    KillSession:      commands.NewKillSessionHandler(repos.Connection, crypto, repos.Gateways),
    CreateUser:       commands.NewCreateUserHandler(repos.Connection, crypto, repos.Gateways),
    CreateDatabase:   commands.NewCreateDatabaseHandler(repos.Connection, crypto, repos.Gateways),
    CreateTable:      commands.NewCreateTableHandler(repos.Connection, crypto, repos.Gateways),
    UpdateTableRow:   commands.NewUpdateTableRowHandler(repos.Connection, crypto, repos.Gateways),
},
```

- [ ] **Step 2: Add the `UpdateTableRow` REST handler to `handlers.go`**

At the end of `internal/transport/rest/handlers.go`, add:

```go
func (s *Server) UpdateTableRow(
	w http.ResponseWriter,
	r *http.Request,
	connectionID contract.ConnectionId,
	databaseName contract.DatabaseName,
	tableName contract.TableName,
) {
	var body contract.UpdateTableRowRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		s.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if len(body.Where) == 0 || len(body.Set) == 0 {
		s.respondError(w, http.StatusBadRequest, "where and set must be non-empty")
		return
	}

	dbName, err := connection.NewIdentifier(string(databaseName))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid database name")
		return
	}

	tblName, err := connection.NewIdentifier(string(tableName))
	if err != nil {
		s.respondError(w, http.StatusBadRequest, "invalid table name")
		return
	}

	cmd := commands.UpdateTableRowCmd{
		ConnectionID: uuid.UUID(connectionID),
		DatabaseName: dbName,
		TableName:    tblName,
		Where:        body.Where,
		Set:          body.Set,
	}

	if err := s.app.Commands.UpdateTableRow.Handle(r.Context(), cmd); err != nil {
		if errors.Is(err, commands.ErrConnectionNotFound) {
			s.respondError(w, http.StatusNotFound, ErrConnectionNotFound)
			return
		}
		log.Printf("WARN: updateTableRow %s/%s/%s: %v", connectionID, databaseName, tableName, err)
		s.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
```

- [ ] **Step 3: Verify the project builds cleanly**

```bash
cd /home/felipemalacarne/repos/mesa && go build ./...
```

Expected: **no errors**.

- [ ] **Step 4: Smoke-test the endpoint manually**

Start the server (`go run ./cmd/server/main.go`) and run a curl against a real connection. Example (replace UUIDs and values with real ones from your local DB):

```bash
curl -X PUT http://localhost:8080/api/connections/<uuid>/databases/mesa/tables/users/rows \
  -H "Content-Type: application/json" \
  -d '{"where":{"id":1},"set":{"name":"Test Updated"}}' \
  -w "\nHTTP %{http_code}\n"
```

Expected: `HTTP 204`

- [ ] **Step 5: Commit**

```bash
git add internal/application/app.go internal/transport/rest/handlers.go
git commit -m "feat(rest): wire UpdateTableRow command and REST handler"
```

---

## Chunk 2: Frontend

### Task 7: Add `--pending` design tokens

**Files:**
- Modify: `web/src/styles.css`

- [ ] **Step 1: Add `--pending` and `--pending-foreground` to `:root`**

In `web/src/styles.css`, inside the `:root { }` block (after the `--chart-5` line), add:

```css
  --pending: oklch(0.80 0.14 88);
  --pending-foreground: oklch(0.45 0.10 88);
```

- [ ] **Step 2: Add dark-mode overrides to `.dark`**

In the `.dark { }` block (after `--chart-5`), add:

```css
  --pending: oklch(0.75 0.14 88);
  --pending-foreground: oklch(0.95 0.05 88);
```

- [ ] **Step 3: Expose to Tailwind via `@theme inline`**

In the `@theme inline { }` block (after `--color-chart-5`), add:

```css
  --color-pending: var(--pending);
  --color-pending-foreground: var(--pending-foreground);
```

- [ ] **Step 4: Verify Tailwind picks up the new tokens**

```bash
cd /home/felipemalacarne/repos/mesa/web && pnpm build 2>&1 | tail -5
```

Expected: build completes with no errors. The classes `bg-pending`, `border-pending`, `text-pending-foreground` are now available.

- [ ] **Step 5: Commit**

```bash
git add web/src/styles.css
git commit -m "feat(ui): add --pending design token for row edit highlighting"
```

---

### Task 8: Add `getRowClassName` prop to DataTable

**Files:**
- Modify: `web/src/components/data-table.tsx`

- [ ] **Step 1: Add `getRowClassName` to the props interface**

In `web/src/components/data-table.tsx`, find the `DataTableProps` interface and add the optional prop:

```ts
interface DataTableProps<TData, TValue> {
  columns: ColumnDef<TData, TValue>[];
  data: TData[];
  isFetching?: boolean;
  pagination?: PaginationState;
  serverSorting?: SortingChangeState;
  getRowClassName?: (row: Row<TData>) => string;
}
```

Also add `Row` to the TanStack Table import at the top of the file:

```ts
import {
  type ColumnDef,
  type Row,
  type SortingState,
  flexRender,
  getCoreRowModel,
  getSortedRowModel,
  useReactTable,
} from "@tanstack/react-table";
```

- [ ] **Step 2: Destructure the new prop in the function signature**

```ts
export function DataTable<TData, TValue>({
  columns,
  data,
  isFetching = false,
  pagination,
  serverSorting,
  getRowClassName,
}: DataTableProps<TData, TValue>) {
```

- [ ] **Step 3: Apply `getRowClassName` to each `TableRow`**

Find the `TableRow` render inside `TableBody` and add the `className` prop:

```tsx
<TableRow
  key={row.id}
  data-state={row.getIsSelected() && "selected"}
  className={getRowClassName ? getRowClassName(row) : undefined}
>
```

- [ ] **Step 4: Verify the project builds**

```bash
cd /home/felipemalacarne/repos/mesa/web && pnpm build 2>&1 | tail -5
```

Expected: no TypeScript or build errors.

- [ ] **Step 5: Commit**

```bash
git add web/src/components/data-table.tsx
git commit -m "feat(ui): add getRowClassName prop to DataTable"
```

---

### Task 9: Create EditableDataTable component

**Files:**
- Create: `web/src/components/editable-data-table.tsx`

- [ ] **Step 1: Create the file with the full implementation**

```tsx
import { useState } from "react";
import type { ColumnDef, Row } from "@tanstack/react-table";
import { toast } from "sonner";

import { DataTable } from "@/components/data-table";
import type { PaginationState, SortingChangeState } from "@/components/data-table";
import { Button } from "@/components/ui/button";

export interface RowChange {
  where: Record<string, unknown>;
  set: Record<string, unknown>;
}

// Note: The spec defined `rowKey: (row) => string` and `onConfirmChanges: (changes[]) => Promise<void>`.
// The plan refines these to:
//   - `getPkValues` (returns the PK object, used both as the stable row key and as the WHERE clause)
//   - `onSaveRow` (per-row callback so EditableDataTable can remove rows from state as each succeeds)
//   - `onConfirmComplete` (called once after the confirm loop finishes, for query invalidation)
// This avoids each consumer re-implementing the sequential-stop-on-error loop.
interface EditableDataTableProps<TData, TValue> {
  columns: ColumnDef<TData, TValue>[];
  data: TData[];
  isFetching?: boolean;
  pagination?: PaginationState;
  serverSorting?: SortingChangeState;
  /** Column ids that support inline editing (non-PK columns). Empty = read-only mode. */
  editableColumns: string[];
  /** Returns the primary key values for a row, used as the stable row key and WHERE clause. */
  getPkValues: (row: TData) => Record<string, unknown>;
  /** Called once per dirty row when the user confirms. Throw to signal failure. */
  onSaveRow: (change: RowChange) => Promise<void>;
  /** Called once after the confirm loop finishes (success or partial failure). Use for query invalidation. */
  onConfirmComplete?: () => void;
}

export function EditableDataTable<TData, TValue>({
  columns,
  data,
  isFetching,
  pagination,
  serverSorting,
  editableColumns,
  getPkValues,
  onSaveRow,
  onConfirmComplete,
}: EditableDataTableProps<TData, TValue>) {
  // rowKey (JSON of PK values) → { colId → newValue }
  const [pendingChanges, setPendingChanges] = useState<
    Map<string, Record<string, unknown>>
  >(new Map());
  const [activeCell, setActiveCell] = useState<{
    rowKey: string;
    col: string;
  } | null>(null);
  const [isConfirming, setIsConfirming] = useState(false);

  function rowKey(row: TData): string {
    return JSON.stringify(getPkValues(row));
  }

  function commitEdit(
    rKey: string,
    colId: string,
    originalValue: unknown,
    newValue: string,
  ) {
    setPendingChanges((prev) => {
      const next = new Map(prev);
      const rowChanges = { ...(next.get(rKey) ?? {}) };

      if (String(originalValue ?? "") === newValue) {
        delete rowChanges[colId];
      } else {
        rowChanges[colId] = newValue;
      }

      if (Object.keys(rowChanges).length === 0) {
        next.delete(rKey);
      } else {
        next.set(rKey, rowChanges);
      }
      return next;
    });
  }

  const wrappedColumns: ColumnDef<TData, TValue>[] = columns.map((col) => {
    if (!col.id || !editableColumns.includes(col.id)) return col;

    return {
      ...col,
      cell: ({ row, getValue }) => {
        const rKey = rowKey(row.original);
        const colId = col.id!;
        const originalValue = getValue();
        const pending = pendingChanges.get(rKey);
        const hasPendingValue = pending !== undefined && colId in pending;
        const displayValue = hasPendingValue ? pending![colId] : originalValue;
        const isDirty = hasPendingValue;
        const isActive =
          activeCell?.rowKey === rKey && activeCell?.col === colId;

        if (isActive) {
          return (
            <input
              autoFocus
              defaultValue={
                displayValue === null || displayValue === undefined
                  ? ""
                  : String(displayValue)
              }
              className="border border-ring rounded px-1.5 py-0.5 text-sm w-full outline-none ring-2 ring-ring/30 bg-background font-mono"
              onKeyDown={(e) => {
                if (e.key === "Escape") {
                  e.preventDefault();
                  setActiveCell(null);
                }
                if (e.key === "Enter") {
                  e.preventDefault();
                  commitEdit(rKey, colId, originalValue, e.currentTarget.value);
                  setActiveCell(null);
                }
              }}
              onBlur={(e) => {
                commitEdit(rKey, colId, originalValue, e.currentTarget.value);
                setActiveCell(null);
              }}
            />
          );
        }

        return (
          <span
            className={`cursor-default select-none ${isDirty ? "text-pending-foreground font-medium" : ""}`}
            onDoubleClick={() =>
              setActiveCell({ rowKey: rKey, col: colId })
            }
          >
            {displayValue === null || displayValue === undefined ? (
              <span className="text-muted-foreground italic">null</span>
            ) : (
              String(displayValue)
            )}
          </span>
        );
      },
    };
  });

  const getRowClassName = (row: Row<TData>) =>
    pendingChanges.has(rowKey(row.original))
      ? "bg-pending/10 border-l-2 border-pending"
      : "";

  async function handleConfirm() {
    setIsConfirming(true);
    try {
      for (const [rKey, colChanges] of pendingChanges) {
        const where = JSON.parse(rKey) as Record<string, unknown>;
        try {
          await onSaveRow({ where, set: colChanges });
          setPendingChanges((prev) => {
            const next = new Map(prev);
            next.delete(rKey);
            return next;
          });
        } catch (err) {
          toast.error(
            `Failed to save row: ${err instanceof Error ? err.message : "Unknown error"}`,
          );
          return;
        }
      }
    } finally {
      setIsConfirming(false);
      onConfirmComplete?.(); // always called after loop — triggers query invalidation in parent
    }
  }

  function handleDiscard() {
    setPendingChanges(new Map());
    setActiveCell(null);
  }

  const readOnly = editableColumns.length === 0;

  return (
    <div className="space-y-3">
      {readOnly && (
        <p className="text-sm text-muted-foreground">
          This table has no primary key — editing is disabled.
        </p>
      )}
      <DataTable
        columns={wrappedColumns}
        data={data}
        isFetching={isFetching}
        pagination={pagination}
        serverSorting={serverSorting}
        getRowClassName={readOnly ? undefined : getRowClassName}
      />
      {pendingChanges.size > 0 && (
        <div className="flex items-center justify-between rounded-md border border-pending bg-pending/10 px-3 py-2">
          <span className="text-sm font-medium text-pending-foreground">
            {pendingChanges.size === 1
              ? "1 row with pending changes"
              : `${pendingChanges.size} rows with pending changes`}
          </span>
          <div className="flex gap-2">
            <Button
              variant="outline"
              size="sm"
              onClick={handleDiscard}
              disabled={isConfirming}
            >
              Discard
            </Button>
            <Button
              size="sm"
              onClick={handleConfirm}
              disabled={isConfirming}
            >
              {isConfirming ? "Saving…" : "Confirm changes"}
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
```

- [ ] **Step 2: Verify TypeScript is happy**

```bash
cd /home/felipemalacarne/repos/mesa/web && pnpm tsc --noEmit 2>&1 | head -20
```

Expected: no errors from `editable-data-table.tsx`.

- [ ] **Step 3: Commit**

```bash
git add web/src/components/editable-data-table.tsx
git commit -m "feat(ui): add EditableDataTable component with inline editing"
```

---

### Task 10: Create RowsTab (replaces SampleTab)

**Files:**
- Create: `web/src/routes/connections/$connectionId/databases/$databaseName/tables/_components/rows-tab.tsx`
- Delete: `web/src/routes/connections/$connectionId/databases/$databaseName/tables/_components/sample-tab.tsx`

The full path (abbreviated as `.../tables/_components/` below) is:
`web/src/routes/connections/$connectionId/databases/$databaseName/tables/_components/`

- [ ] **Step 1: Verify generated hook names exist**

Run these after `make codegen` (Task 2) has completed:

```bash
grep -n "export.*useListColumns\|export.*useUpdateTableRow\|export.*getQueryTableRowsQueryKey\|export.*useQueryTableRows" \
  web/src/api/connections/connections.ts
```

Expected: all four names appear. If orval generated different names (e.g. `useConnectionsUpdateTableRow`), update the imports in Step 2 accordingly before writing the file.

- [ ] **Step 2: Create `rows-tab.tsx`**

```tsx
import { useState } from "react";
import type { ColumnDef } from "@tanstack/react-table";
import { useQueryClient } from "@tanstack/react-query";

import {
  useQueryTableRows,
  getQueryTableRowsQueryKey,
  useListColumns,
  useUpdateTableRow,
} from "@/api/connections/connections";
import type { QueryTableRowsSortOrder } from "@/api/mesaAPI.schemas";
import { EditableDataTable } from "@/components/editable-data-table";
import type { RowChange } from "@/components/editable-data-table";
import { Skeleton } from "@/components/ui/skeleton";

const PAGE_SIZE = 50;

interface RowsTabProps {
  connectionId: string;
  databaseName: string;
  tableName: string;
}

export function RowsTab({ connectionId, databaseName, tableName }: RowsTabProps) {
  const [page, setPage] = useState(0);
  const [sortBy, setSortBy] = useState<string | null>(null);
  const [sortOrder, setSortOrder] = useState<"asc" | "desc">("asc");
  const queryClient = useQueryClient();

  const { data, isLoading, isFetching, isError } = useQueryTableRows(
    connectionId,
    databaseName,
    tableName,
    {
      limit: PAGE_SIZE,
      offset: page * PAGE_SIZE,
      ...(sortBy ? { sort_by: sortBy, sort_order: sortOrder as QueryTableRowsSortOrder } : {}),
    },
    { query: { staleTime: 30_000, placeholderData: (prev) => prev } },
  );

  const { data: columnMetadata } = useListColumns(connectionId, databaseName, tableName, {
    query: { staleTime: 60_000 },
  });

  const updateTableRow = useUpdateTableRow();

  if (isLoading) {
    return (
      <div className="animate-in fade-in-0 duration-300 delay-150 space-y-2">
        {Array.from({ length: 8 }).map((_, i) => (
          <Skeleton key={i} className="h-10 w-full" />
        ))}
      </div>
    );
  }

  if (isError) {
    return <p className="text-sm text-destructive">Failed to load table data.</p>;
  }

  const columns = data?.columns ?? [];
  const rows = data?.rows ?? [];
  const total = data?.total ?? 0;

  const pkColumns = (columnMetadata ?? [])
    .filter((col) => col.primary)
    .map((col) => col.name);

  const editableColumns = (columnMetadata ?? [])
    .filter((col) => !col.primary)
    .map((col) => col.name);

  const columnDefs: ColumnDef<unknown[], unknown>[] = columns.map((col) => ({
    id: col,
    accessorFn: (row: unknown[]) => row[columns.indexOf(col)],
    header: col,
    enableSorting: true,
    cell: ({ getValue }) => {
      const value = getValue();
      if (value === null || value === undefined) {
        return <span className="text-muted-foreground italic">null</span>;
      }
      return String(value);
    },
  }));

  function getPkValues(row: unknown[]): Record<string, unknown> {
    return Object.fromEntries(
      pkColumns.map((col) => [col, row[columns.indexOf(col)]]),
    );
  }

  async function onSaveRow(change: RowChange) {
    await updateTableRow.mutateAsync({
      connectionId,
      databaseName,
      tableName,
      data: change,
    });
  }

  function onConfirmComplete() {
    queryClient.invalidateQueries({
      queryKey: getQueryTableRowsQueryKey(connectionId, databaseName, tableName),
    });
  }

  return (
    <EditableDataTable
      columns={columnDefs}
      data={rows as unknown[][]}
      isFetching={isFetching}
      pagination={{
        pageIndex: page,
        pageSize: PAGE_SIZE,
        total,
        onPageChange: setPage,
      }}
      serverSorting={{
        sortBy,
        sortOrder,
        onSortChange: (col, order) => {
          setSortBy(col);
          setSortOrder(order);
          setPage(0);
        },
      }}
      editableColumns={editableColumns}
      getPkValues={getPkValues}
      onSaveRow={onSaveRow}
      onConfirmComplete={onConfirmComplete}
    />
  );
}
```

- [ ] **Step 3: Delete `sample-tab.tsx`**

```bash
rm web/src/routes/connections/\$connectionId/databases/\$databaseName/tables/_components/sample-tab.tsx
```

- [ ] **Step 4: Verify TypeScript**


```bash
cd /home/felipemalacarne/repos/mesa/web && pnpm tsc --noEmit 2>&1 | grep -v "node_modules" | head -20
```

Expected: no errors from `rows-tab.tsx` or `editable-data-table.tsx`. (Errors in `$tableName.tsx` are expected — not updated yet.)

- [ ] **Step 5: Commit**

```bash
git add "web/src/routes/connections/\$connectionId/databases/\$databaseName/tables/_components/rows-tab.tsx"
git rm "web/src/routes/connections/\$connectionId/databases/\$databaseName/tables/_components/sample-tab.tsx"
git commit -m "feat(ui): add RowsTab with EditableDataTable, remove SampleTab"
```

---

### Task 11: Update the parent route file

**Files:**
- Modify: `web/src/routes/connections/$connectionId/databases/$databaseName/tables/$tableName.tsx`

- [ ] **Step 1: Replace the SampleTab import with RowsTab**

Change:
```tsx
import { SampleTab } from "./_components/sample-tab";
```
To:
```tsx
import { RowsTab } from "./_components/rows-tab";
```

- [ ] **Step 2: Update the tab trigger label and value**

Change:
```tsx
<TabsTrigger value="sample">Sample data</TabsTrigger>
```
To:
```tsx
<TabsTrigger value="rows">Rows</TabsTrigger>
```

- [ ] **Step 3: Update the tab content value and component**

Change:
```tsx
<TabsContent value="sample" className="mt-4">
  <SampleTab
    connectionId={connectionId}
    databaseName={databaseName}
    tableName={tableName}
  />
</TabsContent>
```
To:
```tsx
<TabsContent value="rows" className="mt-4">
  <RowsTab
    connectionId={connectionId}
    databaseName={databaseName}
    tableName={tableName}
  />
</TabsContent>
```

- [ ] **Step 4: Verify the full frontend builds with no errors**

```bash
cd /home/felipemalacarne/repos/mesa/web && pnpm build 2>&1 | tail -10
```

Expected: build completes successfully with no TypeScript errors.

- [ ] **Step 5: Commit**

```bash
git add "web/src/routes/connections/\$connectionId/databases/\$databaseName/tables/\$tableName.tsx"
git commit -m "feat(ui): rename tab to Rows, wire RowsTab in table route"
```

---

### Task 12: End-to-end verification

- [ ] **Step 1: Start the backend**

```bash
cd /home/felipemalacarne/repos/mesa && go run ./cmd/server/main.go
```

- [ ] **Step 2: Start the frontend dev server**

```bash
cd /home/felipemalacarne/repos/mesa/web && pnpm dev
```

- [ ] **Step 3: Manual walkthrough**

1. Open the app, navigate to a table that has a primary key
2. Click the **Rows** tab — verify it loads data (was "Sample data")
3. Double-click a non-PK cell — verify an input appears
4. Type a new value, press **Enter** — verify the row turns yellow with a pending indicator
5. Double-click another cell in a different row, edit it — verify the action bar shows "2 rows with pending changes"
6. Click **Discard** — verify all rows return to their original appearance
7. Repeat edits, then click **Confirm changes** — verify `HTTP 204` is returned, rows refresh, and yellow highlighting clears
8. Navigate to a table without a primary key — verify the "editing is disabled" notice appears

- [ ] **Step 4: Verify no uncommitted changes remain**

```bash
cd /home/felipemalacarne/repos/mesa && git status
```

Expected: clean working tree. All files were committed in Tasks 1–11. If any stray changes remain, stage them explicitly by path before committing — do not use `git add -A`.
