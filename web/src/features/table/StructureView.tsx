/**
 * Structure and Indexes views (`Structure` board). Column edits, new columns and indexes are
 * staged and planned live; the inspector shows the SQL a column edit will run and its risks.
 */
import * as React from "react";
import { useQuery } from "@tanstack/react-query";
import { KeyRound, Link2, Plus, Trash2, Undo2 } from "lucide-react";

import { planChanges } from "@/lib/api/changes/changes";
import type { Column, Database, Index, Table } from "@/lib/api/mesaAPI.schemas";
import { bytes, compact, plural, thousands } from "@/lib/format";
import { cn } from "@/lib/utils";
import { Button } from "@/ui/Button";
import { CodeBlock } from "@/ui/Code";
import { Field, Switch } from "@/ui/controls";

import { useWorkspace } from "../workspace/context";
import { useSchemaEdits, toSchemaChanges, type ColumnEdit, type SchemaEdits } from "./schemaEdits.store";

interface Props {
  database: string;
  schema: string;
  table: Table;
  model?: Database;
  edits: SchemaEdits;
  editKey: string;
  editable: boolean;
  only?: "indexes";
}

type Selection = { kind: "column"; name: string } | { kind: "new-column" } | { kind: "new-index" } | null;

export function StructureView({ database, schema, table, model, edits, editKey, editable, only }: Props) {
  const [sel, setSel] = React.useState<Selection>(null);
  const patch = useSchemaEdits((s) => s.patch);
  const pk = new Set(table.primaryKey?.columns ?? []);
  const uniques = new Set((table.uniques ?? []).filter((u) => u.columns.length === 1).map((u) => u.columns[0]));
  const fks = new Map((table.foreignKeys ?? []).filter((f) => f.columns.length === 1).map((f) => [f.columns[0], f]));
  const referencedBy = React.useMemo(() => {
    const out: { name: string; from: string; onDelete: string }[] = [];
    for (const s of model?.schemas ?? []) {
      for (const t of s.tables) {
        for (const f of t.foreignKeys ?? []) {
          if (f.references.schema === schema && f.references.name === table.name) {
            out.push({ name: f.name ?? "", from: `${t.name}.${f.columns.join(", ")} → ${table.name}.${f.refColumns.join(", ")}`, onDelete: f.onDelete || "NO ACTION" });
          }
        }
      }
    }
    return out;
  }, [model, schema, table.name]);

  React.useEffect(() => {
    const open = (e: Event) => setSel((e as CustomEvent<Selection>).detail);
    window.addEventListener("mesa:structure-select", open);
    return () => window.removeEventListener("mesa:structure-select", open);
  }, []);

  const columns: (Column & { state?: "added" | "dropped" | "edited" })[] = [
    ...table.columns.map((c) => {
      const e = edits.columns[c.name];
      if (edits.dropped.includes(c.name)) return { ...c, state: "dropped" as const };
      if (e) return { ...c, type: e.type ?? c.type, nullable: e.nullable ?? c.nullable, default: e.default === undefined ? c.default : e.default, comment: e.comment ?? c.comment, state: "edited" as const };
      return c;
    }),
    ...edits.added.map((c) => ({ ...c, state: "added" as const })),
  ];
  const indexes: (Index & { state?: "added" | "dropped" })[] = [
    ...(table.indexes ?? []).map((ix) => (edits.indexesDropped.includes(ix.name) ? { ...ix, state: "dropped" as const } : ix)),
    ...edits.indexesAdded.map((ix) => ({ ...ix, state: "added" as const })),
  ];

  const indexSection = (
    <section style={{ display: "flex", flexDirection: "column", gap: 8 }}>
      <h2 className="m-section-title">
        Indexes <span className="m-badge m-badge--count">{indexes.length}</span>
      </h2>
      <div className="m-grid-wrap">
        <table className="m-grid m-grid--ui" style={{ width: "100%" }}>
          <thead>
            <tr>
              <th>Name</th>
              <th>Columns</th>
              <th>Method</th>
              <th>Unique</th>
              <th className="is-num">Size</th>
              <th className="is-num">Scans</th>
              {editable ? <th style={{ width: 40 }} aria-label="Actions" /> : null}
            </tr>
          </thead>
          <tbody>
            {indexes.length === 0 ? (
              <tr>
                <td colSpan={7} className="m-muted">
                  No indexes.
                </td>
              </tr>
            ) : null}
            {indexes.map((ix) => (
              <tr key={ix.name} className={cn(ix.state === "added" && "is-added", ix.state === "dropped" && "is-deleted")}>
                <td className="m-mono">{ix.name}</td>
                <td className="m-mono">
                  {ix.expression ?? ix.columns?.join(", ")}
                  {ix.predicate ? <span className="m-muted"> WHERE {ix.predicate}</span> : null}
                </td>
                <td className="m-mono">{ix.method || "btree"}</td>
                <td className={ix.unique ? undefined : "m-muted"}>{ix.unique ? "Yes" : "No"}</td>
                <td className="is-num m-mono">{ix.sizeBytes !== undefined ? bytes(ix.sizeBytes) : ""}</td>
                <td className="is-num">
                  {ix.scans === 0 && !ix.unique && !ix.state ? (
                    <span className="m-status" style={{ justifyContent: "flex-end" }} title="Never used since statistics were reset">
                      <span className="m-dot m-dot--degraded" />
                      <span className="m-mono">0</span>
                    </span>
                  ) : (
                    <span className="m-mono">{ix.scans !== undefined ? compact(ix.scans) : ""}</span>
                  )}
                </td>
                {editable ? (
                  <td>
                    {ix.state === "added" ? (
                      <Button variant="ghost" size="xs" icon aria-label={`Remove ${ix.name}`} onClick={() => patch(editKey, (e) => ({ ...e, indexesAdded: e.indexesAdded.filter((x) => x.name !== ix.name) }))}>
                        <Undo2 className="m-icon m-icon--sm" aria-hidden="true" />
                      </Button>
                    ) : (
                      <Button
                        variant="ghost"
                        size="xs"
                        icon
                        aria-label={ix.state === "dropped" ? `Keep ${ix.name}` : `Drop ${ix.name}`}
                        onClick={() =>
                          patch(editKey, (e) => ({ ...e, indexesDropped: e.indexesDropped.includes(ix.name) ? e.indexesDropped.filter((x) => x !== ix.name) : [...e.indexesDropped, ix.name] }))
                        }
                      >
                        {ix.state === "dropped" ? <Undo2 className="m-icon m-icon--sm" aria-hidden="true" /> : <Trash2 className="m-icon m-icon--sm" aria-hidden="true" />}
                      </Button>
                    )}
                  </td>
                ) : null}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </section>
  );

  return (
    <div style={{ flex: 1, minHeight: 0, display: "flex" }}>
      <div style={{ flex: 1, minWidth: 0, overflow: "auto", padding: "16px 24px", display: "flex", flexDirection: "column", gap: 24 }}>
        {only === "indexes" ? (
          indexSection
        ) : (
          <>
            <section style={{ display: "flex", flexDirection: "column", gap: 8 }}>
              <h2 className="m-section-title">
                Columns <span className="m-badge m-badge--count">{columns.length}</span>
              </h2>
              <div className="m-grid-wrap">
                <table className="m-grid m-grid--ui" style={{ width: "100%" }}>
                  <thead>
                    <tr>
                      <th style={{ width: 32 }}>#</th>
                      <th>Name</th>
                      <th>Type</th>
                      <th>Nullable</th>
                      <th>Default</th>
                      <th>Constraints</th>
                      <th>Comment</th>
                    </tr>
                  </thead>
                  <tbody>
                    {columns.map((c, i) => {
                      const selected = sel?.kind === "column" && sel.name === c.name;
                      const edit = edits.columns[c.name];
                      const fk = fks.get(c.name);
                      return (
                        <tr
                          key={c.name}
                          className={cn("is-link", selected && "is-selected", c.state === "edited" && "is-modified", c.state === "added" && "is-added", c.state === "dropped" && "is-deleted")}
                          onClick={() => setSel(c.state === "added" ? null : { kind: "column", name: c.name })}
                        >
                          <td className="m-muted m-mono">{c.state === "edited" ? "~" : c.state === "added" ? "+" : c.state === "dropped" ? "−" : i + 1}</td>
                          <td className="m-mono">
                            <span style={{ display: "inline-flex", alignItems: "center", gap: 6 }}>
                              {pk.has(c.name) ? <KeyRound className="m-icon m-icon--sm" aria-label="Primary key" /> : fk ? <Link2 className="m-icon m-icon--sm" aria-label="Foreign key" /> : null}
                              {c.name}
                            </span>
                          </td>
                          <td className={cn("m-mono", edit?.type !== undefined && "is-dirty")}>{c.type}</td>
                          <td className={cn(!c.nullable && "m-muted", edit?.nullable !== undefined && "is-dirty")}>{c.nullable ? "Yes" : "No"}</td>
                          <td className={cn("m-mono", edit?.default !== undefined && "is-dirty")}>{c.identity ? <span className="m-muted">identity ({c.identity})</span> : c.default}</td>
                          <td>
                            <span className="m-row" style={{ gap: 4 }}>
                              {pk.has(c.name) ? <span className="m-badge m-badge--outline">Primary key</span> : null}
                              {uniques.has(c.name) ? <span className="m-badge m-badge--outline">Unique</span> : null}
                              {fk ? <span className="m-badge m-badge--outline m-mono" title={fk.name}>→ {fk.references.name}.{fk.refColumns.join(",")}</span> : null}
                            </span>
                          </td>
                          <td className={cn("m-muted", edit?.comment !== undefined && "is-dirty")}>{c.comment}</td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            </section>
            {(table.checks?.length ?? 0) > 0 ? (
              <section style={{ display: "flex", flexDirection: "column", gap: 8 }}>
                <h2 className="m-section-title">
                  Checks <span className="m-badge m-badge--count">{table.checks!.length}</span>
                </h2>
                <div className="m-grid-wrap">
                  <table className="m-grid m-grid--ui" style={{ width: "100%" }}>
                    <tbody>
                      {table.checks!.map((ch) => (
                        <tr key={ch.name ?? ch.expression}>
                          <td className="m-mono" style={{ width: "30%" }}>
                            {ch.name}
                          </td>
                          <td className="m-mono">{ch.expression}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </section>
            ) : null}
            {indexSection}
            {referencedBy.length > 0 ? (
              <section style={{ display: "flex", flexDirection: "column", gap: 8 }}>
                <h2 className="m-section-title">
                  Referenced by <span className="m-badge m-badge--count">{referencedBy.length}</span>
                </h2>
                <div className="m-grid-wrap">
                  <table className="m-grid m-grid--ui" style={{ width: "100%" }}>
                    <thead>
                      <tr>
                        <th>Constraint</th>
                        <th>From</th>
                        <th>On delete</th>
                      </tr>
                    </thead>
                    <tbody>
                      {referencedBy.map((r) => (
                        <tr key={r.name + r.from}>
                          <td className="m-mono">{r.name}</td>
                          <td className="m-mono">{r.from}</td>
                          <td className="m-mono">{r.onDelete}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </section>
            ) : null}
          </>
        )}
      </div>
      {sel && editable ? (
        <aside className="m-inspector" aria-label="Edit structure" style={{ width: "var(--size-inspector)", flex: "none" }}>
          {sel.kind === "column" ? (
            <ColumnEditor key={sel.name} database={database} schema={schema} table={table} name={sel.name} edits={edits} editKey={editKey} onClose={() => setSel(null)} />
          ) : sel.kind === "new-column" ? (
            <NewColumn editKey={editKey} table={table} onClose={() => setSel(null)} />
          ) : (
            <NewIndex editKey={editKey} table={table} onClose={() => setSel(null)} />
          )}
        </aside>
      ) : null}
    </div>
  );
}

/** Opens the inspector on a new column or index form (used by the toolbar buttons). */
export function openStructureForm(kind: "new-column" | "new-index") {
  window.dispatchEvent(new CustomEvent("mesa:structure-select", { detail: { kind } }));
}

function ColumnEditor({ database, schema, table, name, edits, editKey, onClose }: { database: string; schema: string; table: Table; name: string; edits: SchemaEdits; editKey: string; onClose: () => void }) {
  const { connectionId } = useWorkspace();
  const patch = useSchemaEdits((s) => s.patch);
  const col = table.columns.find((c) => c.name === name);
  const edit = edits.columns[name] ?? {};
  const dropped = edits.dropped.includes(name);
  const setEdit = (p: ColumnEdit) =>
    patch(editKey, (e) => {
      const next: ColumnEdit = { ...(e.columns[name] ?? {}), ...p };
      // Drop fields that equal the column as it is.
      if (next.type === col?.type) delete next.type;
      if (next.nullable === col?.nullable) delete next.nullable;
      if (next.default !== undefined && (next.default ?? null) === (col?.default ?? null)) delete next.default;
      if (next.comment !== undefined && next.comment === (col?.comment ?? "")) delete next.comment;
      const columns = { ...e.columns };
      if (Object.keys(next).length) columns[name] = next;
      else delete columns[name];
      return { ...e, columns };
    });
  const q = { schema, name: table.name };
  const changes = toSchemaChanges({ columns: edit && Object.keys(edit).length ? { [name]: edit } : {}, added: [], dropped: dropped ? [name] : [], indexesAdded: [], indexesDropped: [] }, q, table);
  const preview = useQuery({
    queryKey: ["column-preview", connectionId, database, schema, table.name, name, JSON.stringify(changes)],
    queryFn: () => planChanges(connectionId, database, { database, schema: changes }),
    enabled: changes.length > 0,
    retry: false,
  });
  if (!col) return null;
  const notNull = !(edit.nullable ?? col.nullable);
  return (
    <>
      <div className="m-inspector__head">
        Column <span className="m-mono">{name}</span>
        <span className="m-spacer" />
        <Button variant="ghost" size="xs" onClick={onClose}>
          Done
        </Button>
      </div>
      <div className="m-inspector__body" style={{ gap: 12 }}>
        <Field label="Type" htmlFor="col-type">
          <input id="col-type" className={cn("m-input m-input--sm m-input--mono")} value={edit.type ?? col.type} onChange={(e) => setEdit({ type: e.target.value })} disabled={dropped} />
        </Field>
        <Field label="Default" htmlFor="col-default" help="An SQL expression, e.g. 0, 'free' or now()">
          <input id="col-default" className="m-input m-input--sm m-input--mono" value={(edit.default === undefined ? col.default : edit.default) ?? ""} onChange={(e) => setEdit({ default: e.target.value === "" ? null : e.target.value })} disabled={dropped || !!col.identity} />
        </Field>
        <div
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            gap: 12,
            padding: 8,
            borderRadius: "var(--radius-md)",
            border: `1px solid ${edit.nullable !== undefined ? "var(--pending)" : "var(--border)"}`,
            background: edit.nullable !== undefined ? "var(--pending-surface)" : undefined,
          }}
        >
          <span>
            <span className="m-label" style={{ color: edit.nullable !== undefined ? "var(--pending-foreground)" : undefined }}>
              Not null
            </span>
            {edit.nullable !== undefined ? <span className="m-help" style={{ display: "block" }}>Changed · was {col.nullable ? "nullable" : "not null"}</span> : null}
          </span>
          <Switch aria-label="Not null" checked={notNull} onChange={(v) => setEdit({ nullable: !v })} disabled={dropped} />
        </div>
        <Field label="Comment" htmlFor="col-comment">
          <input id="col-comment" className="m-input m-input--sm" value={edit.comment ?? col.comment ?? ""} onChange={(e) => setEdit({ comment: e.target.value })} disabled={dropped} />
        </Field>
        {changes.length > 0 ? (
          <div className="m-field">
            <span className="m-label">Will run</span>
            {preview.data ? <CodeBlock code={preview.data.upSql} style={{ fontSize: 12, lineHeight: "18px" }} /> : <span className="m-help">Planning…</span>}
            {(preview.data?.plan.risks ?? []).map((r, i) => (
              <span key={i} className={cn("m-help", r.level === "danger" && "m-help--error")}>
                {r.message}
              </span>
            ))}
            {preview.isError ? <span className="m-help m-help--error">{String((preview.error as Error).message)}</span> : null}
          </div>
        ) : null}
        <Button
          variant={dropped ? "outline" : "ghost"}
          size="sm"
          onClick={() => patch(editKey, (e) => ({ ...e, dropped: dropped ? e.dropped.filter((x) => x !== name) : [...e.dropped, name] }))}
          style={{ justifySelf: "start", color: dropped ? undefined : "var(--destructive)" }}
        >
          {dropped ? <Undo2 className="m-icon m-icon--sm" aria-hidden="true" /> : <Trash2 className="m-icon m-icon--sm" aria-hidden="true" />}
          {dropped ? "Keep column" : "Drop column"}
        </Button>
      </div>
    </>
  );
}

const COMMON_TYPES = ["text", "varchar(255)", "bigint", "integer", "numeric(12,2)", "boolean", "timestamptz", "date", "uuid", "jsonb"];

function NewColumn({ editKey, table, onClose }: { editKey: string; table: Table; onClose: () => void }) {
  const patch = useSchemaEdits((s) => s.patch);
  const [name, setName] = React.useState("");
  const [type, setType] = React.useState("text");
  const [nullable, setNullable] = React.useState(true);
  const [def, setDef] = React.useState("");
  const taken = table.columns.some((c) => c.name === name);
  const valid = /^[a-z_][a-z0-9_]*$/i.test(name) && !taken && type.trim() !== "";
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        if (!valid) return;
        patch(editKey, (ed) => ({ ...ed, added: [...ed.added.filter((c) => c.name !== name), { name, type: type.trim(), nullable, default: def.trim() || undefined }] }));
        onClose();
      }}
      style={{ display: "contents" }}
    >
      <div className="m-inspector__head">
        <Plus className="m-icon m-icon--sm" aria-hidden="true" />
        New column
      </div>
      <div className="m-inspector__body" style={{ gap: 12 }}>
        <Field label="Name" htmlFor="new-col-name" error={taken ? "A column has this name" : undefined}>
          <input id="new-col-name" className="m-input m-input--sm m-input--mono" value={name} onChange={(e) => setName(e.target.value)} autoFocus autoComplete="off" />
        </Field>
        <Field label="Type" htmlFor="new-col-type">
          <input id="new-col-type" className="m-input m-input--sm m-input--mono" value={type} onChange={(e) => setType(e.target.value)} list="mesa-types" />
          <datalist id="mesa-types">
            {COMMON_TYPES.map((t) => (
              <option key={t} value={t} />
            ))}
          </datalist>
        </Field>
        <Field label="Default" htmlFor="new-col-default" help={!nullable && !def ? "A NOT NULL column needs a default when the table has rows" : "Optional SQL expression"}>
          <input id="new-col-default" className="m-input m-input--sm m-input--mono" value={def} onChange={(e) => setDef(e.target.value)} />
        </Field>
        <div className="m-row" style={{ justifyContent: "space-between" }}>
          <span className="m-label">Not null</span>
          <Switch aria-label="Not null" checked={!nullable} onChange={(v) => setNullable(!v)} />
        </div>
        <div className="m-row">
          <Button variant="primary" size="sm" type="submit" disabled={!valid}>
            Stage column
          </Button>
          <Button variant="ghost" size="sm" onClick={onClose}>
            Cancel
          </Button>
        </div>
      </div>
    </form>
  );
}

function NewIndex({ editKey, table, onClose }: { editKey: string; table: Table; onClose: () => void }) {
  const patch = useSchemaEdits((s) => s.patch);
  const [cols, setCols] = React.useState<string[]>([]);
  const [unique, setUnique] = React.useState(false);
  const [method, setMethod] = React.useState("btree");
  const auto = `${table.name}_${cols.join("_")}_idx`.slice(0, 63);
  const [name, setName] = React.useState("");
  const finalName = name || auto;
  const valid = cols.length > 0 && !(table.indexes ?? []).some((i) => i.name === finalName);
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault();
        if (!valid) return;
        patch(editKey, (ed) => ({ ...ed, indexesAdded: [...ed.indexesAdded, { name: finalName, columns: cols, unique, method }] }));
        onClose();
      }}
      style={{ display: "contents" }}
    >
      <div className="m-inspector__head">
        <Plus className="m-icon m-icon--sm" aria-hidden="true" />
        New index
      </div>
      <div className="m-inspector__body" style={{ gap: 12 }}>
        <div className="m-field">
          <span className="m-label">Columns, in order</span>
          <div style={{ display: "grid", gap: 2 }}>
            {table.columns.map((c) => {
              const at = cols.indexOf(c.name);
              return (
                <label key={c.name} className="m-row" style={{ flexWrap: "nowrap", cursor: "pointer" }}>
                  <input type="checkbox" checked={at >= 0} onChange={() => setCols(at >= 0 ? cols.filter((x) => x !== c.name) : [...cols, c.name])} style={{ accentColor: "var(--primary)" }} />
                  <span className="m-mono">{c.name}</span>
                  {at >= 0 ? <span className="m-badge m-badge--count">{at + 1}</span> : null}
                  <span className="m-spacer" />
                  <span className="m-help m-mono">{c.type}</span>
                </label>
              );
            })}
          </div>
        </div>
        <Field label="Name" htmlFor="new-ix-name">
          <input id="new-ix-name" className="m-input m-input--sm m-input--mono" value={name} placeholder={auto} onChange={(e) => setName(e.target.value)} />
        </Field>
        <Field label="Method" htmlFor="new-ix-method">
          <input id="new-ix-method" className="m-input m-input--sm m-input--mono" value={method} onChange={(e) => setMethod(e.target.value)} list="mesa-index-methods" />
          <datalist id="mesa-index-methods">
            {["btree", "hash", "gin", "gist", "brin"].map((m) => (
              <option key={m} value={m} />
            ))}
          </datalist>
        </Field>
        <div className="m-row" style={{ justifyContent: "space-between" }}>
          <span className="m-label">Unique</span>
          <Switch aria-label="Unique" checked={unique} onChange={setUnique} />
        </div>
        <p className="m-help">Building an index blocks writes to {table.name} ({thousands(table.rowEstimate ?? 0)} rows) until it finishes.</p>
        <div className="m-row">
          <Button variant="primary" size="sm" type="submit" disabled={!valid}>
            Stage index
          </Button>
          <Button variant="ghost" size="sm" onClick={onClose}>
            Cancel
          </Button>
        </div>
      </div>
    </form>
  );
}

export function structureFacts(table: Table): string[] {
  return [plural(table.columns.length, "column"), plural(table.indexes?.length ?? 0, "index", "indexes")];
}
