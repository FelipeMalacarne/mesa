import { describe, expect, it } from "vitest";

import { fuzzyMatch } from "./fuzzy";

const score = (q: string, t: string) => fuzzyMatch(q, t)?.score ?? -Infinity;

describe("fuzzyMatch", () => {
  it("needs every query character in order", () => {
    expect(fuzzyMatch("ord", "orders")).not.toBeNull();
    expect(fuzzyMatch("oit", "order_items")).not.toBeNull();
    expect(fuzzyMatch("dro", "orders")).toBeNull();
    expect(fuzzyMatch("ordersx", "orders")).toBeNull();
  });

  it("ranks exact, prefix, word start, substring, then subsequence", () => {
    const ranked = ["user", "users", "app_users", "superuser", "u_s_e_r_x"].sort((a, b) => score("user", b) - score("user", a));
    expect(ranked).toEqual(["user", "users", "app_users", "superuser", "u_s_e_r_x"]);
  });

  it("prefers the shorter name within a tier", () => {
    expect(score("ord", "orders")).toBeGreaterThan(score("ord", "orders_archive"));
  });

  it("highlights the characters it matched", () => {
    expect(fuzzyMatch("item", "order_items")?.indices).toEqual([6, 7, 8, 9]);
    expect(fuzzyMatch("oi", "order_items")?.indices).toEqual([0, 6]);
    expect(fuzzyMatch("ui", "userId")?.indices).toEqual([0, 4]);
  });

  it("takes a word start only when the rest of the query still fits", () => {
    expect(fuzzyMatch("ab", "xaxb_a")?.indices).toEqual([1, 3]);
  });

  it("ignores case", () => {
    expect(fuzzyMatch("ORD", "orders")?.indices).toEqual([0, 1, 2]);
  });
});
