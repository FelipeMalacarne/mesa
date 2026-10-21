import { createFileRoute } from "@tanstack/react-router";

import { TablePage } from "@/features/table/TablePage";
import type { TableView } from "@/features/explorer/tabs.store";

interface TableSearch {
  view: TableView;
  kind: "table" | "view";
}

export const Route = createFileRoute("/c/$connectionId/db/$database/t/$schema/$table")({
  validateSearch: (s: Record<string, unknown>): TableSearch => ({
    view: s.view === "structure" || s.view === "indexes" ? s.view : "data",
    kind: s.kind === "view" ? "view" : "table",
  }),
  component: function TableRoute() {
    const { connectionId, database, schema, table } = Route.useParams();
    const { view, kind } = Route.useSearch();
    return <TablePage key={`${connectionId}/${database}/${schema}.${table}`} database={database} schema={schema} table={table} view={view} kind={kind} />;
  },
});
