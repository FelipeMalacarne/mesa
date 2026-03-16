# Rows Tab — Inline Editing

**Date:** 2026-03-15
**Status:** Approved

## Overview

Rename the "Sample data" tab to "Rows" and add inline cell editing with pending-change highlighting and a confirm/discard flow. Editing is backed by a new backend UPDATE endpoint.

## Rename

| Before | After |
|---|---|
| `sample-tab.tsx` | `rows-tab.tsx` |
| `SampleTab` component | `RowsTab` component |
| Tab label "Sample data" | Tab label "Rows" |
| Tab value `"sample"` | Tab value `"rows"` |

## Components

### `EditableDataTable` (`web/src/components/editable-data-table.tsx`)

A wrapper around `DataTable` that adds inline editing capability. Designed to be reusable (e.g., future Indexes tab editing).

**Props:**
```ts
interface EditableDataTableProps<TData, TValue> {
  columns: ColumnDef<TData, TValue>[];
  data: TData[];
  isFetching?: boolean;
  pagination?: PaginationState;
  serverSorting?: SortingChangeState;
  editableColumns: string[];           // column names that can be edited
  rowKey: (row: TData) => string;      // produces a stable key (JSON of PK values)
  onConfirmChanges: (changes: RowChange[]) => Promise<void>;
}

interface RowChange {
  where: Record<string, unknown>;   // PK column(s) → original value(s)
  set: Record<string, unknown>;     // changed column(s) → new value(s)
}
```

**Internal state:**
```ts
// rowKey → { colName → newValue }
pendingChanges: Map<string, Record<string, unknown>>
// rowKey + colName → currently active input
activeCell: { rowKey: string; col: string } | null
```

**Behaviour:**
- Double-click a non-PK cell → sets `activeCell`, renders an `<input>` in place
- Enter or blur on input → commits value to `pendingChanges` if changed from original; clears `activeCell`
- Escape on input → cancels edit without committing; clears `activeCell`
- Rows with entries in `pendingChanges` receive `bg-yellow-50 border-l-2 border-yellow-400`; changed cells render their value in amber (`text-amber-800 font-medium`)
- Action bar appears below the table only when `pendingChanges.size > 0`:
  - Left: "N rows with pending changes"
  - Right: "Discard" button (clears `pendingChanges`) + "Confirm changes" button (calls `onConfirmChanges`, then clears state on success)
- During confirmation, buttons are disabled and show a loading state

**`DataTable` addition:**
Add a single optional prop `getRowClassName?: (row: Row<TData>) => string` to `DataTable`. `EditableDataTable` passes this to apply yellow highlighting.

### `RowsTab` (`web/src/routes/.../tables/_components/rows-tab.tsx`)

Replaces `SampleTab`. Uses `EditableDataTable` instead of `DataTable`.

Additional data fetch: `useListColumns` to determine which columns have `primary: true` — these are excluded from `editableColumns` and rendered as muted, non-interactive cells.

`rowKey` is built from primary key column values: `JSON.stringify({ id: row[pkIndex] })`.

`onConfirmChanges` fires one `useUpdateTableRow` mutation per dirty row sequentially, then calls `queryClient.invalidateQueries` on the rows query key. On any failure, it surfaces an error toast and keeps the pending changes intact.

## Backend

### New endpoint

```
PUT /api/connections/:connectionId/databases/:databaseName/tables/:tableName/rows
```

**Request body:**
```json
{
  "where": { "id": 1 },
  "set":   { "name": "Alice Johnson" }
}
```

**Response:** `204 No Content` on success, `400` on bad input, `500` on query failure.

### Implementation

1. **`UpdateTableRowRequest`** schema added to `mesaAPI.schemas.ts` (and OpenAPI spec)
2. **`UpdateTableRow` command** in `internal/application/commands/update_table_row.go`
   - Builds a parameterized `UPDATE table SET col=$1 WHERE pk=$2` query
   - Validates that `where` and `set` are non-empty
3. **`Administrator` interface** in `gateway.go` gains `UpdateTableRow` method
4. **Postgres gateway** implements `UpdateTableRow`
5. **REST handler** `UpdateTableRow` added to `handlers.go`
6. **OpenAPI spec** updated; orval re-run to regenerate `connections.ts` and `mesaAPI.schemas.ts`

## Error Handling

| Scenario | Behaviour |
|---|---|
| Escape during active cell edit | Cancels that cell's edit, no pending change recorded |
| User edits cell back to original value | Entry removed from `pendingChanges` (row de-highlights) |
| Confirm fails for one row | Toast shown, pending changes preserved for retry |
| No primary key columns found | `editableColumns` is empty; table renders as read-only with a notice |

## Out of Scope

- Deleting rows
- Adding new rows
- Editing cells that contain JSON/array values (render as read-only string for now)
- Optimistic UI updates (query is invalidated after confirmation, not before)
