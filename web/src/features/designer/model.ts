/**
 * Pure helpers over the schema model for the designer: keys, edits made from the UI (the DBML
 * is re-printed from the edited model) and what is new compared with the live database.
 * Layouts live in layout.ts.
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

function renameIn(list: string[] | undefined, from: string, to: string): string[] | undefined {
  return list?.map((c) => (c === from ? to : c));
}

/** Changes a column; a new name follows into keys, indexes and relations. */
export function updateColumn(db: Database, key: string, name: string, patch: Partial<Column>): Database {
  const next = clone(db);
  const ref = findTable(next, key);
  if (!ref) return next;
  const t = ref.table;
  t.columns = t.columns.map((c) => (c.name === name ? { ...c, ...patch } : c));
  for (const c of t.columns) if (c.default === undefined) delete c.default;
  const to = patch.name;
  if (to && to !== name) {
    if (t.primaryKey) t.primaryKey = { ...t.primaryKey, columns: renameIn(t.primaryKey.columns, name, to)! };
    t.uniques = t.uniques?.map((u) => ({ ...u, columns: renameIn(u.columns, name, to)! }));
    t.indexes = t.indexes?.map((i) => ({ ...i, columns: renameIn(i.columns, name, to) }));
    t.foreignKeys = t.foreignKeys?.map((f) => ({ ...f, columns: renameIn(f.columns, name, to)! }));
    for (const o of tablesOf(next))
      o.table.foreignKeys = o.table.foreignKeys?.map((f) => (tableKey(f.references.schema, f.references.name) === key ? { ...f, refColumns: renameIn(f.refColumns, name, to)! } : f));
  }
  return next;
}

/** Puts a column in or out of the primary key; key columns cannot be null. */
export function togglePrimaryKey(db: Database, key: string, column: string): Database {
  const next = clone(db);
  const ref = findTable(next, key);
  if (!ref) return next;
  const t = ref.table;
  const cols = t.primaryKey?.columns ?? [];
  const on = !cols.includes(column);
  const nextCols = on ? [...cols, column] : cols.filter((c) => c !== column);
  t.primaryKey = nextCols.length ? { name: t.primaryKey?.name, columns: nextCols } : undefined;
  if (on) t.columns = t.columns.map((c) => (c.name === column ? { ...c, nullable: false } : c));
  return next;
}

export type KeyKind = "unique" | "index";

/** Adds a unique constraint or an index over the columns, in order. */
export function addKey(db: Database, key: string, kind: KeyKind, columns: string[], name?: string): Database {
  const next = clone(db);
  const ref = findTable(next, key);
  if (!ref || columns.length === 0) return next;
  const t = ref.table;
  if (kind === "unique") t.uniques = [...(t.uniques ?? []), { name: name || undefined, columns }];
  else t.indexes = [...(t.indexes ?? []), { name: name || indexName(t.name, columns, "idx"), columns }];
  return next;
}

/** Removes the i-th unique constraint or index. */
export function removeKey(db: Database, key: string, kind: KeyKind, i: number): Database {
  const next = clone(db);
  const ref = findTable(next, key);
  if (!ref) return next;
  if (kind === "unique") ref.table.uniques = (ref.table.uniques ?? []).filter((_, j) => j !== i);
  else ref.table.indexes = (ref.table.indexes ?? []).filter((_, j) => j !== i);
  return next;
}

/** Whether a single column is unique on its own (constraint or unique index). */
export function isUnique(t: Table, column: string): boolean {
  return (t.uniques ?? []).some((u) => u.columns.length === 1 && u.columns[0] === column) || (t.indexes ?? []).some((i) => i.unique && i.columns?.length === 1 && i.columns[0] === column);
}

export function indexName(table: string, columns: string[], suffix: string): string {
  return `${table}_${columns.join("_")}_${suffix}`.slice(0, 63);
}

export function removeForeignKey(db: Database, key: string, id: string): Database {
  const next = clone(db);
  const ref = findTable(next, key);
  if (ref) ref.table.foreignKeys = (ref.table.foreignKeys ?? []).filter((f) => fkId(f) !== id);
  return next;
}

export function updateForeignKey(db: Database, key: string, id: string, patch: Partial<ForeignKey>): Database {
  const next = clone(db);
  const ref = findTable(next, key);
  if (ref) ref.table.foreignKeys = (ref.table.foreignKeys ?? []).map((f) => (fkId(f) === id ? { ...f, ...patch } : f));
  return next;
}

/** Renames a table; relations to it follow. Only for tables not in the database yet. */
export function renameTable(db: Database, key: string, name: string): Database {
  const next = clone(db);
  const ref = findTable(next, key);
  if (!ref) return next;
  ref.table.name = name;
  for (const o of tablesOf(next))
    o.table.foreignKeys = o.table.foreignKeys?.map((f) => (tableKey(f.references.schema, f.references.name) === key ? { ...f, references: { ...f.references, name } } : f));
  return next;
}

export function setTableComment(db: Database, key: string, comment: string): Database {
  const next = clone(db);
  const ref = findTable(next, key);
  if (ref) ref.table.comment = comment || undefined;
  return next;
}

/** Puts a table the edit dropped back as it is in the database. */
export function restoreTable(db: Database, live: Database, key: string): Database {
  const l = findTable(live, key);
  if (!l || findTable(db, key)) return db;
  return addTable(db, l.schema, JSON.parse(JSON.stringify(l.table)) as Table);
}

/** Puts a dropped column back where it was, with the keys and relations it had. */
export function restoreColumn(db: Database, live: Database, key: string, column: string): Database {
  const l = findTable(live, key)?.table;
  const next = clone(db);
  const t = findTable(next, key)?.table;
  const c = l?.columns.find((x) => x.name === column);
  if (!l || !t || !c || t.columns.some((x) => x.name === column)) return next;
  const i = l.columns.findIndex((x) => x.name === column);
  const before = l.columns.slice(0, i).map((x) => x.name);
  const at = t.columns.reduce((pos, x, j) => (before.includes(x.name) ? j + 1 : pos), 0);
  t.columns = [...t.columns.slice(0, at), { ...c }, ...t.columns.slice(at)];
  const present = new Set(t.columns.map((x) => x.name));
  const fits = (cols: string[] | undefined) => !!cols?.includes(column) && cols.every((x) => present.has(x));
  if (l.primaryKey && fits(l.primaryKey.columns)) t.primaryKey = l.primaryKey;
  for (const u of l.uniques ?? []) if (fits(u.columns) && !(t.uniques ?? []).some((x) => x.columns.join() === u.columns.join())) t.uniques = [...(t.uniques ?? []), u];
  for (const x of l.indexes ?? []) if (fits(x.columns) && !(t.indexes ?? []).some((y) => y.name === x.name)) t.indexes = [...(t.indexes ?? []), x];
  for (const f of l.foreignKeys ?? []) if (fits(f.columns) && !(t.foreignKeys ?? []).some((y) => fkId(y) === fkId(f))) t.foreignKeys = [...(t.foreignKeys ?? []), f];
  return next;
}

/** "customers" → "customer", "categories" → "category". Good enough for column names. */
export function singular(name: string): string {
  if (/ies$/i.test(name)) return name.slice(0, -3) + "y";
  if (/(x|ch|sh|ss|us)es$/i.test(name)) return name.slice(0, -2);
  if (/(ss|us|is)$/i.test(name)) return name;
  if (/s$/i.test(name)) return name.slice(0, -1);
  return name;
}

/** A referencing column takes the referenced type, without what makes it generate values. */
function referencingType(type: string): string {
  const t = type.trim().toLowerCase();
  if (t === "serial" || t === "serial4") return "integer";
  if (t === "bigserial" || t === "serial8") return "bigint";
  if (t === "smallserial" || t === "serial2") return "smallint";
  return type;
}

function keyed(t: Table, column: string): boolean {
  return (t.primaryKey?.columns.length === 1 && t.primaryKey.columns[0] === column) || isUnique(t, column);
}

export interface RelateResult {
  model: Database;
  /** The table that got the foreign key, and the key. */
  from?: string;
  fk?: ForeignKey;
  /** A column made for the relation (dragging a key onto a table). */
  addedColumn?: string;
  error?: string;
  /** The two columns' types differ; the database may refuse the key. */
  warning?: string;
}

/**
 * A relation drawn on the canvas, from a column to a column or to a whole table. The side that
 * is a primary key or unique column is the one referenced; drop a key on a table and that table
 * gets a `<table>_<column>` column for it (reusing one with that name).
 */
export function relate(db: Database, a: { table: string; column: string }, b: { table: string; column?: string }): RelateResult {
  const ta = findTable(db, a.table);
  const tb = findTable(db, b.table);
  if (!ta || !tb) return { model: db, error: "That table is not in the model." };
  let fromRef = ta;
  let fromCol: string;
  let toRef = tb;
  let toCol: string;
  let addedColumn: string | undefined;
  let model = db;
  if (b.column) {
    if (keyed(tb.table, b.column)) [fromCol, toCol] = [a.column, b.column];
    else if (keyed(ta.table, a.column)) [fromRef, fromCol, toRef, toCol] = [tb, b.column, ta, a.column];
    else return { model: db, error: `A relation must refer to a primary key or unique column; neither ${a.column} nor ${b.column} is one.` };
  } else if (keyed(ta.table, a.column)) {
    // A key dropped on a table: that table refers to it.
    [fromRef, toRef, toCol] = [tb, ta, a.column];
    const name = `${singular(ta.table.name)}_${a.column}`;
    const existing = tb.table.columns.find((c) => c.name === name);
    fromCol = name;
    if (!existing) {
      const ref = ta.table.columns.find((c) => c.name === a.column)!;
      model = addColumn(model, tb.key, { name, type: referencingType(ref.type), nullable: true }, false);
      addedColumn = name;
    }
  } else {
    const pk = tb.table.primaryKey?.columns;
    if (pk?.length !== 1) return { model: db, error: `${tb.table.name} has no single-column primary key to refer to; drop onto a unique column instead.` };
    [fromCol, toCol] = [a.column, pk[0]!];
  }
  if (fromRef.key === toRef.key && fromCol === toCol) return { model: db, error: "A column cannot refer to itself." };
  const fk: ForeignKey = { columns: [fromCol], references: { schema: toRef.schema, name: toRef.table.name }, refColumns: [toCol] };
  const fromTable = findTable(model, fromRef.key)!.table;
  if ((fromTable.foreignKeys ?? []).some((f) => fkId(f) === fkId(fk))) return { model: db, error: "That relation is already there." };
  const a1 = fromTable.columns.find((c) => c.name === fromCol)?.type ?? "";
  const b1 = toRef.table.columns.find((c) => c.name === toCol)?.type ?? "";
  const warning = referencingType(a1).toLowerCase() !== referencingType(b1).toLowerCase() ? `${fromRef.table.name}.${fromCol} is ${a1} but ${toRef.table.name}.${toCol} is ${b1}; the database may refuse the key.` : undefined;
  return { model: addForeignKey(model, fromRef.key, fk), from: fromRef.key, fk, addedColumn, warning };
}
