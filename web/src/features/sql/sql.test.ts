import { describe, expect, it } from "vitest";

import { formatSql, isWrite, statementAt, statements, toCsv } from "./sql";

describe("statements", () => {
  it("splits on top-level semicolons only", () => {
    expect(statements("select ';'; -- a; b\nselect 2;\n$$ ; $$;")).toEqual(["select ';'", "-- a; b\nselect 2", "$$ ; $$"]);
  });
  it("drops empty and comment-only statements", () => {
    expect(statements(";; -- nothing\n")).toEqual([]);
  });
});

describe("isWrite", () => {
  it.each([
    ["update t set a = 1", true],
    ["  -- note\nDELETE FROM t", true],
    ["with x as (delete from t returning *) select * from x", true],
    ["create table t (a int)", true],
    ["select * from t", false],
    ["with x as (select 1) select * from x", false],
    ["explain select 1", false],
    ["show search_path", false],
    ["select * into archive from orders", true],
    ["select a from t where b = 'into x from'", false],
  ])("%s → %s", (sql, want) => {
    expect(isWrite(sql)).toBe(want);
  });
});

describe("statementAt", () => {
  it("returns the statement under the cursor", () => {
    const sql = "select 1;\nselect 2;\nselect 3;";
    expect(statementAt(sql, sql.indexOf("2"))).toBe("select 2");
    expect(statementAt(sql, 0)).toBe("select 1");
  });
});

describe("formatSql", () => {
  it("upper-cases keywords and breaks clauses", () => {
    expect(formatSql("select a, b from t where a = 1 order by b")).toBe("SELECT a, b\nFROM t\nWHERE a = 1\nORDER BY b;");
  });
  it("keeps strings untouched", () => {
    expect(formatSql("select 'from where' from t")).toBe("SELECT 'from where'\nFROM t;");
  });
});

describe("toCsv", () => {
  it("quotes when needed", () => {
    expect(toCsv(["a", "b"], [["1", 'x,"y"']])).toBe('a,b\n1,"x,""y"""');
  });
});
