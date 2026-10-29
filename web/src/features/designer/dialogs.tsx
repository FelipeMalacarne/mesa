/** Designer dialogs: new table, new relation, import DBML. Each edits the model; the DBML follows. */
import * as React from "react";
import { Plus, Trash2 } from "lucide-react";

import type { Column, ColumnType, Database, ForeignKey, Table } from "@/lib/api/mesaAPI.schemas";
import { Button } from "@/ui/Button";
import { Field, Select } from "@/ui/controls";
import { Dialog, DialogBody, DialogContent, DialogDescription, DialogFoot, DialogHead, DialogTitle } from "@/ui/primitives/dialog";

import { DefaultField, isIntegerType, KeyList, TypeField, useColumnTypes, type KeyItem } from "./fields";
import { indexName, singular, tablesOf } from "./model";

interface ColumnDraft {
  id: number;
  name: string;
  type: string;
  pk: boolean;
  notNull: boolean;
  increment: boolean;
  default: string;
}

export const NAME = /^[A-Za-z_][A-Za-z0-9_]*$/;

/** The starting columns of a new table, in the engine's own types. */
function starter(types: ColumnType[]): ColumnDraft[] {
  const int = types.find((t) => t.integer)?.spelling ?? "bigint";
  const ts = types.find((t) => t.group === "Date & time")?.spelling ?? "timestamp";
  return [
    { id: 1, name: "id", type: int, pk: true, notNull: true, increment: true, default: "" },
    { id: 2, name: "created_at", type: ts, pk: false, notNull: true, increment: false, default: "CURRENT_TIMESTAMP" },
  ];
}

/** Enum types in the model, as a column type is written (schema-qualified outside public). */
export function enumTypes(model: Database): string[] {
  return model.schemas.flatMap((s) => (s.enums ?? []).map((e) => (s.name === "public" || s.name === model.name ? e.name : `${s.name}.${e.name}`)));
}

export function AddTableDialog({ open, onOpenChange, model, onAdd }: { open: boolean; onOpenChange: (o: boolean) => void; model: Database; onAdd: (schema: string, table: Table) => void }) {
  const types = useColumnTypes();
  const [name, setName] = React.useState("");
  // Engines without schemas (MySQL) have one schema named after the database.
  const schemaless = model.schemas.length === 1 && model.schemas[0]!.name === model.name;
  const defaultSchema = schemaless ? model.name : "public";
  const [schema, setSchema] = React.useState(defaultSchema);
  const [cols, setCols] = React.useState<ColumnDraft[]>([]);
  const [keys, setKeys] = React.useState<KeyItem[]>([]);
  const nextId = React.useRef(3);
  React.useEffect(() => {
    if (open) {
      setName("");
      setSchema(defaultSchema);
      setCols(starter(types));
      setKeys([]);
    }
  }, [open, defaultSchema]); // eslint-disable-line react-hooks/exhaustive-deps
  const taken = tablesOf(model).some((t) => t.schema === schema && t.table.name === name);
  const names = cols.map((c) => c.name);
  const dupes = new Set(names.filter((n, i) => n && names.indexOf(n) !== i));
  const badName = (n: string) => !NAME.test(n) || dupes.has(n);
  const valid = NAME.test(name) && !taken && cols.length > 0 && cols.every((c) => !badName(c.name) && c.type.trim()) && keys.every((k) => k.columns.every((c) => names.includes(c)));
  const set = (i: number, p: Partial<ColumnDraft>) =>
    setCols(
      cols.map((c, j) => {
        if (j !== i) return c;
        const next = { ...c, ...p };
        // Only integers auto-increment.
        if (!isIntegerType(next.type, types)) next.increment = false;
        return next;
      }),
    );
  const rename = (i: number, to: string) => {
    const from = cols[i]!.name;
    set(i, { name: to });
    if (from) setKeys(keys.map((k) => ({ ...k, columns: k.columns.map((c) => (c === from ? to : c)) })));
  };
  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!valid) return;
    const columns: Column[] = cols.map((c) => ({
      name: c.name,
      type: c.type.trim(),
      nullable: !(c.notNull || c.pk || c.increment),
      default: c.increment ? undefined : c.default.trim() || undefined,
      identity: c.increment ? "by default" : undefined,
    }));
    const pk = cols.filter((c) => c.pk).map((c) => c.name);
    onAdd(schema, {
      name,
      columns,
      primaryKey: pk.length ? { columns: pk } : undefined,
      uniques: keys.filter((k) => k.kind === "unique").map((k) => ({ columns: k.columns })),
      indexes: keys.filter((k) => k.kind === "index").map((k) => ({ name: indexName(name, k.columns, "idx"), columns: k.columns })),
    });
    onOpenChange(false);
  };
  const schemas = schemaless ? [model.name] : [...new Set(["public", ...model.schemas.map((s) => s.name)])];
  const pkCols = cols.filter((c) => c.pk).map((c) => c.name);
  const enums = enumTypes(model);
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent width={760} aria-describedby={undefined}>
        <form
          onSubmit={submit}
          onKeyDown={(e) => {
            // Enter picks from a suggestion list; it never submits half a table.
            if (e.key === "Enter" && (e.target as HTMLElement).getAttribute("role") === "combobox") e.preventDefault();
          }}
        >
          <DialogHead>
            <DialogTitle>New table</DialogTitle>
            <DialogDescription>Added to the model and its DBML; nothing runs until you review and apply.</DialogDescription>
          </DialogHead>
          <DialogBody>
            <div style={{ display: "grid", gridTemplateColumns: schemaless ? "minmax(0,1fr)" : "minmax(0,1fr) 180px", gap: 12 }}>
              <Field label="Name" htmlFor="nt-name" error={taken ? "A table has this name" : name && !NAME.test(name) ? "Letters, digits and _; not starting with a digit" : undefined}>
                <input id="nt-name" className="m-input m-input--mono" value={name} onChange={(e) => setName(e.target.value)} placeholder="coupons" autoFocus autoComplete="off" spellCheck={false} />
              </Field>
              {schemaless ? null : (
                <Field label="Schema">
                  <Select aria-label="Schema" mono value={schema} options={schemas.map((s) => ({ value: s, label: s }))} onChange={setSchema} />
                </Field>
              )}
            </div>
            <div className="m-field">
              <span className="m-label">Columns</span>
              <div className="m-coltable" role="table" aria-label="Columns">
                <div className="m-coltable__head" role="row">
                  <span role="columnheader">Name</span>
                  <span role="columnheader">Type</span>
                  <span role="columnheader">Default</span>
                  <span role="columnheader" title="Primary key">PK</span>
                  <span role="columnheader" title="Not null: a value is required">NN</span>
                  <span role="columnheader" title="Auto increment (integer types)">AI</span>
                  <span />
                </div>
                {cols.map((c, i) => {
                  const integer = isIntegerType(c.type, types);
                  return (
                    <div key={c.id} className="m-coltable__row" role="row">
                      <input className="m-input m-input--sm m-input--mono" aria-label="Column name" aria-invalid={!!c.name && badName(c.name)} value={c.name} placeholder="name" spellCheck={false} onChange={(e) => rename(i, e.target.value)} autoFocus={i === cols.length - 1 && i > 1} />
                      <TypeField aria-label="Column type" value={c.type} onChange={(v) => set(i, { type: v })} types={types} enums={enums} />
                      {c.increment ? <span className="m-help m-coltable__auto">auto</span> : <DefaultField aria-label="Default" value={c.default} onChange={(v) => set(i, { default: v })} type={c.type} types={types} />}
                      <input type="checkbox" aria-label="Primary key" checked={c.pk} onChange={() => set(i, { pk: !c.pk })} />
                      <input type="checkbox" aria-label="Not null" checked={c.notNull || c.pk || c.increment} disabled={c.pk || c.increment} onChange={() => set(i, { notNull: !c.notNull })} />
                      <input type="checkbox" aria-label="Auto increment" checked={c.increment} disabled={!integer} title={integer ? undefined : "Only integer columns auto-increment"} onChange={() => set(i, { increment: !c.increment })} />
                      <Button variant="ghost" size="xs" icon aria-label={`Remove ${c.name || "column"}`} onClick={() => (setCols(cols.filter((_, j) => j !== i)), setKeys(keys.map((k) => ({ ...k, columns: k.columns.filter((x) => x !== c.name) })).filter((k) => k.columns.length)))}>
                        <Trash2 className="m-icon m-icon--sm" aria-hidden="true" />
                      </Button>
                    </div>
                  );
                })}
              </div>
              <div className="m-row" style={{ justifyContent: "space-between" }}>
                <Button variant="ghost" size="sm" onClick={() => setCols([...cols, { id: nextId.current++, name: "", type: types.find((t) => t.group === "Text")?.spelling ?? "text", pk: false, notNull: false, increment: false, default: "" }])}>
                  <Plus className="m-icon m-icon--sm" aria-hidden="true" />
                  Column
                </Button>
                {pkCols.length > 1 ? <span className="m-help">Composite primary key ({pkCols.join(", ")})</span> : null}
              </div>
            </div>
            <div className="m-field">
              <span className="m-label">Unique keys and indexes</span>
              <KeyList keys={keys} columns={names} onAdd={(kind, columns) => setKeys([...keys, { kind, columns }])} onRemove={(i) => setKeys(keys.filter((_, j) => j !== i))} />
            </div>
          </DialogBody>
          <DialogFoot>
            <span className="m-spacer" />
            <Button variant="ghost" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button variant="primary" type="submit" disabled={!valid}>
              Add table
            </Button>
          </DialogFoot>
        </form>
      </DialogContent>
    </Dialog>
  );
}

export const ACTIONS = ["NO ACTION", "CASCADE", "SET NULL", "RESTRICT", "SET DEFAULT"] as const;
export type Action = (typeof ACTIONS)[number];

/** Columns a relation can refer to: the primary key and unique columns. */
function keyColumns(t: Table): string[] {
  const out = new Set<string>();
  if (t.primaryKey?.columns.length === 1) out.add(t.primaryKey.columns[0]!);
  for (const u of t.uniques ?? []) if (u.columns.length === 1) out.add(u.columns[0]!);
  for (const i of t.indexes ?? []) if (i.unique && i.columns?.length === 1) out.add(i.columns[0]!);
  return [...out];
}

export function AddRelationDialog({ open, onOpenChange, model, from, onAdd }: { open: boolean; onOpenChange: (o: boolean) => void; model: Database; from: string | null; onAdd: (key: string, fk: ForeignKey) => void }) {
  const tables = tablesOf(model);
  const [src, setSrc] = React.useState("");
  const [srcCol, setSrcCol] = React.useState("");
  const [dst, setDst] = React.useState("");
  const [dstCol, setDstCol] = React.useState("");
  const [onDelete, setOnDelete] = React.useState<Action>("NO ACTION");
  const [onUpdate, setOnUpdate] = React.useState<Action>("NO ACTION");
  const pickDst = (key: string, from = src) => {
    setDst(key);
    const t = tables.find((x) => x.key === key)?.table;
    setDstCol((t && keyColumns(t)[0]) ?? t?.columns[0]?.name ?? "");
    // Suggest the conventional column: customers → customer_id.
    const s = tables.find((x) => x.key === from)?.table;
    const guess = t && s?.columns.find((c) => c.name === `${singular(t.name)}_id`)?.name;
    if (guess) setSrcCol(guess);
  };
  React.useEffect(() => {
    if (!open) return;
    const s = from ?? tables[0]?.key ?? "";
    setSrc(s);
    const st = tables.find((t) => t.key === s);
    setSrcCol(st?.table.columns.find((c) => c.name.endsWith("_id"))?.name ?? st?.table.columns[0]?.name ?? "");
    pickDst(tables.find((t) => t.key !== s)?.key ?? s, s);
    setOnDelete("NO ACTION");
    setOnUpdate("NO ACTION");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);
  const st = tables.find((t) => t.key === src);
  const dt = tables.find((t) => t.key === dst);
  const keys = dt ? keyColumns(dt.table) : [];
  const valid = !!st && !!dt && !!srcCol && !!dstCol;
  const colLabel = (t: Table, c: Column) => `${c.name} ${c.type}${t.primaryKey?.columns.includes(c.name) ? " · pk" : ""}`;
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent width={600} aria-describedby={undefined}>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            if (!valid) return;
            onAdd(src, {
              columns: [srcCol],
              references: { schema: dt!.schema, name: dt!.table.name },
              refColumns: [dstCol],
              onDelete: onDelete === "NO ACTION" ? undefined : onDelete,
              onUpdate: onUpdate === "NO ACTION" ? undefined : onUpdate,
            });
            onOpenChange(false);
          }}
        >
          <DialogHead>
            <DialogTitle>New relation</DialogTitle>
            <DialogDescription>A foreign key from a column to the key it refers to. On the diagram you can also drag a column&apos;s dot onto another table.</DialogDescription>
          </DialogHead>
          <DialogBody>
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
              <Field label="From table">
                <Select aria-label="From table" mono value={src} options={tables.map((t) => ({ value: t.key, label: t.key }))} onChange={(v) => { setSrc(v); setSrcCol(tables.find((t) => t.key === v)?.table.columns[0]?.name ?? ""); }} />
              </Field>
              <Field label="Column">
                <Select aria-label="From column" mono value={srcCol} options={(st?.table.columns ?? []).map((c) => ({ value: c.name, label: colLabel(st!.table, c) }))} onChange={setSrcCol} />
              </Field>
              <Field label="Refers to table">
                <Select aria-label="References table" mono value={dst} options={tables.map((t) => ({ value: t.key, label: t.key }))} onChange={(v) => pickDst(v)} />
              </Field>
              <Field label="Key column" error={dt && keys.length === 0 ? "This table has no single-column primary or unique key" : undefined}>
                <Select aria-label="References column" mono value={dstCol} options={(dt?.table.columns ?? []).filter((c) => keys.length === 0 || keys.includes(c.name)).map((c) => ({ value: c.name, label: colLabel(dt!.table, c) }))} onChange={setDstCol} />
              </Field>
              <Field label="On delete">
                <Select aria-label="On delete" mono value={onDelete} options={ACTIONS.map((a) => ({ value: a, label: a }))} onChange={setOnDelete} />
              </Field>
              <Field label="On update">
                <Select aria-label="On update" mono value={onUpdate} options={ACTIONS.map((a) => ({ value: a, label: a }))} onChange={setOnUpdate} />
              </Field>
            </div>
          </DialogBody>
          <DialogFoot>
            <span className="m-spacer" />
            <Button variant="ghost" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button variant="primary" type="submit" disabled={!valid}>
              Add relation
            </Button>
          </DialogFoot>
        </form>
      </DialogContent>
    </Dialog>
  );
}

export function ImportDialog({ open, onOpenChange, onImport }: { open: boolean; onOpenChange: (o: boolean) => void; onImport: (dbml: string) => void }) {
  const [text, setText] = React.useState("");
  const fileRef = React.useRef<HTMLInputElement>(null);
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent width={640} aria-describedby={undefined}>
        <DialogHead>
          <DialogTitle>Import DBML</DialogTitle>
          <DialogDescription>Replaces the model in the editor. Paste DBML or open a .dbml file; nothing runs until you review.</DialogDescription>
        </DialogHead>
        <DialogBody>
          <textarea className="m-input m-input--mono" style={{ height: 280, padding: 8, resize: "vertical" }} value={text} onChange={(e) => setText(e.target.value)} placeholder={"Table coupons {\n  id bigint [pk, increment]\n  code text [unique, not null]\n}"} aria-label="DBML" />
          <input
            ref={fileRef}
            type="file"
            accept=".dbml,.txt"
            hidden
            onChange={async (e) => {
              const f = e.target.files?.[0];
              if (f) setText(await f.text());
            }}
          />
        </DialogBody>
        <DialogFoot>
          <Button variant="ghost" onClick={() => fileRef.current?.click()}>
            Open file…
          </Button>
          <span className="m-spacer" />
          <Button variant="ghost" onClick={() => onOpenChange(false)}>
            Cancel
          </Button>
          <Button
            variant="primary"
            disabled={!text.trim()}
            onClick={() => {
              onImport(text);
              onOpenChange(false);
            }}
          >
            Import
          </Button>
        </DialogFoot>
      </DialogContent>
    </Dialog>
  );
}
