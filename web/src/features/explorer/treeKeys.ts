/**
 * Keyboard navigation for the object tree, following the WAI-ARIA tree pattern and vim:
 * ↑/↓ or j/k move, → or l expands or steps into a node, ← or h collapses or steps out to the
 * parent, Home/End or gg/G jump to the ends, Ctrl+D/U move ten items, Enter or o opens and
 * moves into the view, Space opens and stays, / goes to the search. These keys type nothing in
 * the tree, so they work with vim keys off too.
 *
 * Items are the rendered `[role=treeitem]` elements in document order (collapsed nodes render
 * no children). An item that opens something is, or holds, `[data-open]`; a row with several
 * buttons (tables) expands through its `[data-toggle]`.
 */
import type * as React from "react";

import { isMac } from "@/lib/utils";

import { focusMain } from "../workspace/focus";

const ITEM = '[role="treeitem"]';

function items(tree: HTMLElement): HTMLElement[] {
  return Array.from(tree.querySelectorAll<HTMLElement>(ITEM));
}

function focus(el: HTMLElement | undefined) {
  if (!el) return;
  el.focus({ preventScroll: true });
  el.scrollIntoView?.({ block: "nearest" });
}

function opener(item: HTMLElement): HTMLElement | null {
  return item.matches("[data-open]") ? item : item.querySelector<HTMLElement>("[data-open]");
}

/** The first item, for moving into the tree from the search. */
export function focusFirstItem(tree: HTMLElement | null) {
  if (tree) focus(items(tree)[0]);
}

/** Opens the first table or view the tree shows. */
export function openFirstObject(tree: HTMLElement | null): boolean {
  const target = tree?.querySelector<HTMLElement>("[data-open]");
  target?.click();
  return !!target;
}

export interface TreeKeyOptions {
  /** `/`: go to the search. */
  onSearch?: () => void;
}

// gg is two presses within a moment.
let lastG = 0;

export function onTreeKeyDown(e: React.KeyboardEvent<HTMLElement>, opts: TreeKeyOptions = {}) {
  if (e.metaKey || e.altKey) return;
  // Ctrl+P and Ctrl+K are ⌘P and ⌘K outside macOS: leave those to the app there.
  if (e.ctrlKey && (!["n", "p", "j", "k", "d", "u"].includes(e.key) || (!isMac && (e.key === "p" || e.key === "k")))) return;
  const tree = e.currentTarget;
  const current = (e.target as HTMLElement).closest<HTMLElement>(ITEM);
  if (!current || !tree.contains(current)) return;
  const list = items(tree);
  const i = list.indexOf(current);
  const expanded = current.getAttribute("aria-expanded");
  const toggle = () => (current.matches("button") ? current : current.querySelector<HTMLElement>("[data-toggle]"))?.click();
  const key = e.ctrlKey ? `C-${e.key}` : e.key;
  if (key !== "g") lastG = 0;

  switch (key) {
    case "ArrowDown":
    case "j":
    case "C-n":
    case "C-j":
      focus(list[i + 1]);
      break;
    case "ArrowUp":
    case "k":
    case "C-p":
    case "C-k":
      focus(list[i - 1]);
      break;
    case "C-d":
      focus(list[Math.min(list.length - 1, i + 10)]);
      break;
    case "C-u":
      focus(list[Math.max(0, i - 10)]);
      break;
    case "Home":
      focus(list[0]);
      break;
    case "End":
    case "G":
      focus(list[list.length - 1]);
      break;
    case "g":
      if (Date.now() - lastG < 600) {
        focus(list[0]);
        lastG = 0;
      } else lastG = Date.now();
      break;
    case "ArrowRight":
    case "l":
      if (expanded === "false") toggle();
      else if (expanded === "true") focus(list[i + 1]);
      break;
    case "ArrowLeft":
    case "h": {
      if (expanded === "true") {
        toggle();
        break;
      }
      const parent = current.closest('[role="group"]')?.previousElementSibling;
      if (parent instanceof HTMLElement && parent.matches(ITEM)) focus(parent);
      break;
    }
    case "Enter":
    case "o":
    case " ": {
      // Enter opens the object and moves into its view, like picking a file in a fuzzy finder;
      // Space opens it and stays. A node that opens nothing (a schema, a folder) toggles.
      const open = opener(current);
      if (open) {
        open.click();
        if (key !== " ") focusMain();
      } else if (current.matches("button")) current.click();
      break;
    }
    case "/":
      if (!opts.onSearch) return;
      opts.onSearch();
      break;
    default:
      return;
  }
  e.preventDefault();
}
