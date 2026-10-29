import { describe, expect, it } from "vitest";

import type { Database } from "@/lib/api/mesaAPI.schemas";

import { addColumn, addForeignKey, addKey, addTable, emptyModel, findTable, newness, relate, removeColumn, removeTable, restoreColumn, restoreTable, singular, togglePrimaryKey, updateColumn } from "./model";

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

  it("starts a new database empty", () => {
    expect(emptyModel("ledger")).toEqual({ name: "ledger", schemas: [{ name: "public", tables: [] }] });
  });

  it("renames a column into its keys and the relations that refer to it", () => {
    const m = updateColumn(live, "public.customers", "id", { name: "customer_id" });
    expect(findTable(m, "public.customers")!.table.primaryKey!.columns).toEqual(["customer_id"]);
    expect(findTable(m, "public.orders")!.table.foreignKeys![0]!.refColumns).toEqual(["customer_id"]);
  });

  it("makes primary key columns not null", () => {
    let m = addColumn(live, "public.customers", { name: "region", type: "text", nullable: true }, false);
    m = togglePrimaryKey(m, "public.customers", "region");
    const t = findTable(m, "public.customers")!.table;
    expect(t.primaryKey!.columns).toEqual(["id", "region"]);
    expect(t.columns.find((c) => c.name === "region")!.nullable).toBe(false);
  });

  it("puts a dropped column back where it was, with its relation", () => {
    const m = restoreColumn(removeColumn(live, "public.orders", "customer_id"), live, "public.orders", "customer_id");
    const t = findTable(m, "public.orders")!.table;
    expect(t.columns).toEqual(findTable(live, "public.orders")!.table.columns);
    expect(t.foreignKeys).toEqual(findTable(live, "public.orders")!.table.foreignKeys);
    expect(restoreTable(removeTable(live, "public.orders"), live, "public.orders").schemas[0]!.tables.map((t) => t.name)).toEqual(["customers", "orders"]);
  });

  describe("relations drawn on the canvas", () => {
    const shop = addTable(live, "public", { name: "invoices", columns: [{ name: "id", type: "bigint", nullable: false }, { name: "total", type: "numeric", nullable: false }], primaryKey: { columns: ["id"] } });

    it("refers to the key, whichever way it was drawn", () => {
      const r = relate(shop, { table: "public.customers", column: "id" }, { table: "public.orders", column: "customer_id" });
      expect(r.error).toMatch(/already/);
      const s = relate(shop, { table: "public.invoices", column: "id" }, { table: "public.orders", column: "customer_id" });
      expect(s.from).toBe("public.orders");
      expect(s.fk).toMatchObject({ columns: ["customer_id"], references: { name: "invoices" }, refColumns: ["id"] });
    });

    it("adds a column when a key is dropped on a table", () => {
      const r = relate(shop, { table: "public.customers", column: "id" }, { table: "public.invoices" });
      expect(r.addedColumn).toBe("customer_id");
      const t = findTable(r.model, "public.invoices")!.table;
      expect(t.columns.find((c) => c.name === "customer_id")).toMatchObject({ type: "bigint", nullable: true });
      expect(t.foreignKeys![0]).toMatchObject({ columns: ["customer_id"], refColumns: ["id"] });
    });

    it("refers to a table's primary key when a plain column is dropped on it, warning when types differ", () => {
      const r = relate(shop, { table: "public.invoices", column: "total" }, { table: "public.customers" });
      expect(r.fk).toMatchObject({ columns: ["total"], references: { name: "customers" }, refColumns: ["id"] });
      expect(r.warning).toMatch(/numeric but customers.id is bigint/);
    });

    it("refuses two plain columns", () => {
      expect(relate(shop, { table: "public.invoices", column: "total" }, { table: "public.orders", column: "customer_id" }).error).toMatch(/primary key or unique/);
    });
  });

  it("adds composite unique keys and named indexes", () => {
    let m = addKey(live, "public.orders", "unique", ["customer_id", "id"]);
    m = addKey(m, "public.orders", "index", ["customer_id"]);
    const t = findTable(m, "public.orders")!.table;
    expect(t.uniques).toEqual([{ name: undefined, columns: ["customer_id", "id"] }]);
    expect(t.indexes).toEqual([{ name: "orders_customer_id_idx", columns: ["customer_id"] }]);
  });

  it("makes singular names for referencing columns", () => {
    expect(["customers", "categories", "addresses", "status", "statuses", "boxes", "person"].map(singular)).toEqual(["customer", "category", "address", "status", "status", "box", "person"]);
  });
});
