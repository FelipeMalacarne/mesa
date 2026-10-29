import { describe, expect, it } from "vitest";

import type { Database, Table } from "@/lib/api/mesaAPI.schemas";

import { bounds, graphOf, layout, placeMissing, stackNear, type LayoutKind } from "./layout";
import { NODE_WIDTH, nodeHeight, tablesOf } from "./model";

const fk = (column: string, table: string) => ({ columns: [column], references: { schema: "public", name: table }, refColumns: ["id"] });
const table = (name: string, refs: string[] = [], extra = 2): Table => ({
  name,
  columns: [{ name: "id", type: "bigint", nullable: false }, ...refs.map((r) => ({ name: `${r}_id`, type: "bigint", nullable: true })), ...Array.from({ length: extra }, (_, i) => ({ name: `c${i}`, type: "text", nullable: true }))],
  primaryKey: { columns: ["id"] },
  foreignKeys: refs.map((r) => fk(`${r}_id`, r)),
});

/** A hub most tables refer to, a few chains, and tables with no relation: a typical app. */
function messy(): Database {
  const tables: Table[] = [table("organizations"), table("users"), table("members", ["organizations", "users"])];
  for (let i = 0; i < 60; i++) tables.push(table(`t${String(i).padStart(2, "0")}`, i % 7 === 0 ? ["organizations", "members"] : ["organizations"], (i * 5) % 9));
  tables.push(table("documents", ["organizations"]), table("versions", ["documents"]), table("signers", ["versions", "users"]));
  tables.push(table("cache", [], 3), table("jobs", [], 1), table("migrations", [], 2));
  tables.push(table("ledgers"), table("entries", ["ledgers"]));
  return { name: "app", schemas: [{ name: "public", tables }] };
}

function rects(db: Database, pos: Record<string, { x: number; y: number }>) {
  return tablesOf(db).map((t) => ({ key: t.key, x: pos[t.key]!.x, y: pos[t.key]!.y, w: NODE_WIDTH, h: nodeHeight(t.table) }));
}

function overlapping(db: Database, pos: Record<string, { x: number; y: number }>): string[] {
  const rs = rects(db, pos);
  const out: string[] = [];
  for (let i = 0; i < rs.length; i++)
    for (let j = i + 1; j < rs.length; j++) {
      const a = rs[i]!;
      const b = rs[j]!;
      if (a.x < b.x + b.w && b.x < a.x + a.w && a.y < b.y + b.h && b.y < a.y + a.h) out.push(`${a.key} × ${b.key}`);
    }
  return out;
}

describe("designer layouts", () => {
  const kinds: LayoutKind[] = ["hierarchy", "clusters", "grid"];

  for (const kind of kinds) {
    it(`${kind}: places every table, with no overlaps, the same way each time`, () => {
      const db = messy();
      const pos = layout(db, kind);
      expect(Object.keys(pos).sort()).toEqual(tablesOf(db).map((t) => t.key).sort());
      expect(overlapping(db, pos)).toEqual([]);
      expect(layout(db, kind)).toEqual(pos);
      for (const p of Object.values(pos)) expect(p.x % 8 === 0 && p.y % 8 === 0 && p.x >= 0 && p.y >= 0).toBe(true);
    });

    it(`${kind}: comes out wide rather than one tall column`, () => {
      const db = messy();
      const b = bounds(rects(db, layout(db, kind)))!;
      expect(b.w / b.h).toBeGreaterThan(0.6);
      expect(b.w / b.h).toBeLessThan(4);
    });
  }

  it("hierarchy: referenced tables sit left of the tables that reference them", () => {
    const pos = layout(messy(), "hierarchy");
    expect(pos["public.documents"]!.x).toBeLessThan(pos["public.versions"]!.x);
    expect(pos["public.versions"]!.x).toBeLessThan(pos["public.signers"]!.x);
    expect(pos["public.ledgers"]!.x).toBeLessThan(pos["public.entries"]!.x);
  });

  it("hierarchy: lines a lone child up with its parent", () => {
    const db: Database = { name: "x", schemas: [{ name: "public", tables: [table("a", [], 8), table("b", ["a"], 0)] }] };
    const pos = layout(db, "hierarchy");
    // b.a_id (row 1) faces a.id (row 0): b sits one row higher than a.
    expect(pos["public.b"]!.y).toBe(pos["public.a"]!.y - 24);
  });

  it("clusters: keeps a chain closer together than unrelated tables", () => {
    const db = messy();
    const pos = layout(db, "clusters");
    const d = (a: string, b: string) => Math.hypot(pos[`public.${a}`]!.x - pos[`public.${b}`]!.x, pos[`public.${a}`]!.y - pos[`public.${b}`]!.y);
    expect(d("versions", "signers")).toBeLessThan(d("versions", "t30"));
  });

  it("grid: reads alphabetically down the columns", () => {
    const db = messy();
    const pos = layout(db, "grid");
    const order = tablesOf(db)
      .map((t) => t.key)
      .sort((a, b) => pos[a]!.x - pos[b]!.x || pos[a]!.y - pos[b]!.y);
    expect(order).toEqual([...order].sort());
  });

  it("leaves self references and unknown tables out of the graph", () => {
    const db: Database = { name: "x", schemas: [{ name: "public", tables: [{ ...table("a", ["a"]) }, table("b", ["zzz"])] }] };
    expect(graphOf(db).links).toEqual([]);
  });

  it("stacks items in order as near their wanted spot as gaps allow", () => {
    expect(stackNear([0, 0], [100, 100])).toEqual([-66, 66]);
    expect(stackNear([0, 500], [100, 100])).toEqual([0, 500]);
  });

  it("places a new table beside the table it references, clear of the others", () => {
    const db = messy();
    const pos = layout(db, "hierarchy");
    const next: Database = { ...db, schemas: [{ ...db.schemas[0]!, tables: [...db.schemas[0]!.tables, table("comments", ["documents"])] }] };
    const placed = placeMissing(next, pos);
    expect(overlapping(next, placed)).toEqual([]);
    const c = placed["public.comments"]!;
    const d = placed["public.documents"]!;
    expect(Math.hypot(c.x - d.x, c.y - d.y)).toBeLessThan(800);
    expect(placed["public.documents"]).toEqual(pos["public.documents"]);
  });

  it("places an unrelated new table near the given point", () => {
    const db = messy();
    const pos = layout(db, "hierarchy");
    const next: Database = { ...db, schemas: [{ ...db.schemas[0]!, tables: [...db.schemas[0]!.tables, table("coupons")] }] };
    const placed = placeMissing(next, pos, { x: 5000, y: 5000 });
    expect(Math.abs(placed["public.coupons"]!.x + NODE_WIDTH / 2 - 5000)).toBeLessThan(100);
  });
});
