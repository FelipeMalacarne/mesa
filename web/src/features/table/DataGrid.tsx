/**
 * The editable data grid (`DataGrid` component, `Main` board). Edits are staged in the changes
 * store and only reach the database through the review dialog.
 *
 * Keys: arrows move, Enter or typing edits, Escape cancels, Tab moves right, ⌘⌫ marks the row
 * for deletion, ⌘C copies the cell. Double-click edits too.
 */
import * as React from "react";
import { ArrowDown, ArrowUp, KeyRound, Link2 } from "lucide-react";
import { toast } from "sonner";

import type { ResultColumn, Table } from "@/lib/api/mesaAPI.schemas";
import { cn, isCmd } from "@/lib/utils";
import { isNull, isNumeric, NULL_INPUT, parseInput, textOf, ValueInputError, type Value } from "@/lib/values";

import { rowKeyOf, useChanges, type RowValues, type TableChanges } from "./changes.store";

export interface GridRow {
  /** "r:<index>" for loaded rows, "i:<insertId>" for staged inserts. */
  id: string;
  kind: "row" | "insert";
  values: Value[];
  key?: RowValues;
  insertId?: string;
}

export interface SortState {
  column: string;
  desc: boolean;
}

interface Props {
  columns: ResultColumn[];
  rows: Value[][];
  offset: number;
  meta?: Table;
  tableKey: string;
  changes: TableChanges;
  editable: boolean;
  sort: SortState | null;
  onSort: (sort: SortState | null) => void;
  selectedRow: string | null;
  onSelectRow: (id: string | null) => void;
  refetching: boolean;
}

/** The grid's rows: loaded rows then staged inserts, with keys for editing. */
export function gridRows(columns: ResultColumn[], rows: Value[][], meta: Table | undefined, changes: TableChanges): GridRow[] {
  const pk = meta?.primaryKey?.columns ?? [];
  const index = new Map(columns.map((c, i) => [c.name, i]));
  const out: GridRow[] = rows.map((values, i) => {
    let key: RowValues | undefined;
    if (pk.length && pk.every((c) => index.has(c))) {
      key = {};
      for (const c of pk) key[c] = values[index.get(c)!]!;
    }
    return { id: `r:${i}`, kind: "row", values, key };
  });
  for (const ins of changes.inserts) {
    out.push({ id: `i:${ins.id}`, kind: "insert", insertId: ins.id, values: columns.map((c) => ins.values[c.name] ?? { k: "null" }) });
  }
  return out;
}

/** The value shown for a cell, staged edits applied. */
export function cellValue(row: GridRow, col: number, columns: ResultColumn[], changes: TableChanges): { value: Value; dirty: boolean; unset: boolean } {
  const name = columns[col]!.name;
  if (row.kind === "insert") {
    const ins = changes.inserts.find((i) => i.id === row.insertId);
    const v = ins?.values[name];
    return { value: v ?? { k: "null" }, dirty: !!v, unset: !v };
  }
  if (row.key) {
    const u = changes.updates[rowKeyOf(row.key)];
    if (u && name in u.values) return { value: u.values[name]!, dirty: true, unset: false };
  }
  return { value: row.values[col] ?? { k: "null" }, dirty: false, unset: false };
}

interface Active {
  row: number;
  col: number;
}

export function DataGrid({ columns, rows, offset, meta, tableKey, changes, editable, sort, onSort, selectedRow, onSelectRow, refetching }: Props) {
  const list = React.useMemo(() => gridRows(columns, rows, meta, changes), [columns, rows, meta, changes]);
  const setCell = useChanges((s) => s.setCell);
  const setInsertCell = useChanges((s) => s.setInsertCell);
  const toggleDelete = useChanges((s) => s.toggleDelete);
  const removeInsert = useChanges((s) => s.removeInsert);
  const [active, setActive] = React.useState<Active | null>(null);
  const [editing, setEditing] = React.useState<{ row: number; col: number; text: string } | null>(null);
  const gridRef = React.useRef<HTMLTableElement>(null);
  const metaCol = React.useMemo(() => new Map((meta?.columns ?? []).map((c) => [c.name, c])), [meta]);
  const pk = new Set(meta?.primaryKey?.columns ?? []);
  const fk = new Set((meta?.foreignKeys ?? []).flatMap((f) => f.columns));

  // A new row gets focus so it is visible and ready to type into.
  const inserts = changes.inserts.length;
  const prevInserts = React.useRef(inserts);
  React.useEffect(() => {
    if (inserts > prevInserts.current) {
      const firstEditable = Math.max(0, columns.findIndex((c) => { const m = metaCol.get(c.name); return !m?.identity && !m?.default; }));
      setActive({ row: list.length - 1, col: firstEditable });
      gridRef.current?.focus();
    }
    prevInserts.current = inserts;
  }, [inserts, list.length, columns, metaCol]);

  // Keep the inspector's row in step with the active cell.
  React.useEffect(() => {
    if (active) onSelectRow(list[active.row]?.id ?? null);
  }, [active, list, onSelectRow]);

  React.useEffect(() => {
    if (!active) return;
    const el = gridRef.current?.querySelector<HTMLElement>(`[data-cell="${active.row}:${active.col}"]`);
    el?.scrollIntoView({ block: "nearest", inline: "nearest" });
  }, [active]);

  const canEdit = (row: GridRow) => editable && (row.kind === "insert" || !!row.key);

  const startEdit = (r: number, c: number, initial?: string) => {
    const row = list[r];
    if (!row || !canEdit(row)) return;
    if (row.kind === "row" && row.key && changes.deletes[rowKeyOf(row.key)]) return;
    const { value, unset } = cellValue(row, c, columns, changes);
    setEditing({ row: r, col: c, text: initial ?? (unset ? "" : textOf(value)) });
  };

  const commit = (r: number, c: number, text: string): boolean => {
    const row = list[r];
    const col = columns[c];
    if (!row || !col) return true;
    let value: Value;
    try {
      if (row.kind === "insert" && text === "") {
        setInsertCell(tableKey, row.insertId!, col.name, undefined);
        return true;
      }
      value = parseInput(col.kind, col.type, text);
    } catch (e) {
      toast.error(e instanceof ValueInputError ? `${col.name}: ${e.message}` : String(e));
      return false;
    }
    if (row.kind === "insert") setInsertCell(tableKey, row.insertId!, col.name, value);
    else if (row.key) setCell(tableKey, row.key, col.name, value, row.values[c] ?? { k: "null" });
    return true;
  };

  const move = (dr: number, dc: number) => {
    setActive((a) => {
      const base = a ?? { row: 0, col: 0 };
      return { row: Math.max(0, Math.min(list.length - 1, base.row + dr)), col: Math.max(0, Math.min(columns.length - 1, base.col + dc)) };
    });
  };

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (editing) return;
    if (!active && ["ArrowDown", "ArrowUp", "ArrowLeft", "ArrowRight"].includes(e.key)) {
      e.preventDefault();
      setActive({ row: 0, col: 0 });
      return;
    }
    if (!active) return;
    const row = list[active.row];
    switch (e.key) {
      case "ArrowDown":
        e.preventDefault();
        move(1, 0);
        return;
      case "ArrowUp":
        e.preventDefault();
        move(-1, 0);
        return;
      case "ArrowLeft":
        e.preventDefault();
        move(0, -1);
        return;
      case "ArrowRight":
      case "Tab":
        e.preventDefault();
        move(0, e.shiftKey ? -1 : 1);
        return;
      case "Enter":
      case "F2":
        e.preventDefault();
        startEdit(active.row, active.col);
        return;
      case "Escape":
        setActive(null);
        return;
      case "Backspace":
      case "Delete":
        if (!row) return;
        if (isCmd(e)) {
          e.preventDefault();
          if (row.kind === "insert") removeInsert(tableKey, row.insertId!);
          else if (row.key && editable) toggleDelete(tableKey, row.key);
          return;
        }
        e.preventDefault();
        startEdit(active.row, active.col, "");
        return;
    }
    if (isCmd(e) && e.key.toLowerCase() === "c" && row) {
      e.preventDefault();
      const { value } = cellValue(row, active.col, columns, changes);
      void navigator.clipboard.writeText(isNull(value) ? "" : textOf(value));
      return;
    }
    if (e.key.length === 1 && !e.metaKey && !e.ctrlKey && !e.altKey) {
      e.preventDefault();
      startEdit(active.row, active.col, e.key);
    }
  };

  return (
    <table ref={gridRef} className={cn("m-grid", refetching && "is-refetching", active && "is-focused")} tabIndex={0} onKeyDown={onKeyDown} aria-rowcount={list.length} role="grid">
      <thead>
        <tr>
          <th className="m-gutter">#</th>
          {columns.map((c) => {
            const m = metaCol.get(c.name);
            const sorted = sort?.column === c.name ? sort : null;
            return (
              <th
                key={c.name}
                className={cn(isNumeric(c.kind) && "is-num")}
                aria-sort={sorted ? (sorted.desc ? "descending" : "ascending") : "none"}
                style={{ cursor: "pointer" }}
                title={`${c.name} ${m?.type ?? c.type}${m && !m.nullable ? " not null" : ""}${m?.default ? ` default ${m.default}` : ""} · click to sort`}
                onClick={() => onSort(!sorted ? { column: c.name, desc: false } : !sorted.desc ? { column: c.name, desc: true } : null)}
              >
                <span className="m-th">
                  {pk.has(c.name) ? <KeyRound className="m-icon m-icon--sm" aria-label="Primary key" /> : fk.has(c.name) ? <Link2 className="m-icon m-icon--sm" aria-label="Foreign key" /> : null}
                  <span>{c.name}</span>
                  <span className="m-th__type">{m?.type ?? c.type}</span>
                  {sorted ? sorted.desc ? <ArrowDown className="m-icon m-icon--sm" aria-hidden="true" /> : <ArrowUp className="m-icon m-icon--sm" aria-hidden="true" /> : null}
                </span>
              </th>
            );
          })}
        </tr>
      </thead>
      <tbody>
        {list.map((row, r) => {
          const deleted = row.kind === "row" && !!row.key && !!changes.deletes[rowKeyOf(row.key)];
          const modified = row.kind === "row" && !!row.key && !!changes.updates[rowKeyOf(row.key)];
          const gutter = row.kind === "insert" ? "+" : deleted ? "−" : modified ? "~" : String(offset + r + 1);
          return (
            <tr key={row.id} className={cn(row.kind === "insert" && "is-added", deleted && "is-deleted", modified && !deleted && "is-modified", selectedRow === row.id && "is-selected")} aria-rowindex={r + 1}>
              <td className="m-gutter" onClick={() => setActive({ row: r, col: active?.col ?? 0 })}>
                {gutter}
              </td>
              {columns.map((c, ci) => {
                const isActive = active?.row === r && active.col === ci;
                const isEditing = editing?.row === r && editing.col === ci;
                const { value, dirty, unset } = cellValue(row, ci, columns, changes);
                const m = metaCol.get(c.name);
                return (
                  <td
                    key={c.name}
                    data-cell={`${r}:${ci}`}
                    className={cn(isNumeric(c.kind) && "is-num", dirty && row.kind === "row" && "is-dirty", isActive && "is-active", isEditing && "is-editing")}
                    onMouseDown={() => setActive({ row: r, col: ci })}
                    onDoubleClick={() => startEdit(r, ci)}
                    title={isNull(value) ? undefined : textOf(value).slice(0, 500)}
                  >
                    {isEditing ? (
                      <CellEditor
                        initial={editing.text}
                        label={c.name}
                        onCommit={(text, next) => {
                          if (!commit(r, ci, text)) return;
                          setEditing(null);
                          gridRef.current?.focus();
                          if (next === "down") move(1, 0);
                          else if (next === "right") move(0, 1);
                        }}
                        onCancel={() => {
                          setEditing(null);
                          gridRef.current?.focus();
                        }}
                      />
                    ) : unset ? (
                      <span className="m-null">{m?.default || m?.identity ? "DEFAULT" : NULL_INPUT}</span>
                    ) : isNull(value) ? (
                      <span className="m-null">{NULL_INPUT}</span>
                    ) : c.kind === "json" ? (
                      <span className="m-json">{textOf(value)}</span>
                    ) : (
                      textOf(value)
                    )}
                  </td>
                );
              })}
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

function CellEditor({ initial, label, onCommit, onCancel }: { initial: string; label: string; onCommit: (text: string, next?: "down" | "right") => void; onCancel: () => void }) {
  const [text, setText] = React.useState(initial);
  const ref = React.useRef<HTMLInputElement>(null);
  const done = React.useRef(false);
  React.useEffect(() => {
    const el = ref.current;
    if (!el) return;
    el.focus();
    el.setSelectionRange(el.value.length, el.value.length);
  }, []);
  return (
    <input
      ref={ref}
      className="m-cell-input"
      aria-label={label}
      value={text}
      onChange={(e) => setText(e.target.value)}
      onKeyDown={(e) => {
        e.stopPropagation();
        if (e.key === "Enter") {
          e.preventDefault();
          done.current = true;
          onCommit(text, "down");
        } else if (e.key === "Tab") {
          e.preventDefault();
          done.current = true;
          onCommit(text, "right");
        } else if (e.key === "Escape") {
          e.preventDefault();
          done.current = true;
          onCancel();
        }
      }}
      onBlur={() => {
        if (!done.current) onCommit(text);
      }}
    />
  );
}
