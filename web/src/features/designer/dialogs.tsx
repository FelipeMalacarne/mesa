/** Designer dialogs: new relation, import DBML (the new table has TableDialog). Each edits the model; the DBML follows. */
import * as React from "react";

import type { Column, Database, ForeignKey, Table } from "@/lib/api/mesaAPI.schemas";
import { Button } from "@/ui/Button";
import { Field, Select } from "@/ui/controls";
import { Dialog, DialogBody, DialogContent, DialogDescription, DialogFoot, DialogHead, DialogTitle } from "@/ui/primitives/dialog";

import { singular, tablesOf } from "./model";

export const NAME = /^[A-Za-z_][A-Za-z0-9_]*$/;

/** Enum types in the model, as a column type is written (schema-qualified outside public). */
export function enumTypes(model: Database): string[] {
  return model.schemas.flatMap((s) => (s.enums ?? []).map((e) => (s.name === "public" || s.name === model.name ? e.name : `${s.name}.${e.name}`)));
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
