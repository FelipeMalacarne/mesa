/**
 * Pure helpers over the schema model for the designer: keys, edits made from the UI (the DBML
 * is re-printed from the edited model), what is new compared with the live database, and a
 * dependency-ordered auto layout.
 */
import type { Column, Database, ForeignKey, Table } from "@/lib/api/mesaAPI.schemas";

export interface Pos {
  x: number;
  y: number;
}

export const NODE_WIDTH = 240;
export const HEAD = 32;
export const ROW = 24;

export function tableKey(schema: string, table: string): string {
  return `${schema}.${table}`;
}

export function nodeHeight(t: Table): number {
  return HEAD + Math.max(1, t.columns.length) * ROW + 2;
}

export interface TableRef {
  schema: string;
  table: Table;
  key: string;
}

export function tablesOf(db: Database | undefined): TableRef[] {
  const out: TableRef[] = [];
  for (const s of db?.schemas ?? []) for (const t of s.tables) out.push({ schema: s.name, table: t, key: tableKey(s.name, t.name) });
  return out;
}

export function findTable(db: Database, key: string): TableRef | undefined {
  return tablesOf(db).find((t) => t.key === key);
}

function clone(db: Database): Database {
  return JSON.parse(JSON.stringify(db)) as Database;
}

function schemaOf(db: Database, name: string) {
  let s = db.schemas.find((x) => x.name === name);
  if (!s) {
    s = { name, tables: [] };
    db.schemas.push(s);
  }
  return s;
}

export function emptyModel(name: string): Database {
  return { name, schemas: [{ name: "public", tables: [] }] };
}

export function addTable(db: Database, schema: string, table: Table): Database {
  const next = clone(db);
  const s = schemaOf(next, schema);
  s.tables = [...s.tables.filter((t) => t.name !== table.name), table];
  return next;
}

export function removeTable(db: Database, key: string): Database {
  const next = clone(db);
  for (const s of next.schemas) {
    s.tables = s.tables.filter((t) => tableKey(s.name, t.name) !== key);
    for (const t of s.tables) t.foreignKeys = (t.foreignKeys ?? []).filter((f) => tableKey(f.references.schema, f.references.name) !== key);
  }
  return next;
}

export function addColumn(db: Database, key: string, column: Column, primaryKey: boolean): Database {
  const next = clone(db);
  const ref = findTable(next, key);
  if (!ref) return next;
  ref.table.columns = [...ref.table.columns, column];
  if (primaryKey) ref.table.primaryKey = { name: ref.table.primaryKey?.name, columns: [...(ref.table.primaryKey?.columns ?? []), column.name] };
  return next;
}

export function removeColumn(db: Database, key: string, column: string): Database {
  const next = clone(db);
  const ref = findTable(next, key);
  if (!ref) return next;
  const t = ref.table;
  t.columns = t.columns.filter((c) => c.name !== column);
  if (t.primaryKey) {
    const cols = t.primaryKey.columns.filter((c) => c !== column);
    t.primaryKey = cols.length ? { ...t.primaryKey, columns: cols } : undefined;
  }
  t.uniques = (t.uniques ?? []).filter((u) => !u.columns.includes(column));
  t.indexes = (t.indexes ?? []).filter((i) => !(i.columns ?? []).includes(column));
  t.foreignKeys = (t.foreignKeys ?? []).filter((f) => !f.columns.includes(column));
  return next;
}

export function addForeignKey(db: Database, key: string, fk: ForeignKey): Database {
  const next = clone(db);
  const ref = findTable(next, key);
  if (!ref) return next;
  ref.table.foreignKeys = [...(ref.table.foreignKeys ?? []), fk];
  return next;
}

/** Tables, columns and relations present in `model` but not in `live`. */
export function newness(live: Database | undefined, model: Database | undefined) {
  const tables = new Set<string>();
  const columns = new Set<string>();
  const edges = new Set<string>();
  const liveTables = new Map(tablesOf(live).map((t) => [t.key, t.table]));
  for (const t of tablesOf(model)) {
    const l = liveTables.get(t.key);
    if (!l) {
      tables.add(t.key);
      continue;
    }
    const cols = new Set(l.columns.map((c) => c.name));
    for (const c of t.table.columns) if (!cols.has(c.name)) columns.add(`${t.key}.${c.name}`);
    const fks = new Set((l.foreignKeys ?? []).map(fkId));
    for (const f of t.table.foreignKeys ?? []) if (!fks.has(fkId(f))) edges.add(`${t.key}:${fkId(f)}`);
  }
  for (const t of tablesOf(model)) if (tables.has(t.key)) for (const f of t.table.foreignKeys ?? []) edges.add(`${t.key}:${fkId(f)}`);
  return { tables, columns, edges };
}

export function fkId(f: ForeignKey): string {
  return `${f.columns.join(",")}>${f.references.schema}.${f.references.name}(${f.refColumns.join(",")})`;
}

/** Places tables left to right by how deep their references go, stacking each column. */
export function autoLayout(db: Database, keep: Record<string, Pos> = {}): Record<string, Pos> {
  const refs = tablesOf(db);
  const byKey = new Map(refs.map((r) => [r.key, r]));
  const depth = new Map<string, number>();
  const visit = (key: string, seen: Set<string>): number => {
    if (depth.has(key)) return depth.get(key)!;
    if (seen.has(key)) return 0;
    seen.add(key);
    const r = byKey.get(key);
    let d = 0;
    for (const f of r?.table.foreignKeys ?? []) {
      const target = tableKey(f.references.schema, f.references.name);
      if (target !== key && byKey.has(target)) d = Math.max(d, visit(target, seen) + 1);
    }
    depth.set(key, d);
    return d;
  };
  for (const r of refs) visit(r.key, new Set());
  const columns = new Map<number, TableRef[]>();
  for (const r of refs) {
    const d = depth.get(r.key) ?? 0;
    columns.set(d, [...(columns.get(d) ?? []), r]);
  }
  const out: Record<string, Pos> = { ...keep };
  for (const [d, list] of [...columns.entries()].sort((a, b) => a[0] - b[0])) {
    let y = 40;
    for (const r of list.sort((a, b) => a.table.name.localeCompare(b.table.name))) {
      if (!keep[r.key]) out[r.key] = { x: 40 + d * (NODE_WIDTH + 100), y };
      y += nodeHeight(r.table) + 40;
    }
  }
  return out;
}

/** Positions for tables that have none yet, placed below or beside the others. */
export function placeMissing(db: Database, positions: Record<string, Pos>): Record<string, Pos> {
  const missing = tablesOf(db).filter((t) => !positions[t.key]);
  if (missing.length === 0) return positions;
  if (Object.keys(positions).length === 0) return autoLayout(db);
  // New tables go in a row under everything already placed.
  const out = { ...positions };
  let bottom = 0;
  const heights = new Map(tablesOf(db).map((t) => [t.key, nodeHeight(t.table)]));
  for (const [key, p] of Object.entries(positions)) bottom = Math.max(bottom, p.y + (heights.get(key) ?? 0));
  let x = 40;
  for (const m of missing) {
    out[m.key] = { x, y: bottom + 60 };
    x += NODE_WIDTH + 60;
  }
  return out;
}
