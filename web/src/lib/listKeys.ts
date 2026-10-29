/**
 * Keyboard for a list of rows (connections, the activity log): the list is one tab stop and
 * focus moves between its rows (elements with `data-row`) with j/k or ↓/↑, gg/G or Home/End,
 * Ctrl+D/U by half a screen; Enter or o clicks the row, l/h open and close a row that expands
 * (`aria-expanded`). These keys type nothing here, so they work with vim keys off too.
 */
import * as React from "react";

const ROW = "[data-row]";

function rowsOf(list: HTMLElement): HTMLElement[] {
  return Array.from(list.querySelectorAll<HTMLElement>(ROW));
}

function focusRow(el: HTMLElement | undefined) {
  if (!el) return;
  el.focus({ preventScroll: true });
  el.scrollIntoView({ block: "nearest" });
}

/** Moves focus into a list from outside it, e.g. ↓ in the list's search field. */
export function focusFirstRow(list: HTMLElement | null) {
  if (list) focusRow(rowsOf(list)[0]);
}

export function useListKeys<T extends HTMLElement>() {
  const lastG = React.useRef(0);

  const onKeyDown = (e: React.KeyboardEvent<T>) => {
    if (e.metaKey || e.altKey || (e.target instanceof HTMLElement && e.target.matches("input, textarea, select"))) return;
    const rows = rowsOf(e.currentTarget);
    const current = (e.target as HTMLElement).closest<HTMLElement>(ROW) ?? undefined;
    const at = current ? rows.indexOf(current) : -1;
    const half = 10;
    const key = e.key;
    let handled = true;
    if (key !== "g") lastG.current = 0;
    if (e.ctrlKey) {
      if (key === "d") focusRow(rows[Math.min(rows.length - 1, at + half)]);
      else if (key === "u") focusRow(rows[Math.max(0, at - half)]);
      else if (key === "n" || key === "j") focusRow(rows[Math.min(rows.length - 1, at + 1)]);
      else handled = false;
    } else if (key === "j" || key === "ArrowDown") focusRow(rows[Math.min(rows.length - 1, at + 1)]);
    else if (key === "k" || key === "ArrowUp") focusRow(rows[Math.max(0, at - 1)]);
    else if (key === "G" || key === "End") focusRow(rows[rows.length - 1]);
    else if (key === "Home") focusRow(rows[0]);
    else if (key === "g") {
      if (Date.now() - lastG.current < 600) {
        focusRow(rows[0]);
        lastG.current = 0;
      } else lastG.current = Date.now();
    } else if (!current) handled = false;
    else if (key === "Enter" || key === "o") current.click();
    else if (key === "l" || key === "ArrowRight") {
      if (current.getAttribute("aria-expanded") === "false") current.click();
    } else if (key === "h" || key === "ArrowLeft") {
      if (current.getAttribute("aria-expanded") === "true") current.click();
    } else handled = false;
    if (handled) e.preventDefault();
  };

  // Tab lands on the first row; the rows themselves are not tab stops.
  const onFocus = (e: React.FocusEvent<T>) => {
    if (e.target === e.currentTarget) focusRow(rowsOf(e.currentTarget)[0]);
  };

  return { tabIndex: 0, onKeyDown, onFocus };
}
