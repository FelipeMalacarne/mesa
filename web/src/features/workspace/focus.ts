/**
 * Moving keyboard focus into the open view after the rail opened something (Enter in the search
 * or the tree), the way picking a file in a fuzzy finder lands in its buffer. Views mark the
 * element that should take it (the grid, the SQL editor) with `data-main-focus`.
 */
const FRAMES = 90;

export function focusMain(): void {
  const before = document.querySelector<HTMLElement>("main [data-main-focus]");
  let frames = 0;
  const step = () => {
    frames++;
    const main = document.querySelector<HTMLElement>("main");
    if (!main || main.contains(document.activeElement)) return;
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
