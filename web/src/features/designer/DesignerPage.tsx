/**
 * The schema designer (`Diagram`, `DiagramNew`, `DiagramApply` boards). An existing database
 * opens drawn from its live schema; a new one opens empty. The DBML and the diagram stay in
 * sync: typing parses into the model, UI edits re-print the DBML. Review plans the diff against
 * the live database; nothing runs until it is applied.
 */
import * as React from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { Database as DatabaseIcon, Download, FileUp, LayoutGrid, Link2, Plus, RefreshCw, Table2, Trash2 } from "lucide-react";
import { toast } from "sonner";

import { applyChanges } from "@/lib/api/changes/changes";
import { diffDesignerModel, getGetDesignerModelQueryKey, parseDbml, printDbml, saveDesignerLayout, useGetDesignerModel } from "@/lib/api/designer/designer";
import { getGetSchemaQueryKey, useListDatabases } from "@/lib/api/explore/explore";
import type { Column, Database, Problem } from "@/lib/api/mesaAPI.schemas";
import { errorMessage } from "@/lib/api/fetch";
import { plural } from "@/lib/format";
import { cn, isCmd } from "@/lib/utils";
import { usePendingStatus, useStatusFacts } from "@/app/status.store";
import { Button } from "@/ui/Button";
import { EmptyState, Kbd, Select } from "@/ui/controls";
import { ReviewDialog } from "@/features/changes/ReviewDialog";
import { useTabs } from "@/features/explorer/tabs.store";
import { Editor } from "@/features/sql/Editor";
import { shortcut } from "@/app/keys";

import { useWorkspace } from "../workspace/context";
import { AddRelationDialog, AddTableDialog, ImportDialog } from "./dialogs";
import { Erd } from "./Erd";
import { addColumn, addForeignKey, addTable, autoLayout, emptyModel, findTable, newness, placeMissing, removeColumn, removeTable, tablesOf, type Pos } from "./model";

type View = "split" | "diagram" | "code";

export function DesignerPage({ database: requested, fresh }: { database?: string; fresh: boolean }) {
  const { connection } = useWorkspace();
  const database = requested ?? connection.database;
  return <Designer key={database} database={database} fresh={fresh} />;
}

function Designer({ database, fresh }: { database: string; fresh: boolean }) {
  const { connectionId, connection } = useWorkspace();
  const navigate = useNavigate();
  const qc = useQueryClient();
  const dbs = useListDatabases(connectionId);
  const live = useGetDesignerModel(connectionId, database, { query: { retry: false, staleTime: Infinity } });

  const [view, setView] = React.useState<View>("split");
  const [dbml, setDbml] = React.useState<string | null>(null);
  const [model, setModel] = React.useState<Database | null>(null);
  const [problems, setProblems] = React.useState<Problem[]>([]);
  const [positions, setPositions] = React.useState<Record<string, Pos>>({});
  const [selected, setSelected] = React.useState<string | null>(null);
  const [dialog, setDialog] = React.useState<"table" | "relation" | "import" | null>(null);
  const [review, setReview] = React.useState(false);
  const parseSeq = React.useRef(0);

  React.useEffect(() => {
    useTabs.getState().open(connectionId, { id: `designer:${database}`, kind: "designer", title: `Designer · ${database}`, database });
  }, [connectionId, database]);

  // Start from the live model once it arrives.
  const loadedAt = live.dataUpdatedAt;
  React.useEffect(() => {
    if (!live.data) return;
    const empty = tablesOf(live.data.schema).length === 0;
    setModel(live.data.schema);
    setDbml(empty ? `// ${database} · no tables yet\n// Write tables here, or add them on the diagram.\n` : live.data.dbml);
    setProblems(live.data.problems);
    setPositions(placeMissing(live.data.schema, live.data.layout.positions));
  }, [loadedAt, database]); // eslint-disable-line react-hooks/exhaustive-deps

  const liveSchema = live.data?.schema;
  const current = model ?? liveSchema ?? emptyModel(database);
  const fresh_ = React.useMemo(() => newness(liveSchema, current), [liveSchema, current]);

  // What would change, planned by the server (also gives the count for the status bar).
  const diff = useQuery({
    queryKey: ["designer-diff", connectionId, database, JSON.stringify(current)],
    queryFn: () => diffDesignerModel(connectionId, database, { schema: current }),
    enabled: !!live.data && !!model,
    retry: false,
    staleTime: 0,
  });
  const changeCount = diff.data?.changes.length ?? 0;
  const errorsCount = problems.filter((p) => p.level === "error").length;

  useStatusFacts([`${plural(tablesOf(current).length, "table")}`, ...(errorsCount ? [plural(errorsCount, "DBML error")] : [])]);
  usePendingStatus(changeCount > 0 && errorsCount === 0 ? { summary: plural(changeCount, "change"), onDiscard: () => reset(), onReview: () => setReview(true) } : null);

  React.useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (isCmd(e) && e.key.toLowerCase() === "s") {
        e.preventDefault();
        if (changeCount > 0 && errorsCount === 0) setReview(true);
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [changeCount, errorsCount]);

  const reset = () => {
    if (!live.data) return;
    setModel(live.data.schema);
    setDbml(live.data.dbml);
    setProblems(live.data.problems);
  };

  // Typing: parse after a short pause; the diagram follows the last model that parsed.
  const onType = (text: string) => {
    setDbml(text);
    const seq = ++parseSeq.current;
    window.setTimeout(async () => {
      if (seq !== parseSeq.current) return;
      try {
        const r = await parseDbml({ dbml: text });
        if (seq !== parseSeq.current) return;
        setProblems(r.problems);
        if (r.schema) {
          const next = { ...r.schema, name: database };
          setModel(next);
          setPositions((p) => placeMissing(next, p));
        }
      } catch (e) {
        setProblems([{ level: "error", message: errorMessage(e) }]);
      }
    }, 350);
  };

  // UI edits: change the model, then print it so the DBML matches.
  const applyModel = async (next: Database) => {
    setModel(next);
    setPositions((p) => placeMissing(next, p));
    try {
      const r = await printDbml({ schema: next });
      parseSeq.current++;
      setDbml(r.dbml);
      setProblems([]);
    } catch (e) {
      toast.error(errorMessage(e));
    }
  };

  const saveLayout = React.useRef<number | null>(null);
  const persistLayout = (pos: Record<string, Pos>) => {
    if (saveLayout.current) window.clearTimeout(saveLayout.current);
    saveLayout.current = window.setTimeout(() => {
      void saveDesignerLayout(connectionId, database, { positions: pos }).catch(() => undefined);
    }, 600);
  };

  const exportDbml = () => {
    const url = URL.createObjectURL(new Blob([dbml ?? ""], { type: "text/plain" }));
    const a = document.createElement("a");
    a.href = url;
    a.download = `${database}.dbml`;
    a.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  };

  if (live.isError) {
    return (
      <div style={{ padding: 24 }}>
        <EmptyState icon={<DatabaseIcon className="m-icon" aria-hidden="true" />} title={`Cannot open ${database}`} description={errorMessage(live.error)} />
      </div>
    );
  }

  const markLines = new Set(problems.filter((p) => p.level === "error" && p.line).map((p) => p.line!));
  const sel = selected ? findTable(current, selected) : undefined;
  const emptyState = (
    <div className="m-empty" style={{ width: 420, background: "var(--card)" }}>
      <div className="m-empty__media">
        <LayoutGrid className="m-icon" aria-hidden="true" />
      </div>
      <p className="m-empty__title">{database} has no tables yet</p>
      <p className="m-empty__desc">Add a table here or write DBML on the left. Both stay in sync, and nothing runs until you review the SQL.</p>
      <div className="m-empty__actions">
        <Button variant="primary" size="sm" onClick={() => setDialog("table")}>
          <Plus className="m-icon m-icon--sm" aria-hidden="true" />
          Add table
        </Button>
        <Button variant="outline" size="sm" onClick={() => setDialog("import")}>
          <FileUp className="m-icon m-icon--sm" aria-hidden="true" />
          Import DBML
        </Button>
      </div>
    </div>
  );

  return (
    <>
      <div className="m-toolbar" style={{ height: "var(--size-toolbar)", flex: "none" }}>
        <div className="m-seg" role="tablist" aria-label="Designer view">
          {(["split", "diagram", "code"] as View[]).map((v) => (
            <button key={v} type="button" role="tab" aria-selected={view === v} className={cn("m-seg__item", view === v && "is-active")} onClick={() => setView(v)}>
              {v === "split" ? "Split" : v === "diagram" ? "Diagram" : "Code"}
            </button>
          ))}
        </div>
        <span className="m-sep" style={{ height: 20, alignSelf: "center" }} />
        <span style={{ display: "inline-flex", alignItems: "center", gap: 6 }}>
          <DatabaseIcon className="m-icon m-icon--sm m-muted" aria-hidden="true" />
          <Select size="sm" mono aria-label="Database" value={database} options={(dbs.data ?? [{ name: database }]).map((d) => ({ value: d.name, label: d.name }))} onChange={(v) => void navigate({ to: "/c/$connectionId/designer", params: { connectionId }, search: { database: v } })} />
        </span>
        <span className="m-sep" style={{ height: 20, alignSelf: "center" }} />
        <Button variant="ghost" size="sm" onClick={() => setDialog("table")}>
          <Table2 className="m-icon m-icon--sm" aria-hidden="true" />
          Table
        </Button>
        <Button variant="ghost" size="sm" onClick={() => setDialog("relation")} disabled={tablesOf(current).length === 0}>
          <Link2 className="m-icon m-icon--sm" aria-hidden="true" />
          Relation
        </Button>
        <Button
          variant="ghost"
          size="sm"
          onClick={() => {
            const pos = autoLayout(current);
            setPositions(pos);
            persistLayout(pos);
          }}
          disabled={tablesOf(current).length === 0}
        >
          <LayoutGrid className="m-icon m-icon--sm" aria-hidden="true" />
          Auto layout
        </Button>
        <span className="m-spacer" />
        <Button
          variant="ghost"
          size="sm"
          onClick={async () => {
            if (changeCount > 0 && !window.confirm("Reload the live schema and drop the edits that are not applied?")) return;
            await qc.invalidateQueries({ queryKey: getGetDesignerModelQueryKey(connectionId, database) });
          }}
        >
          <RefreshCw className="m-icon m-icon--sm" aria-hidden="true" />
          Refresh
        </Button>
        <Button variant="ghost" size="sm" onClick={() => setDialog("import")}>
          <FileUp className="m-icon m-icon--sm" aria-hidden="true" />
          Import
        </Button>
        <Button variant="outline" size="sm" onClick={exportDbml} disabled={!dbml}>
          <Download className="m-icon m-icon--sm" aria-hidden="true" />
          Export
        </Button>
        <Button variant="primary" size="sm" onClick={() => setReview(true)} disabled={changeCount === 0 || errorsCount > 0} title="Review & apply (⌘S)">
          Review &amp; apply <Kbd>{shortcut("KeyS")}</Kbd>
        </Button>
      </div>
      <div style={{ flex: 1, minHeight: 0, display: "flex" }}>
        {view !== "diagram" ? (
          <section aria-label="DBML source" style={{ flex: view === "code" ? 1 : "0 0 40%", minWidth: 0, display: "flex", flexDirection: "column", borderRight: "1px solid var(--border)", background: "var(--background)" }}>
            <div style={{ display: "flex", alignItems: "center", gap: 8, height: 36, flex: "none", padding: "0 12px", borderBottom: "1px solid var(--border-subtle)" }}>
              <span className="m-mono" style={{ fontSize: 12 }}>
                {database}
              </span>
              <span className="m-badge m-badge--mono">DBML</span>
              <span className="m-spacer" />
              {errorsCount > 0 ? (
                <span className="m-status m-help" style={{ color: "var(--destructive)" }}>
                  <span className="m-dot m-dot--down" />
                  {problems.find((p) => p.level === "error")?.message}
                </span>
              ) : changeCount > 0 ? (
                <span className="m-status m-help">
                  <span className="m-dot m-dot--pending" />
                  {plural(changeCount, "change")} not applied
                </span>
              ) : (
                <span className="m-status m-help">
                  <span className="m-dot m-dot--online" />
                  {fresh && tablesOf(current).length === 0 ? "New database" : "Matches the live schema"}
                </span>
              )}
            </div>
            <div style={{ flex: 1, minHeight: 0 }}>{dbml !== null ? <Editor value={dbml} onChange={onType} language="dbml" ariaLabel="DBML" markLines={markLines} /> : <p className="m-help" style={{ padding: 12 }}>Loading the live schema…</p>}</div>
            {problems.filter((p) => p.level === "warning").length > 0 ? (
              <div style={{ borderTop: "1px solid var(--border)", padding: "6px 12px", maxHeight: 96, overflow: "auto" }}>
                {problems
                  .filter((p) => p.level === "warning")
                  .map((p, i) => (
                    <div key={i} className="m-help">
                      {p.line ? `line ${p.line}: ` : ""}
                      {p.message}
                    </div>
                  ))}
              </div>
            ) : null}
          </section>
        ) : null}
        {view !== "code" ? (
          <Erd
            model={current}
            positions={positions}
            onMove={(key, pos) => setPositions((p) => ({ ...p, [key]: pos }))}
            onMoveEnd={() => persistLayout(positions)}
            selected={selected}
            onSelect={setSelected}
            fresh={fresh_}
            empty={live.data ? emptyState : undefined}
          />
        ) : null}
        {sel && view !== "code" ? (
          <TableInspector
            key={sel.key}
            schema={sel.schema}
            tableKey={sel.key}
            model={current}
            isNew={fresh_.tables.has(sel.key)}
            onChange={(m) => void applyModel(m)}
            onRelation={() => setDialog("relation")}
            onClose={() => setSelected(null)}
          />
        ) : null}
      </div>
      <AddTableDialog
        open={dialog === "table"}
        onOpenChange={(o) => setDialog(o ? "table" : null)}
        model={current}
        onAdd={(schema, table) => {
          void applyModel(addTable(current, schema, table));
          setSelected(`${schema}.${table.name}`);
        }}
      />
      <AddRelationDialog open={dialog === "relation"} onOpenChange={(o) => setDialog(o ? "relation" : null)} model={current} from={selected} onAdd={(key, fk) => void applyModel(addForeignKey(current, key, fk))} />
      <ImportDialog open={dialog === "import"} onOpenChange={(o) => setDialog(o ? "import" : null)} onImport={(text) => onType(text)} />
      <ReviewDialog
        open={review}
        onOpenChange={setReview}
        title="Apply schema changes"
        target={database}
        connection={connection}
        reviewKey={["designer", database, JSON.stringify(current)]}
        load={async () => {
          const d = await diffDesignerModel(connectionId, database, { schema: current });
          return { plan: d.plan, decision: d.decision, upSql: d.upSql, downSql: d.downSql, changes: d.changes };
        }}
        apply={async ({ confirmation, transaction }) => {
          const d = await diffDesignerModel(connectionId, database, { schema: current });
          return applyChanges(connectionId, database, { changeSet: { database, schema: d.changes }, confirmation, transaction });
        }}
        onApplied={async () => {
          await qc.invalidateQueries({ queryKey: getGetDesignerModelQueryKey(connectionId, database) });
          await qc.invalidateQueries({ queryKey: getGetSchemaQueryKey(connectionId, database) });
          await saveDesignerLayout(connectionId, database, { positions }).catch(() => undefined);
        }}
      />
    </>
  );
}

function TableInspector({ schema, tableKey, model, isNew, onChange, onRelation, onClose }: { schema: string; tableKey: string; model: Database; isNew: boolean; onChange: (m: Database) => void; onRelation: () => void; onClose: () => void }) {
  const ref = findTable(model, tableKey);
  const [col, setCol] = React.useState<{ name: string; type: string; notNull: boolean } | null>(null);
  if (!ref) return null;
  const t = ref.table;
  const pk = new Set(t.primaryKey?.columns ?? []);
  const uq = new Set((t.uniques ?? []).filter((u) => u.columns.length === 1).map((u) => u.columns[0]));
  const referencedBy = tablesOf(model).flatMap((o) => (o.table.foreignKeys ?? []).filter((f) => `${f.references.schema}.${f.references.name}` === tableKey).map((f) => `${o.table.name}.${f.columns.join(",")}`));
  return (
    <aside className="m-inspector" aria-label="Table inspector" style={{ width: 300, flex: "none" }}>
      <div className="m-inspector__head">
        <Table2 className="m-icon m-icon--sm" aria-hidden="true" />
        Table <span className="m-mono">{t.name}</span>
        {isNew ? (
          <span className="m-badge" style={{ background: "var(--pending-surface)", color: "var(--pending-foreground)", borderColor: "var(--pending)" }}>
            new
          </span>
        ) : null}
        <span className="m-spacer" />
        <Button variant="ghost" size="xs" onClick={onClose}>
          Done
        </Button>
      </div>
      <div className="m-inspector__body" style={{ gap: 12 }}>
        <div className="m-field">
          <span className="m-label">
            Columns <span className="m-help m-mono">{schema}</span>
          </span>
          <div style={{ display: "flex", flexDirection: "column" }}>
            {t.columns.map((c: Column) => {
              const fk = (t.foreignKeys ?? []).find((f) => f.columns.includes(c.name));
              const flags = [pk.has(c.name) ? "pk" : null, uq.has(c.name) ? "uq" : null, !c.nullable && !pk.has(c.name) ? "nn" : null, fk ? "fk" : null].filter(Boolean) as string[];
              return (
                <div key={c.name} style={{ display: "flex", alignItems: "center", gap: 6, height: 32, borderBottom: "1px solid var(--border-subtle)" }}>
                  <span className="m-mono" style={{ fontSize: 12, flex: 1, minWidth: 0, overflow: "hidden", textOverflow: "ellipsis" }}>
                    {c.name}
                  </span>
                  <span className="m-mono m-muted" style={{ fontSize: 12 }}>
                    {c.type}
                  </span>
                  {flags.map((f) => (
                    <span key={f} className="m-badge m-badge--outline m-badge--mono" style={{ height: 18, padding: "0 4px", fontSize: 11 }}>
                      {f}
                    </span>
                  ))}
                  <Button variant="ghost" size="xs" icon aria-label={`Remove ${c.name}`} onClick={() => onChange(removeColumn(model, tableKey, c.name))}>
                    <Trash2 className="m-icon m-icon--sm" aria-hidden="true" />
                  </Button>
                </div>
              );
            })}
          </div>
          {col ? (
            <form
              onSubmit={(e) => {
                e.preventDefault();
                if (!/^[A-Za-z_][A-Za-z0-9_]*$/.test(col.name) || t.columns.some((c) => c.name === col.name)) return;
                onChange(addColumn(model, tableKey, { name: col.name, type: col.type || "text", nullable: !col.notNull }, false));
                setCol(null);
              }}
              style={{ display: "grid", gap: 6 }}
            >
              <input className="m-input m-input--sm m-input--mono" aria-label="New column name" placeholder="name" value={col.name} onChange={(e) => setCol({ ...col, name: e.target.value })} autoFocus />
              <input className="m-input m-input--sm m-input--mono" aria-label="New column type" placeholder="type" list="designer-types" value={col.type} onChange={(e) => setCol({ ...col, type: e.target.value })} />
              <label className="m-row" style={{ cursor: "pointer" }}>
                <input type="checkbox" checked={col.notNull} onChange={() => setCol({ ...col, notNull: !col.notNull })} style={{ accentColor: "var(--primary)" }} />
                <span className="m-help">Not null</span>
              </label>
              <div className="m-row">
                <Button variant="primary" size="xs" type="submit">
                  Add
                </Button>
                <Button variant="ghost" size="xs" onClick={() => setCol(null)}>
                  Cancel
                </Button>
              </div>
            </form>
          ) : (
            <Button variant="ghost" size="sm" style={{ justifyContent: "flex-start" }} onClick={() => setCol({ name: "", type: "text", notNull: false })}>
              <Plus className="m-icon m-icon--sm" aria-hidden="true" />
              Column
            </Button>
          )}
        </div>
        <div className="m-field">
          <span className="m-label">References</span>
          {(t.foreignKeys ?? []).map((f, i) => (
            <span key={i} className="m-mono" style={{ fontSize: 12, display: "flex", alignItems: "center", gap: 6 }}>
              <Link2 className="m-icon m-icon--sm" aria-hidden="true" />
              {f.columns.join(", ")} → {f.references.name}.{f.refColumns.join(", ")}
              {f.onDelete ? <span className="m-muted"> · on delete {f.onDelete.toLowerCase()}</span> : null}
            </span>
          ))}
          {referencedBy.map((r) => (
            <span key={r} className="m-mono m-muted" style={{ fontSize: 12, display: "flex", alignItems: "center", gap: 6 }}>
              <Link2 className="m-icon m-icon--sm" aria-hidden="true" />
              {r} → {t.name}
            </span>
          ))}
          {(t.foreignKeys ?? []).length + referencedBy.length === 0 ? <span className="m-help">No relations.</span> : null}
          <Button variant="ghost" size="sm" style={{ justifyContent: "flex-start" }} onClick={onRelation}>
            <Plus className="m-icon m-icon--sm" aria-hidden="true" />
            Relation
          </Button>
        </div>
        <Button variant="ghost" size="sm" style={{ justifyContent: "flex-start", color: "var(--destructive)" }} onClick={() => onChange(removeTable(model, tableKey))}>
          <Trash2 className="m-icon m-icon--sm" aria-hidden="true" />
          {isNew ? "Remove table" : "Drop table"}
        </Button>
      </div>
    </aside>
  );
}
