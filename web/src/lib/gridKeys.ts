/**
 * Keys in a grid, read as a motion or an action. Arrows, Home/End, Ctrl+Home/End and
 * PageUp/PageDown always move. With vim keys on, so do h j k l, w b, 0 ^ $, gg G, Ctrl+D/U
 * (half a page), with a count in front (5j, 3w, 10G), and i a s c o dd y Y / are actions.
 * Multi-key commands keep their progress in `state`, which the grid holds in a ref.
 */
export interface Cell {
  row: number;
  col: number;
}

export interface GridSize {
  rows: number;
  cols: number;
  /** Rows that fit on screen, for page and half-page moves. */
  page: number;
}

export interface GridKeyState {
  count: string;
  pending: "" | "g" | "d";
}

export type GridAction =
  | { kind: "move"; cell: Cell }
  /** Edit the cell: `replace` starts from empty (s, c), else from its value (i, a). */
  | { kind: "edit"; replace: boolean }
  | { kind: "delete-row" }
  | { kind: "copy-cell" }
  | { kind: "copy-row" }
  | { kind: "insert-row" }
  | { kind: "filter" }
  /** Part of a longer command (a count, the first g of gg); nothing to do yet. */
  | { kind: "pending" };

type KeyLike = Pick<KeyboardEvent, "key" | "metaKey" | "ctrlKey" | "altKey" | "shiftKey">;

export function newGridKeyState(): GridKeyState {
  return { count: "", pending: "" };
}

export function gridKey(e: KeyLike, at: Cell | null, size: GridSize, vim: boolean, state: GridKeyState): GridAction | null {
  if (e.metaKey || e.altKey || size.rows === 0 || size.cols === 0) {
    reset(state);
    return null;
  }
  const here = at ?? { row: 0, col: 0 };
  const clamp = (row: number, col: number): GridAction => ({
    kind: "move",
    cell: { row: Math.max(0, Math.min(size.rows - 1, row)), col: Math.max(0, Math.min(size.cols - 1, col)) },
  });
  // The first move into a grid with no active cell lands on the first cell.
  const by = (dr: number, dc: number) => (at ? clamp(here.row + dr, here.col + dc) : clamp(0, 0));
  const half = Math.max(1, Math.floor(size.page / 2));

  if (e.ctrlKey) {
    const key = e.key.toLowerCase();
    const n = take(state);
    if (e.key === "Home") return clamp(0, here.col);
    if (e.key === "End") return clamp(size.rows - 1, here.col);
    if (vim && key === "d") return by(half * n, 0);
    if (vim && key === "u") return by(-half * n, 0);
    return null;
  }

  switch (e.key) {
    case "ArrowDown":
      return by(take(state), 0);
    case "ArrowUp":
      return by(-take(state), 0);
    case "ArrowRight":
      return by(0, take(state));
    case "ArrowLeft":
      return by(0, -take(state));
    case "Home":
      reset(state);
      return clamp(here.row, 0);
    case "End":
      reset(state);
      return clamp(here.row, size.cols - 1);
    case "PageDown":
      return by(size.page * take(state), 0);
    case "PageUp":
      return by(-size.page * take(state), 0);
  }
  if (!vim || e.key.length !== 1) {
    reset(state);
    return null;
  }

  const key = e.key;
  // A count: 1-9 start it, 0 continues it (alone, 0 is the first column).
  if ((key >= "1" && key <= "9") || (key === "0" && state.count)) {
    state.count += key;
    state.pending = "";
    return { kind: "pending" };
  }
  const pending = state.pending;
  state.pending = "";
  if (pending === "g" && key === "g") {
    const n = state.count ? Number(state.count) : 1;
    state.count = "";
    return clamp(n - 1, here.col);
  }
  if (pending === "d" && key === "d") {
    reset(state);
    return { kind: "delete-row" };
  }
  switch (key) {
    case "j":
      return by(take(state), 0);
    case "k":
      return by(-take(state), 0);
    case "l":
    case "w":
    case "e":
      return by(0, take(state));
    case "h":
    case "b":
      return by(0, -take(state));
    case "0":
    case "^":
      reset(state);
      return clamp(here.row, 0);
    case "$":
      reset(state);
      return clamp(here.row, size.cols - 1);
    case "G": {
      const row = state.count ? Number(state.count) - 1 : size.rows - 1;
      reset(state);
      return clamp(row, here.col);
    }
    case "g":
    case "d":
      state.pending = key;
      return { kind: "pending" };
  }
  reset(state);
  switch (key) {
    case "i":
    case "a":
      return { kind: "edit", replace: false };
    case "s":
    case "c":
      return { kind: "edit", replace: true };
    case "y":
      return { kind: "copy-cell" };
    case "Y":
      return { kind: "copy-row" };
    case "o":
    case "O":
      return { kind: "insert-row" };
    case "/":
      return { kind: "filter" };
  }
  return null;
}

function take(state: GridKeyState): number {
  const n = state.count ? Math.max(1, Number(state.count)) : 1;
  reset(state);
  return n;
}

function reset(state: GridKeyState) {
  state.count = "";
  state.pending = "";
}

/** Rows of a table that fit in its scrolling parent, for page moves. */
export function visibleRows(table: HTMLElement | null): number {
  const row = table?.querySelector<HTMLElement>("tbody tr");
  let box = table?.parentElement;
  while (box && box.scrollHeight <= box.clientHeight && box.parentElement) box = box.parentElement;
  if (!row || !box || row.offsetHeight === 0) return 20;
  return Math.max(1, Math.floor(box.clientHeight / row.offsetHeight) - 1);
}
