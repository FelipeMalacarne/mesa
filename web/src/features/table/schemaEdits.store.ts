/**
 * Staged structure edits per table: column changes, added and dropped columns and indexes.
 * `toSchemaChanges` turns them into the server's change kinds for review.
 */
import { create } from "zustand";

import type { Column, Index, Qualified, SchemaChange, Table } from "@/lib/api/mesaAPI.schemas";

export interface ColumnEdit {
  type?: string;
  nullable?: boolean;
  /** null drops the default. */
  default?: string | null;
  comment?: string;
}

export interface SchemaEdits {
  columns: Record<string, ColumnEdit>;
  added: Column[];
  dropped: string[];
  indexesAdded: Index[];
  indexesDropped: string[];
}

export const EMPTY_EDITS: SchemaEdits = { columns: {}, added: [], dropped: [], indexesAdded: [], indexesDropped: [] };

export function editCount(e: SchemaEdits | undefined): number {
  if (!e) return 0;
  return Object.keys(e.columns).length + e.added.length + e.dropped.length + e.indexesAdded.length + e.indexesDropped.length;
}

/** The structural changes, in an order the server can run top to bottom. */
export function toSchemaChanges(e: SchemaEdits, q: Qualified, table: Table): SchemaChange[] {
  const out: SchemaChange[] = [];
  for (const name of e.indexesDropped) out.push({ kind: "drop_index", table: q, name });
  for (const name of e.dropped) {
    const old = table.columns.find((c) => c.name === name);
    out.push({ kind: "drop_column", table: q, name, oldColumn: old });
  }
  for (const [name, edit] of Object.entries(e.columns)) {
    const old = table.columns.find((c) => c.name === name);
    if (!old || e.dropped.includes(name)) continue;
    if (edit.type !== undefined && edit.type !== old.type) out.push({ kind: "alter_column_type", table: q, column: { ...old, type: edit.type }, oldColumn: old });
    if (edit.nullable !== undefined && edit.nullable !== old.nullable) out.push({ kind: edit.nullable ? "drop_not_null" : "set_not_null", table: q, name });
    if (edit.default !== undefined && edit.default !== (old.default ?? null)) {
      if (edit.default === null || edit.default === "") out.push({ kind: "drop_default", table: q, name });
      else out.push({ kind: "set_default", table: q, column: { ...old, default: edit.default } });
    }
    if (edit.comment !== undefined && edit.comment !== (old.comment ?? "")) out.push({ kind: "set_column_comment", table: q, column: old, comment: edit.comment });
  }
  for (const c of e.added) out.push({ kind: "add_column", table: q, column: c });
  for (const ix of e.indexesAdded) out.push({ kind: "create_index", table: q, index: ix });
  return out;
}

interface State {
  tables: Record<string, SchemaEdits>;
  patch: (key: string, fn: (e: SchemaEdits) => SchemaEdits) => void;
  discard: (key: string) => void;
}

export const useSchemaEdits = create<State>((set, get) => ({
  tables: {},
  patch: (key, fn) => {
    const next = fn(get().tables[key] ?? EMPTY_EDITS);
    const tables = { ...get().tables };
    if (editCount(next) > 0) tables[key] = next;
    else delete tables[key];
    set({ tables });
  },
  discard: (key) => {
    const tables = { ...get().tables };
    delete tables[key];
    set({ tables });
  },
}));

export function useTableEdits(key: string): SchemaEdits {
  return useSchemaEdits((s) => s.tables[key] ?? EMPTY_EDITS);
}
