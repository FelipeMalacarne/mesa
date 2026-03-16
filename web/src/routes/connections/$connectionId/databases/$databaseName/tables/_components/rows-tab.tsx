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
      connectionID: connectionId,
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
