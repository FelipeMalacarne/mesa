/**
 * What the diagram draws, as plain data: each table with its rows, and each relation with the
 * rows it joins. The edited model is drawn against the live schema, so the diagram is also the
 * migration: new tables, columns and relations are marked new, the ones the edit would drop
 * stay on the canvas as ghosts, and columns the server says would change are marked changed.
 */
import type { Column, Database, ForeignKey, SchemaChange, Table } from "@/lib/api/mesaAPI.schemas";

import { fkId, HEAD, NODE_WIDTH, ROW, tableKey, tablesOf, type Pos } from "./model";

export type State = "same" | "new" | "changed" | "dropped";

export interface DiagramRow {
  column: Column;
  state: State;
  pk: boolean;
  fk: boolean;
  unique: boolean;
}

export interface DiagramTable {
  key: string;
  schema: string;
  table: Table;
  state: State;
  rows: DiagramRow[];
  h: number;
}

export interface DiagramEdge {
  id: string;
  /** The referencing table (the "many" side) and the referenced one. */
  from: string;
  to: string;
  fromRow: number;
  toRow: number;
  state: Exclude<State, "changed">;
  fk: ForeignKey;
  /** The referencing columns are unique: one to one. */
  oneToOne: boolean;
  /** The referencing columns may be null: the parent is optional. */
  optional: boolean;
}

export interface Diagram {
  /** The database: its one schema on engines without schemas goes unprinted. */
  name: string;
  tables: DiagramTable[];
  edges: DiagramEdge[];
  byKey: Map<string, DiagramTable>;
}

export function cardHeight(rows: number): number {
  return HEAD + Math.max(1, rows) * ROW + 2;
}

export function edgeId(from: string, f: ForeignKey): string {
  return `${from}:${fkId(f)}`;
}

/** Columns and tables the server's diff would alter in place (type, null, default, keys…). */
export function changedBy(changes: SchemaChange[] | undefined) {
  const columns = new Set<string>();
  const tables = new Set<string>();
  for (const c of changes ?? []) {
    if (!c.table) continue;
    const t = tableKey(c.table.schema, c.table.name);
    switch (c.kind) {
      case "alter_column_type":
      case "set_default":
      case "set_column_comment":
        if (c.column) columns.add(`${t}.${c.column.name}`);
        tables.add(t);
        break;
      case "set_not_null":
      case "drop_not_null":
      case "drop_default":
        if (c.name) columns.add(`${t}.${c.name}`);
        tables.add(t);
        break;
      case "create_table":
      case "drop_table":
        break;
      default:
        tables.add(t);
    }
  }
  return { columns, tables };
}

function keyColumns(t: Table) {
  const pk = new Set(t.primaryKey?.columns ?? []);
  const fk = new Set((t.foreignKeys ?? []).flatMap((f) => f.columns));
  const unique = new Set([...(t.uniques ?? []).filter((u) => u.columns.length === 1).map((u) => u.columns[0]!), ...(t.indexes ?? []).filter((i) => i.unique && i.columns?.length === 1).map((i) => i.columns![0]!)]);
  return { pk, fk, unique };
}

function rowsOf(t: Table, state: (c: Column) => State): DiagramRow[] {
  const k = keyColumns(t);
  return t.columns.map((c) => ({ column: c, state: state(c), pk: k.pk.has(c.name), fk: k.fk.has(c.name), unique: k.unique.has(c.name) }));
}

/** The model drawn against the live schema; `changes` is the server's plan, when known. */
export function diagramOf(model: Database, live: Database | undefined, changes?: SchemaChange[]): Diagram {
  const liveTables = new Map(tablesOf(live).map((t) => [t.key, t]));
  const changed = changedBy(changes);
  const tables: DiagramTable[] = [];
  const modelKeys = new Set<string>();
  for (const ref of tablesOf(model)) {
    modelKeys.add(ref.key);
    const l = liveTables.get(ref.key)?.table;
    if (!l) {
      const rows = rowsOf(ref.table, () => "new");
      tables.push({ key: ref.key, schema: ref.schema, table: ref.table, state: "new", rows, h: cardHeight(rows.length) });
      continue;
    }
    const liveCols = new Set(l.columns.map((c) => c.name));
    const rows = rowsOf(ref.table, (c) => (!liveCols.has(c.name) ? "new" : changed.columns.has(`${ref.key}.${c.name}`) ? "changed" : "same"));
    // Columns the edit drops stay where they were, struck through.
    const kept = new Set(ref.table.columns.map((c) => c.name));
    const lk = keyColumns(l);
    l.columns.forEach((c, i) => {
      if (kept.has(c.name)) return;
      const prev = l.columns
        .slice(0, i)
        .reverse()
        .find((p) => kept.has(p.name) || rows.some((r) => r.column.name === p.name));
      const at = prev ? rows.findIndex((r) => r.column.name === prev.name) + 1 : 0;
      rows.splice(at, 0, { column: c, state: "dropped", pk: lk.pk.has(c.name), fk: lk.fk.has(c.name), unique: lk.unique.has(c.name) });
    });
    const touched = rows.some((r) => r.state !== "same") || changed.tables.has(ref.key) || fkIds(l).join() !== fkIds(ref.table).join();
    tables.push({ key: ref.key, schema: ref.schema, table: ref.table, state: touched ? "changed" : "same", rows, h: cardHeight(rows.length) });
  }
  for (const [key, ref] of liveTables) {
    if (modelKeys.has(key)) continue;
    const rows = rowsOf(ref.table, () => "dropped");
    tables.push({ key, schema: ref.schema, table: ref.table, state: "dropped", rows, h: cardHeight(rows.length) });
  }
  const byKey = new Map(tables.map((t) => [t.key, t]));

  const edges: DiagramEdge[] = [];
  const rowIndex = (key: string, column: string | undefined) => {
    const t = byKey.get(key);
    const i = t ? t.rows.findIndex((r) => r.column.name === column) : -1;
    return Math.max(0, i);
  };
  const push = (from: string, f: ForeignKey, state: DiagramEdge["state"], t: Table) => {
    const to = tableKey(f.references.schema, f.references.name);
    if (!byKey.has(to)) return;
    const k = keyColumns(t);
    const cols = f.columns.map((c) => t.columns.find((x) => x.name === c));
    edges.push({
      id: edgeId(from, f),
      from,
      to,
      fromRow: rowIndex(from, f.columns[0]),
      toRow: rowIndex(to, f.refColumns[0]),
      state,
      fk: f,
      oneToOne: (f.columns.length === 1 && (k.unique.has(f.columns[0]!) || (k.pk.has(f.columns[0]!) && k.pk.size === 1))) || (t.uniques ?? []).some((u) => sameSet(u.columns, f.columns)),
      optional: cols.some((c) => !c || c.nullable),
    });
  };
  for (const t of tables) {
    if (t.state === "dropped") {
      for (const f of t.table.foreignKeys ?? []) push(t.key, f, "dropped", t.table);
      continue;
    }
    const l = liveTables.get(t.key)?.table;
    const liveFks = new Set((l?.foreignKeys ?? []).map(fkId));
    for (const f of t.table.foreignKeys ?? []) push(t.key, f, liveFks.has(fkId(f)) ? "same" : "new", t.table);
    if (l) {
      const now = new Set((t.table.foreignKeys ?? []).map(fkId));
      for (const f of l.foreignKeys ?? []) if (!now.has(fkId(f))) push(t.key, f, "dropped", l);
    }
  }
  return { name: model.name, tables, edges, byKey };
}

function fkIds(t: Table): string[] {
  return (t.foreignKeys ?? []).map(fkId).sort();
}

function sameSet(a: string[], b: string[]): boolean {
  return a.length === b.length && a.every((x) => b.includes(x));
}

/** Tables one relation away from any of `keys`, `keys` included. */
export function neighbours(d: Diagram, keys: Iterable<string>): Set<string> {
  const out = new Set(keys);
  const start = new Set(out);
  for (const e of d.edges) {
    if (start.has(e.from)) out.add(e.to);
    if (start.has(e.to)) out.add(e.from);
  }
  return out;
}

/* ── geometry ─────────────────────────────────────────────────────────── */

export interface EdgeShape {
  /** The line itself. */
  d: string;
  /** Crow's-foot marks at both ends, drawn without dashes. */
  marks: string;
  /** Small circles ("zero or …") at the ends, filled with the canvas colour. */
  rings: Pos[];
  /** Where the line leaves the referencing table and meets the referenced one. */
  start: Pos;
  end: Pos;
}

/** The y of a row's middle on a card placed at `y`. */
export function rowY(y: number, row: number): number {
  return y + HEAD + row * ROW + ROW / 2 + 1;
}

/**
 * The line for a relation: from the side of the referencing card that faces the referenced
 * one, as a curve. Cards that overlap sideways are joined on their right sides by a loop.
 */
export function edgeShape(from: Pos, fromRow: number, to: Pos, toRow: number, self: boolean, e: Pick<DiagramEdge, "oneToOne" | "optional">): EdgeShape {
  const y1 = rowY(from.y, fromRow);
  const y2 = rowY(to.y, toRow);
  let x1: number;
  let x2: number;
  let d1: 1 | -1;
  let d2: 1 | -1;
  const gap = 24;
  if (!self && to.x >= from.x + NODE_WIDTH + gap) [x1, d1, x2, d2] = [from.x + NODE_WIDTH, 1, to.x, -1];
  else if (!self && to.x + NODE_WIDTH + gap <= from.x) [x1, d1, x2, d2] = [from.x, -1, to.x + NODE_WIDTH, 1];
  else {
    // One above the other (or a table referring to itself): a loop out of the right sides.
    [x1, x2, d1, d2] = [from.x + NODE_WIDTH, to.x + NODE_WIDTH, 1, 1];
    const out = Math.max(x1, x2) + 48 + Math.min(80, Math.abs(y2 - y1) / 6);
    const d = `M${x1},${y1} C${out},${y1} ${out},${y2} ${x2},${y2}`;
    return { d, ...ends(x1, y1, d1, x2, y2, d2, e), start: { x: x1, y: y1 }, end: { x: x2, y: y2 } };
  }
  const bend = Math.max(48, Math.abs(x2 - x1) / 2);
  const d = `M${x1},${y1} C${x1 + d1 * bend},${y1} ${x2 + d2 * bend},${y2} ${x2},${y2}`;
  return { d, ...ends(x1, y1, d1, x2, y2, d2, e), start: { x: x1, y: y1 }, end: { x: x2, y: y2 } };
}

/** Crow's foot at the referencing end, bars at the referenced end; a ring for "zero or". */
function ends(x1: number, y1: number, d1: number, x2: number, y2: number, d2: number, e: Pick<DiagramEdge, "oneToOne" | "optional">) {
  const rings: Pos[] = [];
  let marks = "";
  // Referencing end: many (crow's foot) or, one to one, a single bar. Always "zero or".
  if (e.oneToOne) marks += bar(x1 + d1 * 8, y1);
  else marks += `M${x1 + d1 * 12},${y1} L${x1},${y1 - 6} M${x1 + d1 * 12},${y1} L${x1},${y1 + 6}`;
  rings.push({ x: x1 + d1 * 18, y: y1 });
  // Referenced end: exactly one, or zero or one when the reference may be null.
  marks += bar(x2 + d2 * 7, y2);
  if (e.optional) rings.push({ x: x2 + d2 * 14, y: y2 });
  else marks += bar(x2 + d2 * 12, y2);
  return { marks, rings };
}

function bar(x: number, y: number): string {
  return ` M${x},${y - 6} L${x},${y + 6}`;
}
