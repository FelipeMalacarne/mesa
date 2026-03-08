import { useState } from "react";
import type { ColumnDef } from "@tanstack/react-table";

import { useQueryTableRows } from "@/api/connections/connections";
import type { QueryTableRowsSortOrder } from "@/api/mesaAPI.schemas";
import { DataTable } from "@/components/data-table";
import { Skeleton } from "@/components/ui/skeleton";

const PAGE_SIZE = 50;

interface SampleTabProps {
  connectionId: string;
  databaseName: string;
  tableName: string;
}

export function SampleTab({ connectionId, databaseName, tableName }: SampleTabProps) {
  const [page, setPage] = useState(0);
  const [sortBy, setSortBy] = useState<string | null>(null);
  const [sortOrder, setSortOrder] = useState<"asc" | "desc">("asc");

  const { data, isLoading, isError } = useQueryTableRows(
    connectionId,
    databaseName,
    tableName,
    {
      limit: PAGE_SIZE,
      offset: page * PAGE_SIZE,
      ...(sortBy ? { sort_by: sortBy, sort_order: sortOrder as QueryTableRowsSortOrder } : {}),
    },
    { query: { staleTime: 30_000 } },
  );

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

  return (
    <DataTable
      columns={columnDefs}
      data={rows as unknown[][]}
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
    />
  );
}
