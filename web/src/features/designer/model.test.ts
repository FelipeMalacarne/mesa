import { describe, expect, it } from "vitest";

import type { Database } from "@/lib/api/mesaAPI.schemas";

import { addColumn, addForeignKey, addTable, autoLayout, emptyModel, newness, removeColumn, removeTable } from "./model";

const live: Database = {
  name: "shop",
  schemas: [
    {
      name: "public",
      tables: [
        { name: "customers", columns: [{ name: "id", type: "bigint", nullable: false }], primaryKey: { columns: ["id"] } },
        {
          name: "orders",
          columns: [
            { name: "id", type: "bigint", nullable: false },
            { name: "customer_id", type: "bigint", nullable: false },
          ],
          primaryKey: { columns: ["id"] },
          foreignKeys: [{ columns: ["customer_id"], references: { schema: "public", name: "customers" }, refColumns: ["id"] }],
        },
      ],
    },
  ],
};

describe("designer model", () => {
  it("adds and removes tables without touching the input", () => {
    const withTable = addTable(live, "public", { name: "coupons", columns: [{ name: "code", type: "text", nullable: false }] });
    expect(withTable.schemas[0]!.tables.map((t) => t.name)).toContain("coupons");
    expect(live.schemas[0]!.tables).toHaveLength(2);
    const without = removeTable(withTable, "public.customers");
    expect(without.schemas[0]!.tables.find((t) => t.name === "orders")!.foreignKeys).toEqual([]);
  });

  it("tracks what is new against the live model", () => {
    let m = addColumn(live, "public.customers", { name: "email", type: "text", nullable: false }, false);
    m = addTable(m, "public", { name: "coupons", columns: [{ name: "id", type: "bigint", nullable: false }] });
    m = addForeignKey(m, "public.coupons", { columns: ["id"], references: { schema: "public", name: "customers" }, refColumns: ["id"] });
    const n = newness(live, m);
    expect([...n.tables]).toEqual(["public.coupons"]);
    expect([...n.columns]).toEqual(["public.customers.email"]);
    expect(n.edges.size).toBe(1);
  });

  it("drops keys and relations with a removed column", () => {
    const m = removeColumn(live, "public.orders", "customer_id");
    expect(m.schemas[0]!.tables[1]!.foreignKeys).toEqual([]);
  });

  it("lays referenced tables left of the tables that reference them", () => {
    const pos = autoLayout(live);
    expect(pos["public.customers"]!.x).toBeLessThan(pos["public.orders"]!.x);
  });

  it("starts a new database empty", () => {
    expect(emptyModel("ledger")).toEqual({ name: "ledger", schemas: [{ name: "public", tables: [] }] });
  });
});
