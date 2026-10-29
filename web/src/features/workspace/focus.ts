/**
 * Moving keyboard focus into the open view after the rail opened something (Enter in the search
 * or the tree), the way picking a file in a fuzzy finder lands in its buffer. Views mark the
 * element that should take it (the grid, the SQL editor) with `data-main-focus`.
 */
const FRAMES = 90;

export function focusMain(): void {
  const before = document.querySelector<HTMLElement>("main [data-main-focus]");
  const from = document.activeElement;
  let frames = 0;
  const step = () => {
    frames++;
    const main = document.querySelector<HTMLElement>("main");
    // Focus placed in the view meanwhile (by a click, or the view itself) stays where it is.
    if (!main || (document.activeElement !== from && main.contains(document.activeElement))) return;
    const target = main.querySelector<HTMLElement>("[data-main-focus]");
    // A new view replaces the old element; the same view keeps it, so take it after a moment.
    if (target && (target !== before || frames > 10)) {
      target.focus();
      return;
    }
    if (frames < FRAMES) requestAnimationFrame(step);
    else main.focus();
  };
  requestAnimationFrame(step);
}

/**
 * Steps keyboard focus through the open view's panes (`data-pane`: the SQL editor and its
 * results, the data grid), for Ctrl+j / Ctrl+k with vim keys on. From outside the view, any
 * step lands on its main pane. False when the view has nowhere to go.
 */
export function focusPane(step: -1 | 0 | 1): boolean {
  const main = document.querySelector<HTMLElement>("main");
  if (!main) return false;
  const panes = Array.from(main.querySelectorAll<HTMLElement>("[data-pane]")).filter((p) => p.offsetParent !== null);
  const at = panes.findIndex((p) => p.contains(document.activeElement));
  const next = at < 0 ? (panes.find((p) => p.hasAttribute("data-main-focus")) ?? panes[0]) : panes[Math.max(0, Math.min(panes.length - 1, at + step))];
  if (next) next.focus();
  else main.focus();
  return true;
}
