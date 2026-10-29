import { describe, expect, it } from "vitest";

import type { Database } from "@/lib/api/mesaAPI.schemas";

import { diagramOf, edgeShape, neighbours } from "./graph";
import { addColumn, addForeignKey, removeColumn, removeTable } from "./model";

const live: Database = {
  name: "shop",
  schemas: [
    {
      name: "public",
      tables: [
        { name: "customers", columns: [{ name: "id", type: "bigint", nullable: false }, { name: "email", type: "text", nullable: false }], primaryKey: { columns: ["id"] } },
        {
          name: "orders",
          columns: [
            { name: "id", type: "bigint", nullable: false },
            { name: "customer_id", type: "bigint", nullable: true },
            { name: "note", type: "text", nullable: true },
          ],
          primaryKey: { columns: ["id"] },
          foreignKeys: [{ columns: ["customer_id"], references: { schema: "public", name: "customers" }, refColumns: ["id"] }],
        },
        { name: "logs", columns: [{ name: "line", type: "text", nullable: true }] },
      ],
    },
  ],
};

describe("designer diagram", () => {
  it("draws the live schema unchanged", () => {
    const d = diagramOf(live, live);
    expect(d.tables.map((t) => t.state)).toEqual(["same", "same", "same"]);
    expect(d.edges).toHaveLength(1);
    expect(d.edges[0]).toMatchObject({ from: "public.orders", to: "public.customers", fromRow: 1, toRow: 0, state: "same", optional: true, oneToOne: false });
  });

  it("keeps dropped tables, columns and relations as ghosts in place", () => {
    let m = removeColumn(live, "public.orders", "customer_id");
    m = removeTable(m, "public.logs");
    const d = diagramOf(m, live);
    const orders = d.byKey.get("public.orders")!;
    expect(orders.state).toBe("changed");
    expect(orders.rows.map((r) => `${r.column.name}:${r.state}`)).toEqual(["id:same", "customer_id:dropped", "note:same"]);
    expect(d.byKey.get("public.logs")!.state).toBe("dropped");
    expect(d.edges.map((e) => e.state)).toEqual(["dropped"]);
    expect(d.edges[0]!.fromRow).toBe(1);
  });

  it("marks new columns and relations, and the columns the server would change", () => {
    let m = addColumn(live, "public.customers", { name: "org_id", type: "bigint", nullable: false }, false);
    m = addForeignKey(m, "public.customers", { columns: ["org_id"], references: { schema: "public", name: "orders" }, refColumns: ["id"] });
    const d = diagramOf(m, live, [{ kind: "set_not_null", table: { schema: "public", name: "customers" }, name: "email" }]);
    const c = d.byKey.get("public.customers")!;
    expect(c.rows.map((r) => r.state)).toEqual(["same", "changed", "new"]);
    expect(d.edges.find((e) => e.from === "public.customers")!.state).toBe("new");
  });

  it("finds the tables one relation away", () => {
    const d = diagramOf(live, live);
    expect([...neighbours(d, ["public.customers"])].sort()).toEqual(["public.customers", "public.orders"]);
  });

  it("joins facing sides, and loops cards stacked above each other", () => {
    const e = { oneToOne: false, optional: false };
    const right = edgeShape({ x: 0, y: 0 }, 0, { x: 600, y: 0 }, 0, false, e);
    expect(right.start.x).toBe(240);
    expect(right.end.x).toBe(600);
    const left = edgeShape({ x: 600, y: 0 }, 0, { x: 0, y: 0 }, 0, false, e);
    expect(left.start.x).toBe(600);
    expect(left.end.x).toBe(240);
    const stacked = edgeShape({ x: 0, y: 400 }, 0, { x: 40, y: 0 }, 0, false, e);
    expect(stacked.start.x).toBe(240);
    expect(stacked.end.x).toBe(280);
  });
});
