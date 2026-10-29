import { describe, expect, it } from "vitest";

import { gridKey, newGridKeyState, type Cell, type GridAction } from "./gridKeys";

const size = { rows: 100, cols: 5, page: 20 };

function press(keys: string[], at: Cell | null, vim = true): GridAction | null {
  const state = newGridKeyState();
  let out: GridAction | null = null;
  for (const k of keys) {
    const ctrl = k.startsWith("C-");
    const key = ctrl ? k.slice(2) : k;
    out = gridKey({ key, ctrlKey: ctrl, metaKey: false, altKey: false, shiftKey: false }, at, size, vim, state);
  }
  return out;
}

const to = (row: number, col: number) => ({ kind: "move", cell: { row, col } });

describe("gridKey", () => {
  it("moves with the arrows whether or not vim keys are on", () => {
    expect(press(["ArrowDown"], { row: 3, col: 1 }, false)).toEqual(to(4, 1));
    expect(press(["End"], { row: 3, col: 1 }, false)).toEqual(to(3, 4));
    expect(press(["j"], { row: 3, col: 1 }, false)).toBeNull();
  });

  it("starts on the first cell when nothing is active", () => {
    expect(press(["j"], null)).toEqual(to(0, 0));
    expect(press(["G"], null)).toEqual(to(99, 0));
  });

  it("reads hjkl, words and line ends", () => {
    expect(press(["j"], { row: 3, col: 1 })).toEqual(to(4, 1));
    expect(press(["k"], { row: 0, col: 1 })).toEqual(to(0, 1));
    expect(press(["l"], { row: 3, col: 4 })).toEqual(to(3, 4));
    expect(press(["b"], { row: 3, col: 2 })).toEqual(to(3, 1));
    expect(press(["0"], { row: 3, col: 2 })).toEqual(to(3, 0));
    expect(press(["$"], { row: 3, col: 2 })).toEqual(to(3, 4));
  });

  it("takes counts, gg and G", () => {
    expect(press(["5", "j"], { row: 3, col: 1 })).toEqual(to(8, 1));
    expect(press(["1", "0", "j"], { row: 3, col: 1 })).toEqual(to(13, 1));
    expect(press(["g", "g"], { row: 50, col: 2 })).toEqual(to(0, 2));
    expect(press(["G"], { row: 50, col: 2 })).toEqual(to(99, 2));
    expect(press(["1", "2", "G"], { row: 50, col: 2 })).toEqual(to(11, 2));
    expect(press(["g"], { row: 50, col: 2 })).toEqual({ kind: "pending" });
  });

  it("moves half a page with Ctrl+D and Ctrl+U", () => {
    expect(press(["C-d"], { row: 0, col: 0 })).toEqual(to(10, 0));
    expect(press(["C-u"], { row: 15, col: 0 })).toEqual(to(5, 0));
  });

  it("reads the actions", () => {
    expect(press(["i"], { row: 1, col: 1 })).toEqual({ kind: "edit", replace: false });
    expect(press(["s"], { row: 1, col: 1 })).toEqual({ kind: "edit", replace: true });
    expect(press(["d", "d"], { row: 1, col: 1 })).toEqual({ kind: "delete-row" });
    expect(press(["d", "j"], { row: 1, col: 1 })).toEqual(to(2, 1));
    expect(press(["y"], { row: 1, col: 1 })).toEqual({ kind: "copy-cell" });
    expect(press(["Y"], { row: 1, col: 1 })).toEqual({ kind: "copy-row" });
    expect(press(["o"], { row: 1, col: 1 })).toEqual({ kind: "insert-row" });
    expect(press(["/"], { row: 1, col: 1 })).toEqual({ kind: "filter" });
  });

  it("leaves ⌘ and ⌥ keys to the page", () => {
    const state = newGridKeyState();
    expect(gridKey({ key: "c", metaKey: true, ctrlKey: false, altKey: false, shiftKey: false }, { row: 0, col: 0 }, size, true, state)).toBeNull();
  });
});
