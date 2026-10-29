/**
 * The new-table editor. Columns are rows: name, type (the engine's own, or any), default, the
 * key flags, and what the column refers to. A column's name suggests its type until a type is
 * picked, and `<table>_id` offers the relation. Unique keys and indexes span one or several
 * columns. The preview shows the SQL that applying would run, planned by the server.
 */
import * as React from "react";
import { keepPreviousData, useQuery } from "@tanstack/react-query";
import { ArrowRight, Clock, GripVertical, Link2, MessageSquare, Plus, Search, Table2, Trash2, X } from "lucide-react";

import { diffDesignerModel, printDbml } from "@/lib/api/designer/designer";
import type { Column, Database, Table } from "@/lib/api/mesaAPI.schemas";
import { errorMessage } from "@/lib/api/fetch";
import { fuzzyMatch } from "@/lib/fuzzy";
import { cn, isCmd } from "@/lib/utils";
import { shortcut } from "@/app/keys";
import { Button } from "@/ui/Button";
import { CodeBlock } from "@/ui/Code";
import { Field, Kbd, Select } from "@/ui/controls";
import { Dialog, DialogBody, DialogContent, DialogDescription, DialogFoot, DialogHead, DialogTitle } from "@/ui/primitives/dialog";
import { Popover, PopoverContent, PopoverTrigger } from "@/ui/primitives/popover";

import { useWorkspace } from "../workspace/context";
import { guessColumn, guessReference } from "./columns";
import { ACTIONS, enumTypes, NAME, type Action } from "./dialogs";
import { DefaultField, isIntegerType, KeyList, TypeField, useColumnTypes, type KeyItem } from "./fields";
import { useDebounced } from "./hooks";
import { addTable, emptyModel, indexName, referencingType, singular, tablesOf, type TableRef } from "./model";

interface Ref {
  key: string;
  schema: string;
  table: string;
  column: string;
  onDelete: Action;
  /** Index the referencing column: PostgreSQL does not, and joins and deletes need it. */
  index: boolean;
}

interface Draft {
  id: number;
  name: string;
  type: string;
  default: string;
  pk: boolean;
  notNull: boolean;
  unique: boolean;
  increment: boolean;
  comment: string;
  showComment: boolean;
  ref: Ref | null;
  /** What the person set themselves; the name's suggestions leave those alone. */
  touched: { type?: boolean; default?: boolean; notNull?: boolean };
}

const blank = (id: number, over: Partial<Draft> = {}): Draft => ({ id, name: "", type: "", default: "", pk: false, notNull: false, unique: false, increment: false, comment: "", showComment: false, ref: null, touched: {}, ...over });

function starter(types: ReturnType<typeof useColumnTypes>): Draft[] {
  const int = types.find((t) => t.integer)?.spelling ?? "bigint";
  const ts = types.find((t) => t.group === "Date & time")?.spelling ?? "timestamp";
  return [blank(1, { name: "id", type: int, pk: true, notNull: true, increment: true }), blank(2, { name: "created_at", type: ts, notNull: true, default: "CURRENT_TIMESTAMP" })];
}

interface Props {
  open: boolean;
  onOpenChange: (o: boolean) => void;
  /** The edited model: other tables to refer to, names taken. */
  model: Database;
  /** The database as it is; the preview is planned against it. */
  live: Database | undefined;
  onAdd: (schema: string, table: Table) => void;
}

export function TableDialog({ open, onOpenChange, model, live, onAdd }: Props) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent width={1080} aria-describedby={undefined} className="m-tabledlg">
        {open ? <Editor model={model} live={live} onAdd={onAdd} onClose={() => onOpenChange(false)} /> : null}
      </DialogContent>
    </Dialog>
  );
}

function Editor({ model, live, onAdd, onClose }: { model: Database; live: Database | undefined; onAdd: Props["onAdd"]; onClose: () => void }) {
  const { connectionId } = useWorkspace();
  const types = useColumnTypes();
  // Engines without schemas (MySQL) have one schema named after the database.
  const schemaless = model.schemas.length === 1 && model.schemas[0]!.name === model.name;
  const [name, setName] = React.useState("");
  const [schema, setSchema] = React.useState(schemaless ? model.name : "public");
  const [note, setNote] = React.useState("");
  const [cols, setCols] = React.useState<Draft[]>(() => starter(types));
  const [keys, setKeys] = React.useState<KeyItem[]>([]);
  const [drag, setDrag] = React.useState<{ id: number; over: number } | null>(null);
  const [focusId, setFocusId] = React.useState<number | null>(null);
  const nextId = React.useRef(3);
  const textType = types.find((t) => t.group === "Text" && t.name === "text")?.spelling ?? types.find((t) => t.group === "Text")?.spelling ?? "text";

  // The engine's types may arrive after the dialog opened.
  React.useEffect(() => {
    if (types.length) setCols((cs) => (cs.length === 2 && !cs.some((c) => c.touched.type) && cs[0]!.name === "id" ? starter(types).map((s, i) => ({ ...s, name: cs[i]!.name })) : cs));
  }, [types]);

  React.useEffect(() => {
    if (focusId === null) return;
    document.querySelector<HTMLInputElement>(`[data-col-name="${focusId}"]`)?.focus();
    setFocusId(null);
  }, [focusId]);

  const set = (id: number, patch: Partial<Draft>) =>
    setCols((cs) =>
      cs.map((c) => {
        if (c.id !== id) return c;
        const next = { ...c, ...patch, touched: { ...c.touched, ...patch.touched } };
        // Only integers auto-increment, and an auto-incremented column needs no default.
        if (!isIntegerType(next.type, types)) next.increment = false;
        return next;
      }),
    );

  const rename = (c: Draft, to: string) => {
    const g = guessColumn(to, types);
    const patch: Partial<Draft> = { name: to };
    if (!c.touched.type && !c.ref && g.type) patch.type = g.type;
    if (!c.touched.default && !c.increment) patch.default = g.default ?? "";
    if (!c.touched.notNull && !c.pk) patch.notNull = g.notNull ?? false;
    set(c.id, patch);
    if (c.name) setKeys((ks) => ks.map((k) => ({ ...k, columns: k.columns.map((x) => (x === c.name ? to : x)) })));
  };

  const link = (c: Draft, target: TableRef & { column: string }) => {
    const refCol = target.table.columns.find((x) => x.name === target.column);
    set(c.id, { ref: { key: target.key, schema: target.schema, table: target.table.name, column: target.column, onDelete: c.ref?.onDelete ?? "NO ACTION", index: c.ref?.index ?? true }, type: refCol ? referencingType(refCol.type) : c.type, increment: false, touched: { type: true } });
  };

  const add = (over: Partial<Draft> = {}, at = cols.length) => {
    const id = nextId.current++;
    setCols((cs) => [...cs.slice(0, at), blank(id, { type: textType, ...over }), ...cs.slice(at)]);
    setFocusId(id);
    return id;
  };
  const addMany = (list: Partial<Draft>[]) => {
    const fresh = list.filter((d) => !cols.some((c) => c.name === d.name)).map((d) => blank(nextId.current++, d));
    setCols((cs) => [...cs, ...fresh]);
  };
  const remove = (c: Draft) => {
    setCols((cs) => cs.filter((x) => x.id !== c.id));
    setKeys((ks) => ks.map((k) => ({ ...k, columns: k.columns.filter((x) => x !== c.name) })).filter((k) => k.columns.length));
  };
  const move = (from: number, to: number) => {
    if (from === to || to < 0 || to >= cols.length) return;
    setCols((cs) => {
      const next = [...cs];
      const [c] = next.splice(from, 1);
      next.splice(to, 0, c!);
      return next;
    });
  };

  const ts = types.find((t) => t.group === "Date & time")?.spelling ?? "timestamp";
  const presets = (
    <div className="m-tabledlg__presets">
      <Button variant="ghost" size="xs" onClick={() => add()}>
        <Plus className="m-icon m-icon--sm" aria-hidden="true" />
        Column
      </Button>
      <RefPicker
        model={model}
        self={name}
        onPick={(t) => {
          const refCol = t.table.columns.find((x) => x.name === t.column);
          const id = nextId.current++;
          let colName = `${singular(t.table.name)}_${t.column}`;
          for (let n = 2; cols.some((c) => c.name === colName); n++) colName = `${singular(t.table.name)}_${t.column}_${n}`;
          setCols((cs) => [...cs, blank(id, { name: colName, type: refCol ? referencingType(refCol.type) : textType, ref: { key: t.key, schema: t.schema, table: t.table.name, column: t.column, onDelete: "NO ACTION", index: true }, touched: { type: true } })]);
        }}
      >
        <Button variant="ghost" size="xs" disabled={tablesOf(model).length === 0}>
          <Link2 className="m-icon m-icon--sm" aria-hidden="true" />
          Reference
        </Button>
      </RefPicker>
      <Button
        variant="ghost"
        size="xs"
        onClick={() =>
          addMany([
            { name: "created_at", type: ts, default: "CURRENT_TIMESTAMP", notNull: true },
            { name: "updated_at", type: ts, default: "CURRENT_TIMESTAMP" },
          ])
        }
      >
        <Clock className="m-icon m-icon--sm" aria-hidden="true" />
        Timestamps
      </Button>
      <Button variant="ghost" size="xs" onClick={() => addMany([{ name: "deleted_at", type: ts }])}>
        <Trash2 className="m-icon m-icon--sm" aria-hidden="true" />
        Soft delete
      </Button>
    </div>
  );

  // What is wrong, row by row and overall; the first one is said next to the button.
  const taken = tablesOf(model).some((t) => t.schema === schema && t.table.name === name);
  const names = cols.map((c) => c.name);
  const dupes = new Set(names.filter((n, i) => n && names.indexOf(n) !== i));
  const rowError = (c: Draft): string | null => {
    if (!c.name) return "Needs a name";
    if (!NAME.test(c.name)) return "Letters, digits and _, not starting with a digit";
    if (dupes.has(c.name)) return "Another column has this name";
    if (!c.type.trim()) return "Needs a type";
    return null;
  };
  const nameError = !name ? null : !NAME.test(name) ? "Letters, digits and _, not starting with a digit" : taken ? `${schema}.${name} already exists` : null;
  const errors = [
    !name ? "Name the table" : nameError ? `Table name: ${nameError.charAt(0).toLowerCase()}${nameError.slice(1)}` : null,
    cols.length === 0 ? "Add a column" : null,
    ...cols.map((c, i) => (rowError(c) ? `Column ${i + 1}: ${rowError(c)!.toLowerCase()}` : null)),
  ].filter(Boolean) as string[];
  const warnings = [
    cols.length > 0 && !cols.some((c) => c.pk) ? "No primary key: Mesa's grid cannot edit the rows of a table without one." : null,
    ...cols.filter((c) => c.ref).flatMap((c) => {
      const t = tablesOf(model).find((x) => x.key === c.ref!.key);
      const rc = t?.table.columns.find((x) => x.name === c.ref!.column);
      return rc && referencingType(rc.type).toLowerCase() !== c.type.trim().toLowerCase() ? [`${c.name} is ${c.type} but ${c.ref!.table}.${c.ref!.column} is ${rc.type}; the database may refuse the key.`] : [];
    }),
  ].filter(Boolean) as string[];

  const build = (): Table => {
    const columns: Column[] = cols.map((c) => ({
      name: c.name,
      type: c.type.trim(),
      nullable: !(c.notNull || c.pk || c.increment),
      default: c.increment ? undefined : c.default.trim() || undefined,
      identity: c.increment ? "by default" : undefined,
      comment: c.comment.trim() || undefined,
    }));
    const pk = cols.filter((c) => c.pk).map((c) => c.name);
    return {
      name,
      comment: note.trim() || undefined,
      columns,
      primaryKey: pk.length ? { columns: pk } : undefined,
      uniques: [...cols.filter((c) => c.unique && !c.pk).map((c) => ({ columns: [c.name] })), ...keys.filter((k) => k.kind === "unique").map((k) => ({ columns: k.columns }))],
      indexes: [
        ...keys.filter((k) => k.kind === "index").map((k) => ({ name: indexName(name, k.columns, "idx"), columns: k.columns })),
        // Referencing columns that no key already leads with get an index of their own.
        ...cols.filter((c) => c.ref?.index && !c.unique && !(c.pk && pk[0] === c.name) && !keys.some((k) => k.columns[0] === c.name)).map((c) => ({ name: indexName(name, [c.name], "idx"), columns: [c.name] })),
      ],
      foreignKeys: cols
        .filter((c) => c.ref)
        .map((c) => ({ columns: [c.name], references: { schema: c.ref!.schema, name: c.ref!.table }, refColumns: [c.ref!.column], onDelete: c.ref!.onDelete === "NO ACTION" ? undefined : c.ref!.onDelete })),
    };
  };

  const submit = () => {
    if (errors.length) return;
    onAdd(schema, build());
    onClose();
  };

  const schemas = schemaless ? [model.name] : [...new Set(["public", ...model.schemas.map((s) => s.name)])];
  const enums = enumTypes(model);
  const pkCols = cols.filter((c) => c.pk).map((c) => c.name);

  return (
    <form
      className="m-tabledlg__form"
      onSubmit={(e) => {
        e.preventDefault();
        submit();
      }}
      onKeyDown={(e) => {
        if (isCmd(e) && e.key === "Enter") {
          e.preventDefault();
          submit();
          return;
        }
        // Enter picks from a suggestion list; it never submits half a table.
        if (e.key === "Enter" && (e.target as HTMLElement).getAttribute("role") === "combobox") e.preventDefault();
      }}
    >
      <DialogHead>
        <DialogTitle>New table</DialogTitle>
        <DialogDescription>Added to the model and its DBML; nothing runs until you review and apply.</DialogDescription>
      </DialogHead>
      <DialogBody className="m-tabledlg__body">
        <div className="m-tabledlg__top" style={{ gridTemplateColumns: schemaless ? "minmax(0,1fr) minmax(0,1.4fr)" : "minmax(0,1fr) 180px minmax(0,1.4fr)" }}>
          <Field label="Name" htmlFor="nt-name" error={nameError ?? undefined}>
            <input id="nt-name" className="m-input m-input--mono" value={name} onChange={(e) => setName(e.target.value)} placeholder="coupons" autoFocus autoComplete="off" spellCheck={false} />
          </Field>
          {schemaless ? null : (
            <Field label="Schema">
              <Select aria-label="Schema" mono value={schema} options={schemas.map((s) => ({ value: s, label: s }))} onChange={setSchema} />
            </Field>
          )}
          <Field label="Note" htmlFor="nt-note">
            <input id="nt-note" className="m-input" value={note} onChange={(e) => setNote(e.target.value)} placeholder="What this table holds (optional)" autoComplete="off" />
          </Field>
        </div>

        <div className="m-field">
          <div className="m-tabledlg__bar">
            <span className="m-label">Columns</span>
            <span className="m-help m-tabledlg__legend">
              <b>PK</b> primary key · <b>NN</b> not null · <b>UQ</b> unique · <b>AI</b> auto increment
            </span>
            <span className="m-spacer" />
            {presets}
          </div>
          <div className="m-colgrid" role="table" aria-label="Columns">
            <div className="m-colgrid__head" role="row">
              <span />
              <span role="columnheader">Name</span>
              <span role="columnheader">Type</span>
              <span role="columnheader">Default</span>
              <span role="columnheader">Flags</span>
              <span role="columnheader">Refers to</span>
              <span />
              <span />
            </div>
            {cols.map((c, i) => {
              const err = c.name || c.type ? rowError(c) : null;
              const integer = isIntegerType(c.type, types);
              const suggestion = !c.ref ? guessReference(c.name, model, schema, name) : undefined;
              return (
                <div
                  key={c.id}
                  className={cn("m-colgrid__item", drag && drag.over === i && drag.id !== c.id && (cols.findIndex((x) => x.id === drag.id) < i ? "is-drop-after" : "is-drop-before"), drag?.id === c.id && "is-dragging")}
                  onDragOver={(e) => {
                    if (!drag) return;
                    e.preventDefault();
                    if (drag.over !== i) setDrag({ ...drag, over: i });
                  }}
                  onDrop={(e) => {
                    e.preventDefault();
                    if (drag) move(cols.findIndex((x) => x.id === drag.id), i);
                    setDrag(null);
                  }}
                >
                  <div className="m-colgrid__row" role="row">
                    <span
                      className="m-colgrid__handle"
                      draggable
                      title="Drag to reorder (or Alt+↑ ↓ in the name)"
                      aria-label={`Move ${c.name || "column"}`}
                      onDragStart={(e) => {
                        e.dataTransfer.effectAllowed = "move";
                        e.dataTransfer.setData("text/plain", String(c.id));
                        setDrag({ id: c.id, over: i });
                      }}
                      onDragEnd={() => setDrag(null)}
                    >
                      <GripVertical className="m-icon m-icon--sm" aria-hidden="true" />
                    </span>
                    <input
                      data-col-name={c.id}
                      className="m-input m-input--sm m-input--mono"
                      aria-label="Column name"
                      aria-invalid={!!err && !!c.name}
                      value={c.name}
                      placeholder="name"
                      spellCheck={false}
                      autoComplete="off"
                      onChange={(e) => rename(c, e.target.value)}
                      onKeyDown={(e) => {
                        if (e.altKey && (e.key === "ArrowUp" || e.key === "ArrowDown")) {
                          e.preventDefault();
                          move(i, i + (e.key === "ArrowUp" ? -1 : 1));
                          setFocusId(c.id);
                        } else if (e.key === "Enter" && !isCmd(e)) {
                          // Enter goes to the next column's name, adding one at the end.
                          e.preventDefault();
                          if (i === cols.length - 1) add({}, i + 1);
                          else setFocusId(cols[i + 1]!.id);
                        }
                      }}
                    />
                    <TypeField aria-label="Column type" value={c.type} onChange={(v) => set(c.id, { type: v, touched: { type: true } })} types={types} enums={enums} />
                    {c.increment ? (
                      <span className="m-help m-colgrid__auto" title="Values come from a sequence">
                        auto
                      </span>
                    ) : (
                      <DefaultField aria-label="Default" value={c.default} onChange={(v) => set(c.id, { default: v, touched: { default: true } })} type={c.type} types={types} />
                    )}
                    <div className="m-flags" role="group" aria-label="Flags">
                      <Flag label="PK" title="Primary key" on={c.pk} onClick={() => set(c.id, { pk: !c.pk })} />
                      <Flag label="NN" title="Not null: a value is required" on={c.notNull || c.pk || c.increment} disabled={c.pk || c.increment} onClick={() => set(c.id, { notNull: !c.notNull, touched: { notNull: true } })} />
                      <Flag label="UQ" title="Unique: no two rows share a value" on={c.unique || (c.pk && pkCols.length === 1)} disabled={c.pk} onClick={() => set(c.id, { unique: !c.unique })} />
                      <Flag label="AI" title={integer ? "Auto increment (identity / AUTO_INCREMENT)" : "Only integer columns auto-increment"} on={c.increment} disabled={!integer} onClick={() => set(c.id, { increment: !c.increment })} />
                    </div>
                    <div className="m-colgrid__ref">
                      {c.ref ? (
                        <RefPicker model={model} self={name} column={c.name} current={c.ref} onPick={(t) => link(c, t)} onChange={(patch) => set(c.id, { ref: { ...c.ref!, ...patch } })} onUnlink={() => set(c.id, { ref: null })}>
                          <button type="button" className="m-refchip is-on" title={`Refers to ${c.ref.schema}.${c.ref.table}.${c.ref.column}${c.ref.onDelete !== "NO ACTION" ? `, on delete ${c.ref.onDelete.toLowerCase()}` : ""}${c.ref.index ? ", indexed" : ""}`}>
                            <ArrowRight className="m-icon m-icon--sm" aria-hidden="true" />
                            <span>
                              {c.ref.table}.{c.ref.column}
                            </span>
                          </button>
                        </RefPicker>
                      ) : suggestion ? (
                        <span className="m-refchip-group">
                          <button type="button" className="m-refchip is-suggested" title={`Refer to ${suggestion.key}.${suggestion.column}`} onClick={() => link(c, suggestion)}>
                            <ArrowRight className="m-icon m-icon--sm" aria-hidden="true" />
                            <span>{suggestion.table.name}?</span>
                          </button>
                          <RefPicker model={model} self={name} onPick={(t) => link(c, t)}>
                            <Button variant="ghost" size="xs" icon aria-label="Refer to another table" title="Refer to another table">
                              <Search className="m-icon m-icon--sm" aria-hidden="true" />
                            </Button>
                          </RefPicker>
                        </span>
                      ) : (
                        <RefPicker model={model} self={name} onPick={(t) => link(c, t)}>
                          <button type="button" className="m-refchip" title="Refer to a column of another table">
                            <Link2 className="m-icon m-icon--sm" aria-hidden="true" />
                            <span>none</span>
                          </button>
                        </RefPicker>
                      )}
                    </div>
                    <Button variant="ghost" size="xs" icon aria-label={c.showComment ? "Hide the comment" : "Add a comment"} aria-pressed={c.showComment || !!c.comment} className={cn((c.showComment || c.comment) && "is-on")} title={c.comment || "Comment"} onClick={() => set(c.id, { showComment: !c.showComment })}>
                      <MessageSquare className="m-icon m-icon--sm" aria-hidden="true" />
                    </Button>
                    <Button variant="ghost" size="xs" icon aria-label={`Remove ${c.name || "column"}`} onClick={() => remove(c)}>
                      <X className="m-icon m-icon--sm" aria-hidden="true" />
                    </Button>
                  </div>
                  {c.showComment ? (
                    <input className="m-input m-input--sm m-colgrid__comment" aria-label={`Comment on ${c.name || "column"}`} value={c.comment} placeholder="What this column holds" onChange={(e) => set(c.id, { comment: e.target.value })} autoFocus />
                  ) : null}
                  {err && c.name ? <span className="m-help m-help--error m-colgrid__err">{err}</span> : null}
                </div>
              );
            })}
          </div>
          <div className="m-row" style={{ justifyContent: "space-between" }}>
            <Button variant="ghost" size="sm" onClick={() => add()}>
              <Plus className="m-icon m-icon--sm" aria-hidden="true" />
              Column
            </Button>
            {pkCols.length > 1 ? <span className="m-help">Composite primary key ({pkCols.join(", ")})</span> : null}
          </div>
        </div>

        <div className="m-field">
          <span className="m-label">Indexes and unique keys</span>
          <KeyList keys={keys} columns={names} onAdd={(kind, columns) => setKeys([...keys, { kind, columns }])} onRemove={(i) => setKeys(keys.filter((_, j) => j !== i))} />
        </div>

        {warnings.length ? (
          <div className="m-tabledlg__warn">
            {warnings.map((w) => (
              <span key={w} className="m-status" style={{ whiteSpace: "normal", alignItems: "flex-start" }}>
                <span className="m-dot m-dot--degraded" style={{ marginTop: 5 }} />
                {w}
              </span>
            ))}
          </div>
        ) : null}

        <Preview connectionId={connectionId} model={model} live={live} schema={schema} table={errors.length ? null : build()} />
      </DialogBody>
      <DialogFoot>
        {errors.length && (name || cols.some((c) => c.name !== "id" && c.name !== "created_at")) ? <span className="m-help m-help--error m-tabledlg__first">{errors[0]}</span> : null}
        <span className="m-spacer" />
        <Button variant="ghost" onClick={onClose}>
          Cancel
        </Button>
        <Button variant="primary" type="submit" disabled={errors.length > 0} title={errors[0]}>
          Add table <Kbd>{shortcut("Enter")}</Kbd>
        </Button>
      </DialogFoot>
    </form>
  );
}

function Flag({ label, title, on, disabled, onClick }: { label: string; title: string; on: boolean; disabled?: boolean; onClick: () => void }) {
  return (
    <button type="button" className={cn("m-flag", on && "is-on")} aria-pressed={on} aria-label={title} title={title} disabled={disabled} onClick={onClick}>
      {label}
    </button>
  );
}

/** Choose the table and key column a column refers to, and what deleting a referenced row does. */
function RefPicker({ model, self, column, current, onPick, onChange, onUnlink, children }: { model: Database; self: string; column?: string; current?: Ref; onPick: (t: TableRef & { column: string }) => void; onChange?: (patch: Partial<Ref>) => void; onUnlink?: () => void; children: React.ReactElement }) {
  const [open, setOpen] = React.useState(false);
  const [q, setQ] = React.useState("");
  const [hi, setHi] = React.useState(0);
  const tables = tablesOf(model).filter((t) => t.table.name !== self);
  const query = q.trim().toLowerCase().replace(/\s+/g, "");
  const shown = (query ? tables.map((t) => ({ t, m: fuzzyMatch(query, t.table.name) })).filter((x) => x.m).sort((a, b) => b.m!.score - a.m!.score).map((x) => x.t) : tables).slice(0, 50);
  const keysOf = (t: Table) => {
    const out = new Set<string>();
    if (t.primaryKey?.columns.length === 1) out.add(t.primaryKey.columns[0]!);
    for (const u of t.uniques ?? []) if (u.columns.length === 1) out.add(u.columns[0]!);
    for (const i of t.indexes ?? []) if (i.unique && i.columns?.length === 1) out.add(i.columns[0]!);
    return [...out];
  };
  const pick = (t: TableRef) => {
    const k = keysOf(t.table)[0];
    if (!k) return;
    onPick({ ...t, column: k });
    if (!current) setOpen(false);
  };
  const cur = current ? tables.find((t) => t.key === current.key) : undefined;
  return (
    <Popover
      open={open}
      onOpenChange={(o) => {
        setOpen(o);
        setQ("");
        setHi(0);
      }}
    >
      <PopoverTrigger asChild>{children}</PopoverTrigger>
      <PopoverContent className="m-refpick" align="end">
        {current && cur ? (
          <div className="m-refpick__current">
            <div className="m-row" style={{ flexWrap: "nowrap" }}>
              <Table2 className="m-icon m-icon--sm m-muted" aria-hidden="true" />
              <span className="m-mono" style={{ minWidth: 0, overflow: "hidden", textOverflow: "ellipsis" }}>
                {current.schema}.{current.table}
              </span>
            </div>
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 8 }}>
              <Field label="Key column">
                <Select size="sm" mono aria-label="Key column" value={current.column} options={keysOf(cur.table).map((k) => ({ value: k, label: k }))} onChange={(v) => onPick({ ...cur, column: v })} />
              </Field>
              <Field label="On delete">
                <Select size="sm" mono aria-label="On delete" value={current.onDelete} options={ACTIONS.map((a) => ({ value: a, label: a }))} onChange={(v) => onChange?.({ onDelete: v })} />
              </Field>
            </div>
            <label className="m-row" style={{ flexWrap: "nowrap", alignItems: "flex-start", cursor: "pointer" }}>
              <input type="checkbox" checked={current.index} onChange={() => onChange?.({ index: !current.index })} style={{ width: 16, height: 16, margin: "1px 0 0", accentColor: "var(--primary)" }} />
              <span className="m-help" style={{ color: "var(--foreground)" }}>
                Index <span className="m-mono">{column || "this column"}</span>
                <span className="m-help" style={{ display: "block" }}>
                  Keeps joins, and deletes in {current.table}, fast. PostgreSQL does not index it by itself.
                </span>
              </span>
            </label>
            <div className="m-row">
              <Button variant="secondary" size="xs" onClick={() => setOpen(false)}>
                Done
              </Button>
              <Button variant="ghost" size="xs" style={{ color: "var(--destructive)" }} onClick={() => (onUnlink?.(), setOpen(false))}>
                Remove the reference
              </Button>
            </div>
            <div className="m-menu__sep" />
            <div className="m-combo__group">Or refer to another table</div>
          </div>
        ) : null}
        <div className="m-refpick__search">
          <Search className="m-icon m-icon--sm m-muted" aria-hidden="true" />
          <input
            autoFocus
            className="m-erd-find__input"
            placeholder="Find a table…"
            aria-label="Find a table to refer to"
            value={q}
            onChange={(e) => (setQ(e.target.value), setHi(0))}
            onKeyDown={(e) => {
              if (e.key === "ArrowDown" || e.key === "ArrowUp") {
                e.preventDefault();
                const n = shown.length;
                if (n) setHi((h) => (h + (e.key === "ArrowDown" ? 1 : -1) + n) % n);
              } else if (e.key === "Enter") {
                e.preventDefault();
                e.stopPropagation();
                if (shown[hi]) pick(shown[hi]!);
              }
            }}
          />
        </div>
        <div className="m-refpick__list" role="listbox" aria-label="Tables">
          {shown.length === 0 ? <div className="m-help" style={{ padding: "6px 8px" }}>{tables.length ? "No table matches." : "No other tables yet."}</div> : null}
          {shown.map((t, i) => {
            const k = keysOf(t.table);
            return (
              <button key={t.key} type="button" role="option" aria-selected={i === hi} disabled={k.length === 0} className={cn("m-menu__item", i === hi && "is-selected", current?.key === t.key && "is-current")} onMouseEnter={() => setHi(i)} onClick={() => pick(t)} title={k.length ? undefined : "No single-column primary or unique key to refer to"}>
                <Table2 className="m-icon m-icon--sm" aria-hidden="true" />
                <span className="m-mono" style={{ minWidth: 0, overflow: "hidden", textOverflow: "ellipsis" }}>
                  {t.table.name}
                </span>
                <span className="m-help m-mono" style={{ marginLeft: "auto" }}>
                  {k[0] ?? "no key"}
                </span>
              </button>
            );
          })}
        </div>
      </PopoverContent>
    </Popover>
  );
}

/** The SQL applying this table would run (planned by the server against the live schema), or its DBML. */
function Preview({ connectionId, model, live, schema, table }: { connectionId: string; model: Database; live: Database | undefined; schema: string; table: Table | null }) {
  const [mode, setMode] = React.useState<"sql" | "dbml" | null>("sql");
  const settled = useDebounced(JSON.stringify(table ? [schema, table] : null), 400);
  const draft = React.useMemo(() => JSON.parse(settled) as [string, Table] | null, [settled]);
  const sql = useQuery({
    queryKey: ["table-preview", "sql", connectionId, model.name, settled],
    queryFn: () => diffDesignerModel(connectionId, model.name, { schema: addTable(live ?? emptyModel(model.name), draft![0], draft![1]) }),
    enabled: mode === "sql" && !!draft,
    placeholderData: keepPreviousData,
    retry: false,
  });
  const dbml = useQuery({
    queryKey: ["table-preview", "dbml", settled],
    queryFn: () => printDbml({ schema: { name: model.name, schemas: [{ name: draft![0], tables: [draft![1]] }] } }),
    enabled: mode === "dbml" && !!draft,
    placeholderData: keepPreviousData,
    retry: false,
  });
  const q = mode === "dbml" ? dbml : sql;
  const text = mode === "dbml" ? (dbml.data?.dbml ?? "").replace(/^Project[^{]*\{[^}]*\}\s*/m, "").trim() : (sql.data?.upSql ?? "").trim();
  return (
    <div className="m-field">
      <div className="m-row">
        <span className="m-label">Preview</span>
        <div className="m-seg" role="tablist" aria-label="Preview">
          <button type="button" role="tab" aria-selected={mode === "sql"} className={cn("m-seg__item", mode === "sql" && "is-active")} onClick={() => setMode(mode === "sql" ? null : "sql")}>
            SQL
          </button>
          <button type="button" role="tab" aria-selected={mode === "dbml"} className={cn("m-seg__item", mode === "dbml" && "is-active")} onClick={() => setMode(mode === "dbml" ? null : "dbml")}>
            DBML
          </button>
        </div>
        <span className="m-help">{mode === "sql" ? "What applying runs, planned against the live database" : mode === "dbml" ? "How the table reads in the model" : "Hidden"}</span>
      </div>
      {mode ? (
        !draft ? (
          <p className="m-help" style={{ margin: 0 }}>
            {table ? "Planning…" : "Shown once the table has a name and its columns are complete."}
          </p>
        ) : q.isError ? (
          <p className="m-help m-help--error" style={{ margin: 0 }}>
            {errorMessage(q.error)}
          </p>
        ) : (
          <CodeBlock code={text || (q.isFetching ? "…" : "")} language={mode} className={cn("m-code--wrap", q.isPlaceholderData && "is-refetching")} style={{ maxHeight: 220 }} />
        )
      ) : null}
    </div>
  );
}
