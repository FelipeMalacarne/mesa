import { describe, expect, it } from "vitest";

import type { ColumnType, Database } from "@/lib/api/mesaAPI.schemas";

import { guessColumn, guessReference } from "./columns";

const pg: ColumnType[] = [
  { name: "bigint", group: "Numeric", spelling: "bigint", integer: true },
  { name: "integer", group: "Numeric", spelling: "integer", integer: true },
  { name: "numeric", group: "Numeric", args: "precision", spelling: "numeric(12,2)" },
  { name: "text", group: "Text", spelling: "text" },
  { name: "varchar", group: "Text", args: "length", spelling: "varchar(255)" },
  { name: "boolean", group: "Boolean", spelling: "boolean" },
  { name: "timestamptz", group: "Date & time", spelling: "timestamptz" },
  { name: "date", group: "Date & time", spelling: "date" },
  { name: "uuid", group: "Identifiers", spelling: "uuid" },
  { name: "jsonb", group: "JSON", spelling: "jsonb" },
];
const mysql: ColumnType[] = [
  { name: "bigint unsigned", group: "Numeric", spelling: "bigint unsigned", integer: true },
  { name: "int", group: "Numeric", spelling: "int", integer: true },
  { name: "decimal", group: "Numeric", args: "precision", spelling: "decimal(12,2)" },
  { name: "varchar", group: "Text", args: "length", spelling: "varchar(255)" },
  { name: "char", group: "Text", args: "length", spelling: "char(36)" },
  { name: "text", group: "Text", spelling: "text" },
  { name: "tinyint(1)", group: "Boolean", spelling: "tinyint(1)" },
  { name: "timestamp", group: "Date & time", spelling: "timestamp" },
  { name: "json", group: "JSON", spelling: "json" },
];

describe("column guesses", () => {
  it("reads the usual names, in each engine's types", () => {
    expect(guessColumn("created_at", pg)).toEqual({ type: "timestamptz", default: "CURRENT_TIMESTAMP", notNull: true });
    expect(guessColumn("created_at", mysql).type).toBe("timestamp");
    expect(guessColumn("is_active", pg)).toEqual({ type: "boolean", default: "false", notNull: true });
    expect(guessColumn("is_active", mysql).type).toBe("tinyint(1)");
    expect(guessColumn("customer_id", mysql).type).toBe("bigint unsigned");
    expect(guessColumn("total_price", pg).type).toBe("numeric(12,2)");
    expect(guessColumn("price", mysql).type).toBe("decimal(12,2)");
    expect(guessColumn("email", pg).type).toBe("varchar(255)");
    expect(guessColumn("phone", pg).type).toBe("varchar(32)");
    expect(guessColumn("public_uuid", mysql).type).toBe("char(36)");
    expect(guessColumn("metadata", pg).type).toBe("jsonb");
    expect(guessColumn("birth_date", pg).type).toBe("date");
    expect(guessColumn("description", mysql).type).toBe("text");
    expect(guessColumn("zzz", pg)).toEqual({});
  });

  it("finds the table an _id column refers to", () => {
    const db: Database = {
      name: "shop",
      schemas: [
        {
          name: "public",
          tables: [
            { name: "customers", columns: [{ name: "id", type: "bigint", nullable: false }], primaryKey: { columns: ["id"] } },
            { name: "categories", columns: [{ name: "code", type: "text", nullable: false }], primaryKey: { columns: ["code"] } },
            { name: "logs", columns: [{ name: "line", type: "text", nullable: true }] },
          ],
        },
      ],
    };
    expect(guessReference("customer_id", db, "public")).toMatchObject({ key: "public.customers", column: "id" });
    expect(guessReference("category_id", db, "public")).toMatchObject({ key: "public.categories", column: "code" });
    expect(guessReference("log_id", db, "public")).toBeUndefined();
    expect(guessReference("customer_id", db, "public", "customers")).toBeUndefined();
    expect(guessReference("name", db, "public")).toBeUndefined();
  });
});
