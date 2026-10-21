/**
 * A table or view tab: Data (editable grid), Structure and Indexes, with staged changes that
 * go through the review dialog (`Main`, `Structure` and `Commit` boards).
 */
import * as React from "react";
import { keepPreviousData, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { Code2, Columns3, Database as DatabaseIcon, Hash, Info, PanelRight, Plus, RefreshCw, Rows3, Trash2 } from "lucide-react";
import { toast } from "sonner";

import { applyChanges, planChanges } from "@/lib/api/changes/changes";
import { getGetSchemaQueryKey, getGetTableQueryKey, queryRows, useGetSchema, useGetTable } from "@/lib/api/explore/explore";
import type { ChangeSet, RowsQueryFiltersItem } from "@/lib/api/mesaAPI.schemas";
import { errorMessage } from "@/lib/api/fetch";
import { bytes, duration, nsToMs, plural, thousands } from "@/lib/format";
import { ValueInputError } from "@/lib/values";
import { cn, isCmd } from "@/lib/utils";
import { usePendingStatus, useStatusFacts } from "@/app/status.store";
import { useUi } from "@/app/ui.store";
import { Button } from "@/ui/Button";
import { CodeBlock } from "@/ui/Code";
import { EmptyState } from "@/ui/controls";
import { Dialog, DialogBody, DialogContent, DialogFoot, DialogHead, DialogTitle } from "@/ui/primitives/dialog";
import { ReviewDialog } from "@/features/changes/ReviewDialog";
import { tableTabId, useTabs, type TableView } from "@/features/explorer/tabs.store";

import { useWorkspace } from "../workspace/context";
import { countsOf, rowKeyOf, summaryOf, tableKey as makeTableKey, toChangeSet, useChanges, useTableChanges } from "./changes.store";
import { DataGrid, gridRows, type SortState } from "./DataGrid";
import { FilterBar, toFilters, type FilterDraft } from "./FilterBar";
import { Inspector } from "./Inspector";
import { editCount, toSchemaChanges, useSchemaEdits, useTableEdits } from "./schemaEdits.store";
import { openStructureForm, StructureView, structureFacts } from "./StructureView";

const PAGE = 100;

interface Props {
  database: string;
  schema: string;
  table: string;
  view: TableView;
  kind: "table" | "view";
}

export function TablePage({ database, schema, table, view, kind }: Props) {
  const { connectionId, connection } = useWorkspace();
  const navigate = useNavigate();
  const qc = useQueryClient();
  const q = React.useMemo(() => ({ schema, name: table }), [schema, table]);

  React.useEffect(() => {
    useTabs.getState().open(connectionId, { id: tableTabId(database, schema, table), kind: "table", title: table, database, schema, table, objectKind: kind, view });
  }, [connectionId, database, schema, table, kind, view]);

  const meta = useGetTable(connectionId, database, schema, table, { query: { retry: false, enabled: kind === "table" } });
  const model = useGetSchema(connectionId, database, undefined, { query: { staleTime: 30_000 } });
  const viewMeta = kind === "view" ? model.data?.schemas.find((s) => s.name === schema)?.views?.find((v) => v.name === table) : undefined;

  const [page, setPage] = React.useState(1);
  const [sort, setSort] = React.useState<SortState | null>(null);
  const [drafts, setDrafts] = React.useState<FilterDraft[]>([]);
  const [filters, setFilters] = React.useState<RowsQueryFiltersItem[]>([]);
  const [selectedRow, setSelectedRow] = React.useState<string | null>(null);
  const inspectorOpen = useUi((s) => s.inspectorOpen);
  const toggleInspector = useUi((s) => s.toggleInspector);
  const [review, setReview] = React.useState<"rows" | "schema" | null>(null);
  const [ddl, setDdl] = React.useState(false);

  // Without an explicit sort, rows come in primary-key order so an edited row keeps its place.
  const pkSort = React.useMemo(() => (meta.data?.primaryKey?.columns ?? []).map((column) => ({ column, desc: false })), [meta.data]);
  const effectiveSort = sort ? [sort] : pkSort;
  const metaSettled = kind === "view" || meta.isSuccess || meta.isError;
  const rows = useQuery({
    queryKey: ["rows", connectionId, database, schema, table, filters, effectiveSort, page],
    queryFn: () => queryRows(connectionId, database, { table: q, filters, sort: effectiveSort, limit: PAGE, offset: (page - 1) * PAGE }),
    placeholderData: keepPreviousData,
    enabled: view === "data" && metaSettled,
    retry: false,
  });

  const tk = makeTableKey(connectionId, database, schema, table);
  const changes = useTableChanges(tk);
  const discardRows = useChanges((s) => s.discard);
  const stageInsert = useChanges((s) => s.stageInsert);
  const toggleDelete = useChanges((s) => s.toggleDelete);
  const removeInsert = useChanges((s) => s.removeInsert);
  const edits = useTableEdits(tk);
  const discardEdits = useSchemaEdits((s) => s.discard);

  const hasPk = !!meta.data?.primaryKey?.columns.length;
  const editable = kind === "table" && hasPk;
  const columns = rows.data?.result.columns ?? [];
  const total = rows.data?.total ?? 0;
  const pages = Math.max(1, Math.ceil(total / PAGE));
  const counts = countsOf(changes);
  const structureCount = editCount(edits);

  const list = React.useMemo(() => (rows.data ? gridRows(columns, rows.data.result.rows, meta.data, changes) : []), [rows.data, columns, meta.data, changes]);
  const selected = list.find((r) => r.id === selectedRow) ?? null;

  const tableModel = meta.data;
  const facts =
    view === "data"
      ? rows.data
        ? [`${thousands(total)} rows`, duration(nsToMs(rows.data.result.duration))]
        : []
      : tableModel
        ? structureFacts(tableModel)
        : [];
  useStatusFacts(facts, view === "data" ? { page, pages, onPrev: () => setPage((p) => Math.max(1, p - 1)), onNext: () => setPage((p) => Math.min(pages, p + 1)) } : null);

  const pendingRows = view === "data" && counts.total > 0;
  const pendingSchema = view !== "data" && structureCount > 0;
  usePendingStatus(
    pendingRows
      ? { summary: summaryOf(counts), onDiscard: () => discardRows(tk), onReview: () => setReview("rows") }
      : pendingSchema
        ? { summary: plural(structureCount, "structure change"), onDiscard: () => discardEdits(tk), onReview: () => setReview("schema") }
        : null,
  );

  React.useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (isCmd(e) && e.key.toLowerCase() === "s") {
        e.preventDefault();
        if (pendingRows) setReview("rows");
        else if (pendingSchema) setReview("schema");
      } else if (isCmd(e) && e.key.toLowerCase() === "i") {
        e.preventDefault();
        toggleInspector();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [pendingRows, pendingSchema, toggleInspector]);

  const setView = (v: TableView) => void navigate({ to: "/c/$connectionId/db/$database/t/$schema/$table", params: { connectionId, database, schema, table }, search: { view: v, kind } });

  const applyFilters = () => {
    try {
      setFilters(toFilters(drafts, columns));
      setPage(1);
    } catch (e) {
      toast.error(e instanceof ValueInputError ? e.message : errorMessage(e));
    }
  };

  const refresh = async () => {
    await qc.invalidateQueries({ queryKey: ["rows", connectionId, database, schema, table] });
    await qc.invalidateQueries({ queryKey: getGetTableQueryKey(connectionId, database, schema, table) });
  };

  const afterApply = async () => {
    await qc.invalidateQueries({ queryKey: ["rows", connectionId, database, schema, table] });
    await qc.invalidateQueries({ queryKey: getGetTableQueryKey(connectionId, database, schema, table) });
    await qc.invalidateQueries({ queryKey: getGetSchemaQueryKey(connectionId, database) });
  };

  const rowChangeSet = (): ChangeSet => toChangeSet(changes, database, q);
  const schemaChangeSet = (): ChangeSet => ({ database, schema: tableModel ? toSchemaChanges(edits, q, tableModel) : [] });

  const segs: { v: TableView; label: string; icon: React.ElementType }[] = [
    { v: "data", label: "Data", icon: Rows3 },
    { v: "structure", label: "Structure", icon: Columns3 },
    { v: "indexes", label: "Indexes", icon: Hash },
  ];

  const errorState = (err: unknown) => (
    <div style={{ padding: 24 }}>
      <EmptyState icon={<DatabaseIcon className="m-icon" aria-hidden="true" />} title={`Cannot open ${schema}.${table}`} description={errorMessage(err)} />
    </div>
  );

  return (
    <>
      <div className={cn("m-toolbar", view === "data" && drafts.length > 1 && "m-toolbar--wrap")} style={{ minHeight: "var(--size-toolbar)", flex: "none" }}>
        <nav className="m-seg" aria-label="Table view">
          {segs
            .filter((s) => kind === "table" || s.v === "data" || s.v === "structure")
            .map((s) => (
              <button key={s.v} type="button" className={cn("m-seg__item", view === s.v && "is-active")} aria-current={view === s.v ? "page" : undefined} onClick={() => setView(s.v)}>
                <s.icon className="m-icon m-icon--sm" aria-hidden="true" />
                {s.label}
              </button>
            ))}
        </nav>
        <span className="m-sep" style={{ height: 20, alignSelf: "center" }} />
        {view === "data" ? (
          <>
            {columns.length > 0 ? <FilterBar columns={columns} drafts={drafts} onChange={setDrafts} onApply={applyFilters} applied={filters.length > 0} /> : <span className="m-help">Loading…</span>}
            <span className="m-spacer" />
            <Button variant="ghost" size="sm" icon aria-label="Refresh" title="Refresh" onClick={refresh}>
              <RefreshCw className="m-icon m-icon--sm" aria-hidden="true" />
            </Button>
            {selected && editable ? (
              <Button
                variant="ghost"
                size="sm"
                title="Mark the row for deletion (⌘⌫)"
                onClick={() => {
                  if (selected.kind === "insert") removeInsert(tk, selected.insertId!);
                  else if (selected.key) toggleDelete(tk, selected.key);
                }}
              >
                <Trash2 className="m-icon m-icon--sm" aria-hidden="true" />
                {selected.kind === "row" && selected.key && changes.deletes[rowKeyOf(selected.key)] ? "Keep row" : "Delete row"}
              </Button>
            ) : null}
            <Button variant="outline" size="sm" disabled={!editable} title={editable ? "Add a row" : undefined} onClick={() => stageInsert(tk)}>
              <Plus className="m-icon m-icon--sm" aria-hidden="true" />
              Row
            </Button>
            <Button variant="ghost" size="sm" icon aria-label="Toggle inspector" aria-pressed={inspectorOpen} title="Inspector (⌘I)" onClick={toggleInspector}>
              <PanelRight className="m-icon m-icon--sm" aria-hidden="true" />
            </Button>
          </>
        ) : (
          <>
            <span className="m-help m-mono" style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
              {database}.{schema}.{table}
              {tableModel?.rowEstimate !== undefined ? ` · ${thousands(Math.max(0, tableModel.rowEstimate))} rows` : ""}
              {tableModel?.sizeBytes ? ` · ${bytes(tableModel.sizeBytes)}` : ""}
            </span>
            <span className="m-spacer" />
            {kind === "table" ? (
              <>
                {view === "structure" ? (
                  <Button variant="outline" size="sm" onClick={() => openStructureForm("new-column")}>
                    <Plus className="m-icon m-icon--sm" aria-hidden="true" />
                    Column
                  </Button>
                ) : null}
                <Button variant="outline" size="sm" onClick={() => openStructureForm("new-index")}>
                  <Plus className="m-icon m-icon--sm" aria-hidden="true" />
                  Index
                </Button>
                <Button variant="ghost" size="sm" onClick={() => setDdl(true)} disabled={!tableModel}>
                  <Code2 className="m-icon m-icon--sm" aria-hidden="true" />
                  Show DDL
                </Button>
              </>
            ) : null}
          </>
        )}
      </div>
      {view === "data" && kind === "table" && meta.isSuccess && !hasPk ? (
        <div className="m-notice m-notice--warning">
          <Info className="m-icon m-icon--sm" aria-hidden="true" />
          This table has no primary key, so rows cannot be told apart safely. Editing is disabled; use the SQL editor.
        </div>
      ) : null}
      {view === "data" && kind === "view" ? (
        <div className="m-notice">
          <Info className="m-icon m-icon--sm" aria-hidden="true" />
          Views are read-only.
        </div>
      ) : null}

      {view === "data" ? (
        rows.isError ? (
          errorState(rows.error)
        ) : (
          <div style={{ flex: 1, minHeight: 0, display: "flex" }}>
            <div className="m-grid-wrap m-grid-wrap--flush" style={{ flex: 1, minWidth: 0 }}>
              {rows.data ? (
                <DataGrid
                  columns={columns}
                  rows={rows.data.result.rows}
                  offset={(page - 1) * PAGE}
                  meta={meta.data}
                  tableKey={tk}
                  changes={changes}
                  editable={editable}
                  sort={sort}
                  onSort={(s) => {
                    setSort(s);
                    setPage(1);
                  }}
                  selectedRow={selectedRow}
                  onSelectRow={setSelectedRow}
                  refetching={rows.isFetching && !rows.isPending}
                />
              ) : null}
              {rows.data && rows.data.result.rows.length === 0 && changes.inserts.length === 0 ? (
                <div style={{ padding: 24 }}>
                  <EmptyState title={filters.length ? "No rows match" : "No rows yet"} description={filters.length ? "Change or remove a condition." : editable ? "Add one with + Row." : undefined} />
                </div>
              ) : null}
            </div>
            {inspectorOpen ? <Inspector row={selected} columns={columns} meta={meta.data} changes={changes} onClose={toggleInspector} /> : null}
          </div>
        )
      ) : kind === "view" ? (
        <div style={{ flex: 1, overflow: "auto", padding: "16px 24px", display: "flex", flexDirection: "column", gap: 16 }}>
          <h2 className="m-section-title">Definition</h2>
          <CodeBlock code={viewMeta?.definition?.trim() || "-- definition not available"} />
        </div>
      ) : meta.isError ? (
        errorState(meta.error)
      ) : tableModel ? (
        <StructureView database={database} schema={schema} table={tableModel} model={model.data} edits={edits} editKey={tk} editable={kind === "table"} only={view === "indexes" ? "indexes" : undefined} />
      ) : null}

      <ReviewDialog
        open={review === "rows"}
        onOpenChange={(o) => !o && setReview(null)}
        title="Review changes"
        target={`${database}.${schema}.${table}`}
        connection={connection}
        reviewKey={["rows", tk, JSON.stringify(changes)]}
        load={async () => {
          const r = await planChanges(connectionId, database, rowChangeSet());
          return { plan: r.plan, decision: r.decision, upSql: r.upSql, downSql: r.downSql };
        }}
        apply={({ confirmation, transaction }) => applyChanges(connectionId, database, { changeSet: rowChangeSet(), confirmation, transaction })}
        onApplied={async () => {
          discardRows(tk);
          await afterApply();
        }}
      />
      <ReviewDialog
        open={review === "schema"}
        onOpenChange={(o) => !o && setReview(null)}
        title="Apply schema changes"
        target={`${database}.${schema}.${table}`}
        connection={connection}
        reviewKey={["schema", tk, JSON.stringify(edits)]}
        load={async () => {
          const cs = schemaChangeSet();
          const r = await planChanges(connectionId, database, cs);
          return { plan: r.plan, decision: r.decision, upSql: r.upSql, downSql: r.downSql, changes: cs.schema };
        }}
        apply={({ confirmation, transaction }) => applyChanges(connectionId, database, { changeSet: schemaChangeSet(), confirmation, transaction })}
        onApplied={async () => {
          discardEdits(tk);
          await afterApply();
        }}
      />
      <DdlDialog open={ddl} onOpenChange={setDdl} database={database} schema={schema} table={table} />
    </>
  );
}

function DdlDialog({ open, onOpenChange, database, schema, table }: { open: boolean; onOpenChange: (o: boolean) => void; database: string; schema: string; table: string }) {
  const { connectionId } = useWorkspace();
  const meta = useGetTable(connectionId, database, schema, table);
  const ddl = useQuery({
    queryKey: ["ddl", connectionId, database, schema, table, meta.dataUpdatedAt],
    queryFn: async () => {
      const r = await planChanges(connectionId, database, { database, schema: [{ kind: "create_table", table: { schema, name: table }, newTable: meta.data! }] });
      return r.upSql;
    },
    enabled: open && !!meta.data,
  });
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent width={720} aria-describedby={undefined}>
        <DialogHead>
          <DialogTitle>
            DDL for <span className="m-mono">{schema}.{table}</span>
          </DialogTitle>
        </DialogHead>
        <DialogBody>{ddl.data ? <CodeBlock code={ddl.data} style={{ maxHeight: 480 }} /> : <p className="m-help">{ddl.isError ? errorMessage(ddl.error) : "Generating…"}</p>}</DialogBody>
        <DialogFoot>
          <span className="m-help">Foreign keys are created separately so every table exists first.</span>
          <span className="m-spacer" />
          <Button
            variant="outline"
            disabled={!ddl.data}
            onClick={() => {
              void navigator.clipboard.writeText(ddl.data ?? "").then(() => toast.success("DDL copied"));
            }}
          >
            Copy
          </Button>
          <Button variant="primary" onClick={() => onOpenChange(false)}>
            Done
          </Button>
        </DialogFoot>
      </DialogContent>
    </Dialog>
  );
}
