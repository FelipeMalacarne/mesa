import { describe, expect, it, vi } from "vitest";

import { focusFirstItem, onTreeKeyDown, openFirstObject } from "./treeKeys";

// db (expanded) > folder Tables (expanded) > table orders (row with toggle + open), column id
function tree() {
  const root = document.createElement("div");
  root.innerHTML = `
    <button role="treeitem" id="db" aria-expanded="true">shop</button>
    <div role="group">
      <button role="treeitem" id="folder" aria-expanded="true">Tables</button>
      <div role="group">
        <div role="treeitem" id="orders" tabindex="-1" aria-expanded="false"><button data-toggle id="toggle"></button><button data-open id="open">orders</button></div>
        <div role="treeitem" id="col" tabindex="-1">id</div>
      </div>
    </div>`;
  document.body.replaceChildren(root);
  return root;
}

function press(root: HTMLElement, key: string) {
  const ev = { key, currentTarget: root, target: document.activeElement, metaKey: false, ctrlKey: false, altKey: false, preventDefault: vi.fn() };
  onTreeKeyDown(ev as never);
  return ev;
}

const $ = (id: string) => document.getElementById(id)!;

describe("object tree keys", () => {
  it("moves, steps out to the parent and jumps to the ends", () => {
    const root = tree();
    focusFirstItem(root);
    expect(document.activeElement?.id).toBe("db");
    press(root, "ArrowDown");
    press(root, "ArrowDown");
    expect(document.activeElement?.id).toBe("orders");
    press(root, "ArrowLeft");
    expect(document.activeElement?.id).toBe("folder");
    press(root, "End");
    expect(document.activeElement?.id).toBe("col");
    press(root, "Home");
    expect(document.activeElement?.id).toBe("db");
  });

  it("expands with → and opens with Enter through the row's buttons", () => {
    const root = tree();
    const toggled = vi.fn();
    const opened = vi.fn();
    $("toggle").addEventListener("click", toggled);
    $("open").addEventListener("click", opened);
    $("orders").focus();
    press(root, "ArrowRight");
    expect(toggled).toHaveBeenCalledOnce();
    const ev = press(root, "Enter");
    expect(opened).toHaveBeenCalledOnce();
    expect(ev.preventDefault).toHaveBeenCalled();
    expect(openFirstObject(root)).toBe(true);
    expect(opened).toHaveBeenCalledTimes(2);
  });

  it("collapses an expanded node with ← before stepping out", () => {
    const root = tree();
    const collapsed = vi.fn();
    $("folder").addEventListener("click", collapsed);
    $("folder").focus();
    press(root, "ArrowLeft");
    expect(collapsed).toHaveBeenCalledOnce();
    expect(document.activeElement?.id).toBe("folder");
  });
});
