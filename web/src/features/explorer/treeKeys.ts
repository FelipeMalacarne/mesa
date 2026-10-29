/**
 * Keyboard navigation for the object tree, following the WAI-ARIA tree pattern: ↑/↓ move,
 * → expands or steps into a node, ← collapses or steps out to the parent, Home/End jump, Enter
 * opens. Items are the rendered `[role=treeitem]` elements in document order (collapsed nodes
 * render no children). Inside an item, `[data-open]` opens it and `[data-toggle]` expands it,
 * for rows that hold more than one button (tables).
 */
import type * as React from "react";

const ITEM = '[role="treeitem"]';

function items(tree: HTMLElement): HTMLElement[] {
  return Array.from(tree.querySelectorAll<HTMLElement>(ITEM));
}

function focus(el: HTMLElement | undefined) {
  if (!el) return;
  el.focus();
  el.scrollIntoView?.({ block: "nearest" });
}

/** The first item, for moving into the tree from the filter. */
export function focusFirstItem(tree: HTMLElement | null) {
  if (tree) focus(items(tree)[0]);
}

/** Opens the first table or view the tree shows, for Enter in the filter. */
export function openFirstObject(tree: HTMLElement | null): boolean {
  const target = tree?.querySelector<HTMLElement>("[data-open]");
  target?.click();
  return !!target;
}

export function onTreeKeyDown(e: React.KeyboardEvent<HTMLElement>) {
  if (e.metaKey || e.ctrlKey || e.altKey) return;
  const tree = e.currentTarget;
  const current = (e.target as HTMLElement).closest<HTMLElement>(ITEM);
  if (!current || !tree.contains(current)) return;
  const list = items(tree);
  const i = list.indexOf(current);
  const expanded = current.getAttribute("aria-expanded");
  const toggle = () => (current.matches("button") ? current : current.querySelector<HTMLElement>("[data-toggle]"))?.click();

  switch (e.key) {
    case "ArrowDown":
      focus(list[i + 1]);
      break;
    case "ArrowUp":
      focus(list[i - 1]);
      break;
    case "Home":
      focus(list[0]);
      break;
    case "End":
      focus(list[list.length - 1]);
      break;
    case "ArrowRight":
      if (expanded === "false") toggle();
      else if (expanded === "true") focus(list[i + 1]);
      break;
    case "ArrowLeft": {
      if (expanded === "true") {
        toggle();
        break;
      }
      const parent = current.closest('[role="group"]')?.previousElementSibling;
      if (parent instanceof HTMLElement && parent.matches(ITEM)) focus(parent);
      break;
    }
    case "Enter":
      // Buttons open on Enter by themselves; rows with several buttons open their [data-open].
      if (current.matches("button")) return;
      current.querySelector<HTMLElement>("[data-open]")?.click();
      break;
    default:
      return;
  }
  e.preventDefault();
}
