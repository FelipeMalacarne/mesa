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
