import {
  type ColumnDef,
  type Row,
  type SortingState,
  flexRender,
  getCoreRowModel,
  getSortedRowModel,
  useReactTable,
} from "@tanstack/react-table";
import { ArrowDown, ArrowUp, ArrowUpDown } from "lucide-react";
import { useState } from "react";

import { Button } from "@/components/ui/button";
import {
  Pagination,
  PaginationContent,
  PaginationItem,
  PaginationNext,
  PaginationPrevious,
} from "@/components/ui/pagination";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

export interface PaginationState {
  pageIndex: number;
  pageSize: number;
  total: number;
  onPageChange: (page: number) => void;
}

export interface SortingChangeState {
  sortBy: string | null;
  sortOrder: "asc" | "desc";
  onSortChange: (sortBy: string | null, sortOrder: "asc" | "desc") => void;
}

interface DataTableProps<TData, TValue> {
  columns: ColumnDef<TData, TValue>[];
  data: TData[];
  isFetching?: boolean;
  pagination?: PaginationState;
  serverSorting?: SortingChangeState;
  getRowClassName?: (row: Row<TData>) => string;
}

function SortIcon({ state }: { state: "asc" | "desc" | false }) {
  if (state === "asc") return <ArrowUp className="ml-1 inline h-3 w-3" />;
  if (state === "desc") return <ArrowDown className="ml-1 inline h-3 w-3" />;
  return <ArrowUpDown className="ml-1 inline h-3 w-3 opacity-40" />;
}

export function DataTable<TData, TValue>({
  columns,
  data,
  isFetching = false,
  pagination,
  serverSorting,
  getRowClassName,
}: DataTableProps<TData, TValue>) {
  const [sorting, setSorting] = useState<SortingState>([]);

  const table = useReactTable({
    data,
    columns,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
    manualSorting: !!serverSorting,
    state: { sorting },
    onSortingChange: (updater) => {
      const next = typeof updater === "function" ? updater(sorting) : updater;
      setSorting(next);
      if (serverSorting) {
        const first = next[0];
        serverSorting.onSortChange(
          first ? (first.id as string) : null,
          first ? (first.desc ? "desc" : "asc") : "asc",
        );
      }
    },
  });

  const totalPages = pagination
    ? Math.ceil(pagination.total / pagination.pageSize)
    : null;

  return (
    <div className="space-y-3">
      <div className="rounded-md border overflow-hidden">
        {/* Single scroll container — neutralise Table's inner overflow-x-auto so
            horizontal scroll is contained here, not on the page */}
        <div
          className={`max-h-[480px] overflow-auto [&_[data-slot=table-container]]:overflow-visible transition-opacity duration-150 ${isFetching ? "opacity-60 pointer-events-none" : "opacity-100"}`}
        >
          <Table>
            <TableHeader className="sticky top-0 z-10 bg-background shadow-[0_1px_0_0_hsl(var(--border))]">
              {table.getHeaderGroups().map((headerGroup) => (
                <TableRow key={headerGroup.id}>
                  {headerGroup.headers.map((header) => {
                    const canSort = header.column.getCanSort();
                    const sorted = header.column.getIsSorted();
                    return (
                      <TableHead key={header.id} className="whitespace-nowrap">
                        {header.isPlaceholder ? null : canSort ? (
                          <Button
                            variant="ghost"
                            size="xs"
                            className="-ml-2 font-medium text-foreground"
                            onClick={header.column.getToggleSortingHandler()}
                          >
                            {flexRender(
                              header.column.columnDef.header,
                              header.getContext(),
                            )}
                            <SortIcon state={sorted} />
                          </Button>
                        ) : (
                          flexRender(
                            header.column.columnDef.header,
                            header.getContext(),
                          )
                        )}
                      </TableHead>
                    );
                  })}
                </TableRow>
              ))}
            </TableHeader>
            <TableBody>
              {table.getRowModel().rows?.length ? (
                table.getRowModel().rows.map((row) => (
                  <TableRow
                    key={row.id}
                    data-state={row.getIsSelected() && "selected"}
                    className={getRowClassName ? getRowClassName(row) : undefined}
                  >
                    {row.getVisibleCells().map((cell) => (
                      <TableCell key={cell.id} className="whitespace-nowrap">
                        {flexRender(
                          cell.column.columnDef.cell,
                          cell.getContext(),
                        )}
                      </TableCell>
                    ))}
                  </TableRow>
                ))
              ) : (
                <TableRow>
                  <TableCell
                    colSpan={columns.length}
                    className="h-24 text-center"
                  >
                    No results.
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </div>
      </div>

      {pagination && totalPages !== null && totalPages > 1 && (
        <div className="flex items-center justify-between text-sm text-muted-foreground">
          <span>
            Showing {pagination.pageIndex * pagination.pageSize + 1}–
            {Math.min(
              (pagination.pageIndex + 1) * pagination.pageSize,
              pagination.total,
            )}{" "}
            of {pagination.total}
          </span>
          <Pagination className="w-auto">
            <PaginationContent>
              <PaginationItem>
                <PaginationPrevious
                  onClick={() =>
                    pagination.pageIndex > 0 &&
                    pagination.onPageChange(pagination.pageIndex - 1)
                  }
                  aria-disabled={pagination.pageIndex === 0}
                  className={
                    pagination.pageIndex === 0
                      ? "pointer-events-none opacity-50"
                      : "cursor-pointer"
                  }
                />
              </PaginationItem>
              <PaginationItem>
                <PaginationNext
                  onClick={() =>
                    pagination.pageIndex < totalPages - 1 &&
                    pagination.onPageChange(pagination.pageIndex + 1)
                  }
                  aria-disabled={pagination.pageIndex >= totalPages - 1}
                  className={
                    pagination.pageIndex >= totalPages - 1
                      ? "pointer-events-none opacity-50"
                      : "cursor-pointer"
                  }
                />
              </PaginationItem>
            </PaginationContent>
          </Pagination>
        </div>
      )}
    </div>
  );
}
