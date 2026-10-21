/** Designer dialogs: new table, new relation, import DBML. Each edits the model; the DBML follows. */
import * as React from "react";
import { Plus, Trash2 } from "lucide-react";

import type { Column, Database, ForeignKey, Table } from "@/lib/api/mesaAPI.schemas";
import { Button } from "@/ui/Button";
import { Field, Select } from "@/ui/controls";
import { Dialog, DialogBody, DialogContent, DialogDescription, DialogFoot, DialogHead, DialogTitle } from "@/ui/primitives/dialog";

import { tablesOf } from "./model";

const TYPES = ["bigint", "integer", "text", "varchar(255)", "boolean", "numeric(12,2)", "timestamptz", "date", "uuid", "jsonb"];

interface ColumnDraft {
  name: string;
  type: string;
  pk: boolean;
  notNull: boolean;
  unique: boolean;
  default: string;
}

const NAME = /^[A-Za-z_][A-Za-z0-9_]*$/;

export function AddTableDialog({ open, onOpenChange, model, onAdd }: { open: boolean; onOpenChange: (o: boolean) => void; model: Database; onAdd: (schema: string, table: Table) => void }) {
  const [name, setName] = React.useState("");
  const [schema, setSchema] = React.useState("public");
  const [cols, setCols] = React.useState<ColumnDraft[]>([]);
  React.useEffect(() => {
    if (open) {
      setName("");
      setSchema("public");
      setCols([
        { name: "id", type: "bigint", pk: true, notNull: true, unique: false, default: "" },
        { name: "created_at", type: "timestamptz", pk: false, notNull: true, unique: false, default: "now()" },
      ]);
    }
  }, [open]);
  const taken = tablesOf(model).some((t) => t.schema === schema && t.table.name === name);
  const valid = NAME.test(name) && !taken && cols.length > 0 && cols.every((c) => NAME.test(c.name) && c.type.trim()) && new Set(cols.map((c) => c.name)).size === cols.length;
  const set = (i: number, p: Partial<ColumnDraft>) => setCols(cols.map((c, j) => (j === i ? { ...c, ...p } : c)));
  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    if (!valid) return;
    const columns: Column[] = cols.map((c) => ({
      name: c.name,
      type: c.type.trim(),
      nullable: !(c.notNull || c.pk),
      default: c.default.trim() || undefined,
      identity: c.pk && /^(bigint|integer|int|int8|int4|smallint)$/i.test(c.type.trim()) && !c.default.trim() ? "by default" : undefined,
    }));
    const pk = cols.filter((c) => c.pk).map((c) => c.name);
    onAdd(schema, {
      name,
      columns,
      primaryKey: pk.length ? { columns: pk } : undefined,
      uniques: cols.filter((c) => c.unique && !c.pk).map((c) => ({ columns: [c.name] })),
    });
    onOpenChange(false);
  };
  const schemas = [...new Set(["public", ...model.schemas.map((s) => s.name)])];
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent width={640} aria-describedby={undefined}>
        <form onSubmit={submit}>
          <DialogHead>
            <DialogTitle>New table</DialogTitle>
            <DialogDescription>The table is added to the model and its DBML; nothing runs until you review and apply.</DialogDescription>
          </DialogHead>
          <DialogBody>
            <div style={{ display: "grid", gridTemplateColumns: "minmax(0,1fr) 160px", gap: 12 }}>
              <Field label="Name" htmlFor="nt-name" error={taken ? "A table has this name" : undefined}>
                <input id="nt-name" className="m-input m-input--mono" value={name} onChange={(e) => setName(e.target.value)} placeholder="coupons" autoFocus autoComplete="off" />
              </Field>
              <Field label="Schema">
                <Select aria-label="Schema" mono value={schema} options={schemas.map((s) => ({ value: s, label: s }))} onChange={setSchema} />
              </Field>
            </div>
            <div className="m-field">
              <span className="m-label">Columns</span>
              <div style={{ display: "grid", gridTemplateColumns: "minmax(0,1.2fr) minmax(0,1fr) minmax(0,1fr) 36px 36px 36px 28px", gap: 6, alignItems: "center" }}>
                <span className="m-help">Name</span>
                <span className="m-help">Type</span>
                <span className="m-help">Default</span>
                <span className="m-help" title="Primary key">PK</span>
                <span className="m-help" title="Not null">NN</span>
                <span className="m-help" title="Unique">UQ</span>
                <span />
                {cols.map((c, i) => (
                  <React.Fragment key={i}>
                    <input className="m-input m-input--sm m-input--mono" aria-label="Column name" value={c.name} onChange={(e) => set(i, { name: e.target.value })} />
                    <input className="m-input m-input--sm m-input--mono" aria-label="Column type" value={c.type} list="designer-types" onChange={(e) => set(i, { type: e.target.value })} />
                    <input className="m-input m-input--sm m-input--mono" aria-label="Default" value={c.default} onChange={(e) => set(i, { default: e.target.value })} />
                    <input type="checkbox" aria-label="Primary key" checked={c.pk} onChange={() => set(i, { pk: !c.pk })} style={{ accentColor: "var(--primary)" }} />
                    <input type="checkbox" aria-label="Not null" checked={c.notNull || c.pk} disabled={c.pk} onChange={() => set(i, { notNull: !c.notNull })} style={{ accentColor: "var(--primary)" }} />
                    <input type="checkbox" aria-label="Unique" checked={c.unique} onChange={() => set(i, { unique: !c.unique })} style={{ accentColor: "var(--primary)" }} />
                    <Button variant="ghost" size="xs" icon aria-label={`Remove ${c.name}`} onClick={() => setCols(cols.filter((_, j) => j !== i))}>
                      <Trash2 className="m-icon m-icon--sm" aria-hidden="true" />
                    </Button>
                  </React.Fragment>
                ))}
              </div>
              <datalist id="designer-types">
                {TYPES.map((t) => (
                  <option key={t} value={t} />
                ))}
              </datalist>
              <div>
                <Button variant="ghost" size="sm" onClick={() => setCols([...cols, { name: "", type: "text", pk: false, notNull: false, unique: false, default: "" }])}>
                  <Plus className="m-icon m-icon--sm" aria-hidden="true" />
                  Column
                </Button>
              </div>
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

const ACTIONS = ["NO ACTION", "CASCADE", "SET NULL", "RESTRICT", "SET DEFAULT"] as const;

export function AddRelationDialog({ open, onOpenChange, model, from, onAdd }: { open: boolean; onOpenChange: (o: boolean) => void; model: Database; from: string | null; onAdd: (key: string, fk: ForeignKey) => void }) {
  const tables = tablesOf(model);
  const [src, setSrc] = React.useState("");
  const [srcCol, setSrcCol] = React.useState("");
  const [dst, setDst] = React.useState("");
  const [dstCol, setDstCol] = React.useState("");
  const [onDelete, setOnDelete] = React.useState<(typeof ACTIONS)[number]>("NO ACTION");
  React.useEffect(() => {
    if (!open) return;
    const s = from ?? tables[0]?.key ?? "";
    setSrc(s);
    const st = tables.find((t) => t.key === s);
    setSrcCol(st?.table.columns.find((c) => c.name.endsWith("_id"))?.name ?? st?.table.columns[0]?.name ?? "");
    const d = tables.find((t) => t.key !== s)?.key ?? s;
    setDst(d);
    const dt = tables.find((t) => t.key === d);
    setDstCol(dt?.table.primaryKey?.columns[0] ?? dt?.table.columns[0]?.name ?? "");
    setOnDelete("NO ACTION");
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);
  const st = tables.find((t) => t.key === src);
  const dt = tables.find((t) => t.key === dst);
  const valid = !!st && !!dt && !!srcCol && !!dstCol;
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent width={560} aria-describedby={undefined}>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            if (!valid) return;
            onAdd(src, { columns: [srcCol], references: { schema: dt!.schema, name: dt!.table.name }, refColumns: [dstCol], onDelete: onDelete === "NO ACTION" ? undefined : onDelete });
            onOpenChange(false);
          }}
        >
          <DialogHead>
            <DialogTitle>New relation</DialogTitle>
            <DialogDescription>A foreign key from a column to the column it refers to.</DialogDescription>
          </DialogHead>
          <DialogBody>
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 12 }}>
              <Field label="From table">
                <Select aria-label="From table" mono value={src} options={tables.map((t) => ({ value: t.key, label: t.key }))} onChange={(v) => { setSrc(v); setSrcCol(tables.find((t) => t.key === v)?.table.columns[0]?.name ?? ""); }} />
              </Field>
              <Field label="Column">
                <Select aria-label="From column" mono value={srcCol} options={(st?.table.columns ?? []).map((c) => ({ value: c.name, label: `${c.name} ${c.type}` }))} onChange={setSrcCol} />
              </Field>
              <Field label="References table">
                <Select aria-label="References table" mono value={dst} options={tables.map((t) => ({ value: t.key, label: t.key }))} onChange={(v) => { setDst(v); const t = tables.find((x) => x.key === v); setDstCol(t?.table.primaryKey?.columns[0] ?? t?.table.columns[0]?.name ?? ""); }} />
              </Field>
              <Field label="Column">
                <Select aria-label="References column" mono value={dstCol} options={(dt?.table.columns ?? []).map((c) => ({ value: c.name, label: `${c.name} ${c.type}` }))} onChange={setDstCol} />
              </Field>
            </div>
            <Field label="On delete">
              <Select aria-label="On delete" mono value={onDelete} options={ACTIONS.map((a) => ({ value: a, label: a }))} onChange={setOnDelete} />
            </Field>
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
