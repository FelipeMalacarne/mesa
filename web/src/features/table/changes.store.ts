/**
 * Staged row edits, keyed by table (`connectionId/database/schema.table`).
 *
 * A cell edit, a new row or a delete mark lives here until the person commits it through
 * the review dialog or discards it. `toChangeSet` builds the ChangeSet exactly as the API
 * schema describes it: update = primary-key values + changed cells, insert = typed cells,
 * delete = primary-key values.
 */
import { create } from "zustand";

import type { ChangeSet, Qualified, RowChange } from "@/lib/api/mesaAPI.schemas";
import { isNull, valuesEqual, type Value } from "@/lib/values";
import { uid } from "@/lib/utils";

export type RowValues = Record<string, Value>;

export interface StagedUpdate {
  key: RowValues;
  values: RowValues;
  original: RowValues;
}

export interface StagedInsert {
  id: string;
  values: RowValues;
}

export interface StagedDelete {
  key: RowValues;
}

export interface TableChanges {
  updates: Record<string, StagedUpdate>;
  inserts: StagedInsert[];
  deletes: Record<string, StagedDelete>;
}

export interface ChangeCounts {
  edited: number;
  added: number;
  deleted: number;
  total: number;
}

export const EMPTY_CHANGES: TableChanges = { updates: {}, inserts: [], deletes: {} };

export function tableKey(connectionId: string, database: string, schema: string, table: string): string {
  return `${connectionId}/${database}/${schema}.${table}`;
}

/** A stable identity for a row from its primary-key values. */
export function rowKeyOf(key: RowValues): string {
  return JSON.stringify(
    Object.keys(key)
      .sort()
      .map((k) => [k, isNull(key[k]) ? null : key[k]!.v ?? null]),
  );
}

export function countsOf(tc: TableChanges | undefined): ChangeCounts {
  if (!tc) return { edited: 0, added: 0, deleted: 0, total: 0 };
  const edited = Object.keys(tc.updates).length;
  // A new row with no values yet is a placeholder, not a change.
  const added = tc.inserts.filter((i) => Object.keys(i.values).length > 0).length;
  const deleted = Object.keys(tc.deletes).length;
  return { edited, added, deleted, total: edited + added + deleted };
}

export function hasChanges(tc: TableChanges | undefined): boolean {
  return countsOf(tc).total > 0;
}

/** True while anything is staged, empty new rows included (the store keeps those). */
function hasAnything(tc: TableChanges): boolean {
  return tc.inserts.length > 0 || Object.keys(tc.updates).length > 0 || Object.keys(tc.deletes).length > 0;
}

/** "1 edited · 1 new · 1 deleted" (only the non-zero parts). */
export function summaryOf(counts: ChangeCounts): string {
  const parts: string[] = [];
  if (counts.edited) parts.push(`${counts.edited} edited`);
  if (counts.added) parts.push(`${counts.added} new`);
  if (counts.deleted) parts.push(`${counts.deleted} deleted`);
  return parts.join(" · ");
}

/** Kinds whose equality is well defined in SQL, so an old value can join the WHERE clause. */
const COMPARABLE = new Set(["null", "bool", "int", "decimal", "text", "uuid", "time", "bytes"]);

/**
 * Builds the ChangeSet for one table. Rows marked for deletion drop their pending edits.
 * An update's key carries the primary key plus the old values of the cells it changes (when
 * they compare safely): the server then refuses the update if someone changed the row since it
 * was read, and can write the statement that restores it.
 */
export function toChangeSet(tc: TableChanges, database: string, table: Qualified): ChangeSet {
  const rows: RowChange[] = [];
  for (const [rowKey, u] of Object.entries(tc.updates)) {
    if (tc.deletes[rowKey]) continue;
    if (Object.keys(u.values).length === 0) continue;
    const key: RowValues = { ...u.key };
    for (const [col, old] of Object.entries(u.original)) {
      if (!(col in key) && COMPARABLE.has(old.k)) key[col] = old;
    }
    rows.push({ kind: "update", table, key, values: u.values });
  }
  for (const d of Object.values(tc.deletes)) {
    rows.push({ kind: "delete", table, key: d.key });
  }
  for (const i of tc.inserts) {
    if (Object.keys(i.values).length === 0) continue;
    rows.push({ kind: "insert", table, values: i.values });
  }
  return { database, rows };
}

interface ChangesState {
  tables: Record<string, TableChanges>;
  /** Stages one cell. A value equal to the original clears that cell's edit. */
  setCell: (tableKey: string, key: RowValues, column: string, value: Value, original: Value) => void;
  stageInsert: (tableKey: string) => string;
  setInsertCell: (tableKey: string, insertId: string, column: string, value: Value | undefined) => void;
  removeInsert: (tableKey: string, insertId: string) => void;
  toggleDelete: (tableKey: string, key: RowValues) => void;
  discard: (tableKey: string) => void;
}

export const useChanges = create<ChangesState>((set, get) => {
  const patch = (tableKey: string, fn: (tc: TableChanges) => TableChanges) => {
    const current = get().tables[tableKey] ?? EMPTY_CHANGES;
    const next = fn(current);
    const tables = { ...get().tables };
    if (hasAnything(next)) tables[tableKey] = next;
    else delete tables[tableKey];
    set({ tables });
  };

  return {
    tables: {},
    setCell: (tableKey, key, column, value, original) =>
      patch(tableKey, (tc) => {
        const rk = rowKeyOf(key);
        const u: StagedUpdate = tc.updates[rk] ?? { key, values: {}, original: {} };
        const values = { ...u.values };
        const originals = { ...u.original };
        if (valuesEqual(value, original)) {
          delete values[column];
          delete originals[column];
        } else {
          values[column] = value;
          originals[column] = original;
        }
        const updates = { ...tc.updates };
        if (Object.keys(values).length === 0) delete updates[rk];
        else updates[rk] = { key, values, original: originals };
        return { ...tc, updates };
      }),
    stageInsert: (tableKey) => {
      const id = uid("row");
      patch(tableKey, (tc) => ({ ...tc, inserts: [...tc.inserts, { id, values: {} }] }));
      return id;
    },
    setInsertCell: (tableKey, insertId, column, value) =>
      patch(tableKey, (tc) => ({
        ...tc,
        inserts: tc.inserts.map((i) => {
          if (i.id !== insertId) return i;
          const values = { ...i.values };
          if (value === undefined) delete values[column];
          else values[column] = value;
          return { ...i, values };
        }),
      })),
    removeInsert: (tableKey, insertId) =>
      patch(tableKey, (tc) => ({ ...tc, inserts: tc.inserts.filter((i) => i.id !== insertId) })),
    toggleDelete: (tableKey, key) =>
      patch(tableKey, (tc) => {
        const rk = rowKeyOf(key);
        const deletes = { ...tc.deletes };
        if (deletes[rk]) delete deletes[rk];
        else deletes[rk] = { key };
        return { ...tc, deletes };
      }),
    discard: (tableKey) => patch(tableKey, () => EMPTY_CHANGES),
  };
});

export function useTableChanges(tableKey: string | null): TableChanges {
  return useChanges((s) => (tableKey ? s.tables[tableKey] ?? EMPTY_CHANGES : EMPTY_CHANGES));
}
