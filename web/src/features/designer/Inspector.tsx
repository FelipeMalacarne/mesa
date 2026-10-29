/**
 * The panel beside the diagram: the selected table (rename while it is new, edit its columns,
 * keys, indexes and relations) or the selected relation (its actions). Every change edits the
 * model and the DBML follows; dropped tables and columns can be put back from here.
 */
import * as React from "react";
import { ArrowRight, ChevronDown, ChevronRight, KeyRound, Link2, Plus, RotateCcw, Table2, Trash2 } from "lucide-react";

import type { Column, Database, Table } from "@/lib/api/mesaAPI.schemas";
import { cn } from "@/lib/utils";
import { Button } from "@/ui/Button";
import { Field, Select } from "@/ui/controls";

import { guessColumn } from "./columns";
import { ACTIONS, enumTypes, NAME, type Action } from "./dialogs";
import { DefaultField, isIntegerType, KeyList, TypeField, useColumnTypes, type KeyItem } from "./fields";
import type { DiagramEdge, DiagramTable, State } from "./graph";
import { addColumn, addForeignKey, addKey, findTable, fkId, isUnique, removeColumn, removeForeignKey, removeKey, removeTable, restoreColumn, restoreTable, setTableComment, tableKey, tablesOf, togglePrimaryKey, updateColumn, updateForeignKey, type KeyKind } from "./model";

/** How a change is recorded for undo: edits with the same key in a row are one step. */
export type Apply = (next: Database, coalesce?: string) => void;

function StateBadge({ state }: { state: State }) {
  if (state === "same") return null;
  return <span className={cn("m-badge m-erd-state", `is-${state}`)}>{state === "dropped" ? "drop" : state}</span>;
}

export function TableInspector({ t, model, live, onChange, onRename, onSelectEdge, onSelectTable, onRelation, onClose }: { t: DiagramTable; model: Database; live: Database | undefined; onChange: Apply; onRename: (name: string) => void; onSelectEdge: (id: string) => void; onSelectTable: (key: string) => void; onRelation: () => void; onClose: () => void }) {
  const types = useColumnTypes();
  const [open, setOpen] = React.useState<string | null>(null);
  const [adding, setAdding] = React.useState<{ name: string; type: string; typed?: boolean } | null>(null);
  const key = t.key;
  const liveTable = live ? findTable(live, key)?.table : undefined;

  if (t.state === "dropped") {
    return (
      <aside className="m-inspector" aria-label="Table inspector" style={{ width: 320, flex: "none" }}>
        <div className="m-inspector__head">
          <Table2 className="m-icon m-icon--sm" aria-hidden="true" />
          <span className="m-mono m-erd-insp__title">{t.table.name}</span>
          <StateBadge state="dropped" />
          <span className="m-spacer" />
          <Button variant="ghost" size="xs" onClick={onClose}>
            Done
          </Button>
        </div>
        <div className="m-inspector__body">
          <p className="m-help" style={{ margin: 0 }}>
            Applying drops this table and its {t.table.columns.length} columns, with their data.
          </p>
          <Button variant="secondary" size="sm" onClick={() => live && onChange(restoreTable(model, live, key))}>
            <RotateCcw className="m-icon m-icon--sm" aria-hidden="true" />
            Keep the table
          </Button>
        </div>
      </aside>
    );
  }

  const table = t.table;
  const isNew = t.state === "new";
  const enums = enumTypes(model);
  const keyItems: (KeyItem & { from: KeyKind; i: number })[] = [
    ...(table.uniques ?? []).map((u, i) => ({ kind: "unique" as const, columns: u.columns, name: u.name, from: "unique" as const, i })),
    ...(table.indexes ?? []).map((x, i) => ({ kind: x.unique ? ("unique" as const) : ("index" as const), columns: x.columns ?? [x.expression ?? "expression"], name: x.name, from: "index" as const, i })),
  ];
  const referencedBy = tablesOf(model).flatMap((o) => (o.table.foreignKeys ?? []).filter((f) => tableKey(f.references.schema, f.references.name) === key).map((f) => ({ from: o.key, name: o.table.name, f })));

  return (
    <aside className="m-inspector" aria-label="Table inspector" style={{ width: 340, flex: "none" }}>
      <div className="m-inspector__head">
        <Table2 className="m-icon m-icon--sm" aria-hidden="true" />
        <span className="m-mono m-erd-insp__title">{table.name}</span>
        <StateBadge state={t.state} />
        <span className="m-spacer" />
        <Button variant="ghost" size="xs" onClick={onClose}>
          Done
        </Button>
      </div>
      <div className="m-inspector__body" style={{ gap: 14 }}>
        {isNew ? <TableName table={table} schema={t.schema} model={model} onRename={onRename} /> : null}

        <div className="m-field">
          <span className="m-label">
            Columns <span className="m-help m-mono">{t.schema}</span>
          </span>
          <div className="m-erd-insp__cols">
            {t.rows.map((r) => {
              const c = r.column;
              if (r.state === "dropped")
                return (
                  <div key={c.name} className="m-erd-insp__col is-dropped">
                    <span className="m-erd-insp__colname m-mono">{c.name}</span>
                    <span className="m-mono m-muted m-erd-insp__coltype">{c.type}</span>
                    <Button variant="ghost" size="xs" onClick={() => live && onChange(restoreColumn(model, live, key, c.name))} title="Keep this column">
                      <RotateCcw className="m-icon m-icon--sm" aria-hidden="true" />
                      Keep
                    </Button>
                  </div>
                );
              const expanded = open === c.name;
              const flags = [r.pk ? "pk" : null, r.unique ? "uq" : null, !c.nullable && !r.pk ? "nn" : null, c.identity ? "ai" : null].filter(Boolean) as string[];
              return (
                <div key={c.name} className={cn("m-erd-insp__colwrap", expanded && "is-open")}>
                  <button type="button" className={cn("m-erd-insp__col", r.state !== "same" && `is-${r.state}`)} aria-expanded={expanded} onClick={() => setOpen(expanded ? null : c.name)}>
                    {expanded ? <ChevronDown className="m-icon m-icon--sm" aria-hidden="true" /> : <ChevronRight className="m-icon m-icon--sm" aria-hidden="true" />}
                    {r.pk ? <KeyRound className="m-icon m-icon--sm" aria-label="Primary key" /> : r.fk ? <Link2 className="m-icon m-icon--sm" aria-label="Foreign key" /> : null}
                    <span className="m-erd-insp__colname m-mono">{c.name}</span>
                    <span className="m-mono m-muted m-erd-insp__coltype">{c.type}</span>
                    {flags.map((f) => (
                      <span key={f} className="m-badge m-badge--outline m-badge--mono m-erd-insp__flag">
                        {f}
                      </span>
                    ))}
                  </button>
                  {expanded ? <ColumnEditor table={table} tableKey={key} column={c} live={liveTable?.columns.find((x) => x.name === c.name)} isNewColumn={r.state === "new"} model={model} types={types} enums={enums} onChange={onChange} onRenamed={setOpen} /> : null}
                </div>
              );
            })}
          </div>
          {adding ? (
            <form
              className="m-erd-insp__add"
              onSubmit={(e) => {
                e.preventDefault();
                if (!NAME.test(adding.name) || table.columns.some((c) => c.name === adding.name) || !adding.type.trim()) return;
                const g = adding.typed ? {} : guessColumn(adding.name, types);
                onChange(addColumn(model, key, { name: adding.name, type: adding.type.trim(), nullable: !g.notNull, default: g.default }, false));
                setOpen(adding.name);
                setAdding(null);
              }}
              onKeyDown={(e) => {
                if (e.key === "Enter" && (e.target as HTMLElement).getAttribute("role") === "combobox") e.preventDefault();
                if (e.key === "Escape") (e.stopPropagation(), setAdding(null));
              }}
            >
              <input
                className="m-input m-input--sm m-input--mono"
                aria-label="New column name"
                placeholder="name"
                value={adding.name}
                onChange={(e) => {
                  // The name suggests a type until one is picked.
                  const g = guessColumn(e.target.value, types);
                  setAdding({ ...adding, name: e.target.value, type: !adding.typed && g.type ? g.type : adding.type });
                }}
                autoFocus
                spellCheck={false}
              />
              <TypeField aria-label="New column type" value={adding.type} onChange={(v) => setAdding({ ...adding, type: v, typed: true })} types={types} enums={enums} />
              <div className="m-row">
                <Button variant="primary" size="xs" type="submit" disabled={!NAME.test(adding.name) || table.columns.some((c) => c.name === adding.name)}>
                  Add column
                </Button>
                <Button variant="ghost" size="xs" onClick={() => setAdding(null)}>
                  Cancel
                </Button>
                {adding.name && table.columns.some((c) => c.name === adding.name) ? <span className="m-help m-help--error">Taken</span> : null}
              </div>
            </form>
          ) : (
            <Button variant="ghost" size="sm" style={{ justifyContent: "flex-start" }} onClick={() => setAdding({ name: "", type: types.find((x) => x.group === "Text")?.spelling ?? "text" })}>
              <Plus className="m-icon m-icon--sm" aria-hidden="true" />
              Column
            </Button>
          )}
        </div>

        <div className="m-field">
          <span className="m-label">Unique keys and indexes</span>
          <KeyList
            keys={keyItems}
            columns={table.columns.map((c) => c.name)}
            onAdd={(kind, columns) => onChange(addKey(model, key, kind, columns))}
            onRemove={(i) => {
              const k = keyItems[i]!;
              onChange(removeKey(model, key, k.from, k.i));
            }}
          />
        </div>

        <div className="m-field">
          <span className="m-label">Relations</span>
          {(table.foreignKeys ?? []).map((f) => (
            <button key={fkId(f)} type="button" className="m-erd-insp__rel" onClick={() => onSelectEdge(`${key}:${fkId(f)}`)}>
              <Link2 className="m-icon m-icon--sm" aria-hidden="true" />
              <span className="m-mono">{f.columns.join(", ")}</span>
              <ArrowRight className="m-icon m-icon--sm m-muted" aria-hidden="true" />
              <span className="m-mono">
                {f.references.name}.{f.refColumns.join(", ")}
              </span>
              {f.onDelete && f.onDelete.toUpperCase() !== "NO ACTION" ? <span className="m-help">on delete {f.onDelete.toLowerCase()}</span> : null}
            </button>
          ))}
          {referencedBy.map((r) => (
            <button key={`${r.from}:${fkId(r.f)}`} type="button" className="m-erd-insp__rel m-muted" onClick={() => onSelectTable(r.from)}>
              <Link2 className="m-icon m-icon--sm" aria-hidden="true" />
              <span className="m-mono">
                {r.name}.{r.f.columns.join(", ")}
              </span>
              <ArrowRight className="m-icon m-icon--sm" aria-hidden="true" />
              <span className="m-mono">{r.f.refColumns.join(", ")}</span>
            </button>
          ))}
          {(table.foreignKeys ?? []).length + referencedBy.length === 0 ? <span className="m-help">None yet. Drag a column&apos;s dot onto another table, or:</span> : null}
          <Button variant="ghost" size="sm" style={{ justifyContent: "flex-start" }} onClick={onRelation}>
            <Plus className="m-icon m-icon--sm" aria-hidden="true" />
            Relation
          </Button>
        </div>

        <Field label="Note" htmlFor="insp-note">
          <CommitInput id="insp-note" value={table.comment ?? ""} placeholder="What this table holds" onCommit={(v) => onChange(setTableComment(model, key, v))} />
        </Field>

        <Button variant="ghost" size="sm" style={{ justifyContent: "flex-start", color: "var(--destructive)" }} onClick={() => onChange(removeTable(model, key))}>
          <Trash2 className="m-icon m-icon--sm" aria-hidden="true" />
          {isNew ? "Remove table" : "Drop table"}
        </Button>
      </div>
    </aside>
  );
}

/** A text input that edits locally and commits on Enter or when it loses focus. */
function CommitInput({ value, onCommit, valid = () => true, ...rest }: { value: string; onCommit: (v: string) => void; valid?: (v: string) => boolean } & Omit<React.ComponentProps<"input">, "value" | "onChange">) {
  const [draft, setDraft] = React.useState(value);
  React.useEffect(() => setDraft(value), [value]);
  const commit = () => {
    if (draft !== value && valid(draft)) onCommit(draft);
    else setDraft(value);
  };
  return (
    <input
      {...rest}
      className={cn("m-input m-input--sm", rest.className)}
      value={draft}
      aria-invalid={draft !== value && !valid(draft)}
      spellCheck={false}
      onChange={(e) => setDraft(e.target.value)}
      onBlur={commit}
      onKeyDown={(e) => {
        if (e.key === "Enter") (e.preventDefault(), commit());
        if (e.key === "Escape") (e.stopPropagation(), setDraft(value));
      }}
    />
  );
}

function TableName({ table, schema, model, onRename }: { table: Table; schema: string; model: Database; onRename: (name: string) => void }) {
  const taken = (n: string) => tablesOf(model).some((t) => t.schema === schema && t.table.name === n && t.table !== table);
  return (
    <Field label="Name" htmlFor="insp-name" help="Tables already in the database keep their name here.">
      <CommitInput id="insp-name" className="m-input--mono" value={table.name} valid={(v) => NAME.test(v) && !taken(v)} onCommit={onRename} />
    </Field>
  );
}

function ColumnEditor({ table, tableKey: key, column: c, live, isNewColumn, model, types, enums, onChange, onRenamed }: { table: Table; tableKey: string; column: Column; live: Column | undefined; isNewColumn: boolean; model: Database; types: ReturnType<typeof useColumnTypes>; enums: string[]; onChange: Apply; onRenamed: (name: string) => void }) {
  const pk = (table.primaryKey?.columns ?? []).includes(c.name);
  const unique = isUnique(table, c.name);
  const integer = isIntegerType(c.type, types);
  const edit = (patch: Partial<Column>, field: string) => onChange(updateColumn(model, key, c.name, patch), `${key}.${c.name}.${field}`);
  const was: string[] = [];
  if (live) {
    if (live.type !== c.type) was.push(live.type);
    if (live.nullable !== c.nullable) was.push(live.nullable ? "null" : "not null");
    if ((live.default ?? "") !== (c.default ?? "")) was.push(live.default ? `default ${live.default}` : "no default");
  }
  return (
    <div className="m-erd-insp__editor">
      {isNewColumn ? (
        <Field label="Name">
          <CommitInput
            className="m-input--mono"
            aria-label="Column name"
            value={c.name}
            valid={(v) => NAME.test(v) && !table.columns.some((x) => x.name === v)}
            onCommit={(v) => {
              onChange(updateColumn(model, key, c.name, { name: v }));
              onRenamed(v);
            }}
          />
        </Field>
      ) : null}
      <Field label="Type">
        <TypeField aria-label="Column type" value={c.type} onChange={(v) => edit({ type: v, ...(isIntegerType(v, types) ? {} : { identity: undefined }) }, "type")} types={types} enums={enums} />
      </Field>
      {c.identity ? null : (
        <Field label="Default">
          <DefaultField aria-label="Default" value={c.default ?? ""} onChange={(v) => edit({ default: v.trim() ? v : undefined }, "default")} type={c.type} types={types} />
        </Field>
      )}
      <div className="m-erd-insp__toggles">
        <label title="A value is required">
          <input type="checkbox" checked={!c.nullable} disabled={pk || !!c.identity} onChange={() => edit({ nullable: !c.nullable }, "nullable")} />
          Not null
        </label>
        <label>
          <input type="checkbox" checked={pk} onChange={() => onChange(togglePrimaryKey(model, key, c.name))} />
          Primary key
        </label>
        <label title="No two rows share a value">
          <input
            type="checkbox"
            checked={unique}
            disabled={pk}
            onChange={() => {
              if (!unique) return onChange(addKey(model, key, "unique", [c.name]));
              const ui = (table.uniques ?? []).findIndex((u) => u.columns.length === 1 && u.columns[0] === c.name);
              if (ui >= 0) return onChange(removeKey(model, key, "unique", ui));
              const ii = (table.indexes ?? []).findIndex((x) => x.unique && x.columns?.length === 1 && x.columns[0] === c.name);
              if (ii >= 0) onChange(removeKey(model, key, "index", ii));
            }}
          />
          Unique
        </label>
        <label title={integer ? "Values come from a sequence (identity / AUTO_INCREMENT)" : "Only integer columns auto-increment"}>
          <input type="checkbox" checked={!!c.identity} disabled={!integer} onChange={() => edit(c.identity ? { identity: undefined } : { identity: "by default", nullable: false, default: undefined }, "identity")} />
          Auto increment
        </label>
      </div>
      <Field label="Comment">
        <CommitInput aria-label="Column comment" value={c.comment ?? ""} onCommit={(v) => edit({ comment: v || undefined }, "comment")} />
      </Field>
      {was.length ? <p className="m-help" style={{ margin: 0 }}>In the database: {was.join(", ")}</p> : null}
      <div className="m-row">
        <Button variant="ghost" size="xs" style={{ color: "var(--destructive)" }} onClick={() => onChange(removeColumn(model, key, c.name))}>
          <Trash2 className="m-icon m-icon--sm" aria-hidden="true" />
          {isNewColumn ? "Remove column" : "Drop column"}
        </Button>
      </div>
    </div>
  );
}

export function RelationInspector({ edge, model, live, onChange, onSelectTable, onClose }: { edge: DiagramEdge; model: Database; live: Database | undefined; onChange: Apply; onSelectTable: (key: string) => void; onClose: () => void }) {
  const f = edge.fk;
  const id = fkId(f);
  const from = findTable(model, edge.from)?.table ?? (live ? findTable(live, edge.from)?.table : undefined);
  const to = f.references.name;
  const action = (v: string | undefined): Action => ((v?.toUpperCase() ?? "NO ACTION") as Action);
  const set = (patch: { onDelete?: string; onUpdate?: string }) => onChange(updateForeignKey(model, edge.from, id, patch));
  const many = edge.oneToOne ? "one" : "many";
  return (
    <aside className="m-inspector" aria-label="Relation inspector" style={{ width: 320, flex: "none" }}>
      <div className="m-inspector__head">
        <Link2 className="m-icon m-icon--sm" aria-hidden="true" />
        Relation
        <StateBadge state={edge.state} />
        <span className="m-spacer" />
        <Button variant="ghost" size="xs" onClick={onClose}>
          Done
        </Button>
      </div>
      <div className="m-inspector__body" style={{ gap: 14 }}>
        <div className="m-erd-insp__ends">
          <button type="button" className="m-erd-insp__end" onClick={() => onSelectTable(edge.from)}>
            <span className="m-mono">{from?.name ?? edge.from}</span>
            <span className="m-mono m-muted">{f.columns.join(", ")}</span>
          </button>
          <ArrowRight className="m-icon m-icon--sm m-muted" aria-hidden="true" />
          <button type="button" className="m-erd-insp__end" onClick={() => onSelectTable(edge.to)}>
            <span className="m-mono">{to}</span>
            <span className="m-mono m-muted">{f.refColumns.join(", ")}</span>
          </button>
        </div>
        <p className="m-help" style={{ margin: 0 }}>
          Each {from?.name ?? "row"} row refers to {edge.optional ? "zero or one" : "exactly one"} {to} row; each {to} row has {many === "one" ? "at most one" : "any number of"} {from?.name ?? "referencing"} rows.
          {f.name ? (
            <>
              {" "}
              Constraint <span className="m-mono">{f.name}</span>.
            </>
          ) : null}
        </p>
        {edge.state === "dropped" ? (
          <>
            <p className="m-help" style={{ margin: 0 }}>
              Applying drops this foreign key.
            </p>
            <Button
              variant="secondary"
              size="sm"
              disabled={!findTable(model, edge.from)}
              onClick={() => {
                if (!live) return;
                const m = from && from.columns.some((c) => f.columns.includes(c.name)) ? model : restoreColumn(model, live, edge.from, f.columns[0]!);
                onChange((findTable(m, edge.from)?.table.foreignKeys ?? []).some((x) => fkId(x) === id) ? m : addForeignKey(m, edge.from, f));
              }}
            >
              <RotateCcw className="m-icon m-icon--sm" aria-hidden="true" />
              Keep the relation
            </Button>
          </>
        ) : (
          <>
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 8 }}>
              <Field label="On delete">
                <Select aria-label="On delete" size="sm" mono value={action(f.onDelete)} options={ACTIONS.map((a) => ({ value: a, label: a }))} onChange={(v) => set({ onDelete: v === "NO ACTION" ? undefined : v })} />
              </Field>
              <Field label="On update">
                <Select aria-label="On update" size="sm" mono value={action(f.onUpdate)} options={ACTIONS.map((a) => ({ value: a, label: a }))} onChange={(v) => set({ onUpdate: v === "NO ACTION" ? undefined : v })} />
              </Field>
            </div>
            <Button variant="ghost" size="sm" style={{ justifyContent: "flex-start", color: "var(--destructive)" }} onClick={() => onChange(removeForeignKey(model, edge.from, id))}>
              <Trash2 className="m-icon m-icon--sm" aria-hidden="true" />
              {edge.state === "new" ? "Remove relation" : "Drop relation"}
            </Button>
          </>
        )}
      </div>
    </aside>
  );
}
