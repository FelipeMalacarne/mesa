import { beforeEach, describe, expect, it, vi } from "vitest";

import { useTabs, type Tab } from "@/features/explorer/tabs.store";

import { runCommand } from "./commands";
import { label, matches } from "./keys";
import { useUi } from "./ui.store";

const key = (code: string, mods: Partial<KeyboardEventInit> = {}) => new KeyboardEvent("keydown", { code, ...mods });

describe("bindings (Ctrl as ⌘ outside macOS)", () => {
  it("matches the exact modifiers", () => {
    expect(matches(key("KeyW", { ctrlKey: true }), { code: "KeyW", mod: true })).toBe(true);
    expect(
      matches(key("KeyW", { ctrlKey: true, shiftKey: true }), {
        code: "KeyW",
        mod: true,
      }),
    ).toBe(false);
    expect(matches(key("KeyW", { altKey: true }), { code: "KeyW", alt: true })).toBe(true);
    expect(matches(key("Tab", { ctrlKey: true }), { code: "Tab", ctrl: true })).toBe(true);
    expect(matches(key("KeyW"), { code: "KeyW", alt: true })).toBe(false);
  });
  it("prints shortcuts for the platform", () => {
    expect(label({ code: "KeyT", mod: true, shift: true })).toBe("Ctrl+Shift+T");
    expect(label({ code: "BracketRight", alt: true })).toBe("Alt+]");
  });
});

describe("tab commands", () => {
  const cid = "c1";
  const t = (n: number): Tab => ({
    id: `query:${n}`,
    kind: "query",
    title: `Query ${n}`,
  });
  let navigate: ReturnType<typeof vi.fn>;
  const ctx = () => ({ navigate: navigate as never, connectionId: cid });
  const active = () => useTabs.getState().byConnection[cid]?.activeId;
  // Commands repeated within 250 ms run once; tests move the clock past that.
  let now = 0;

  beforeEach(() => {
    now += 1000;
    vi.spyOn(performance, "now").mockImplementation(() => (now += 1000));
    useTabs.setState({ byConnection: {} });
    for (const n of [1, 2, 3]) useTabs.getState().open(cid, t(n));
    useTabs.getState().activate(cid, "query:2");
    navigate = vi.fn();
  });

  it("closes the active tab and shows its neighbour", () => {
    runCommand("tab.close", ctx());
    expect(useTabs.getState().byConnection[cid].tabs.map((x) => x.id)).toEqual(["query:1", "query:3"]);
    expect(active()).toBe("query:3");
    expect(navigate).toHaveBeenCalledWith(expect.objectContaining({ search: { tab: "query:3" } }));
  });

  it("reopens what was closed, newest first", () => {
    runCommand("tab.close", ctx());
    runCommand("tab.close", ctx());
    runCommand("tab.reopen", ctx());
    expect(active()).toBe("query:3");
    runCommand("tab.reopen", ctx());
    expect(active()).toBe("query:2");
    expect(useTabs.getState().byConnection[cid].tabs.map((x) => x.id)).toEqual(["query:1", "query:3", "query:2"]);
  });

  it("cycles and jumps between tabs", () => {
    runCommand("tab.next", ctx());
    expect(active()).toBe("query:3");
    runCommand("tab.next", ctx());
    expect(active()).toBe("query:1");
    runCommand("tab.prev", ctx());
    expect(active()).toBe("query:3");
    runCommand("tab.1", ctx());
    expect(active()).toBe("query:1");
    runCommand("tab.9", ctx());
    expect(active()).toBe("query:3");
    navigate.mockClear();
    runCommand("tab.5", ctx());
    expect(navigate).not.toHaveBeenCalled();
  });

  it("runs a command once when a menu and a key press both send it", () => {
    vi.spyOn(performance, "now").mockReturnValue(now + 50_000);
    runCommand("help", ctx());
    runCommand("help", ctx());
    expect(useUi.getState().helpOpen).toBe(true);
  });

  it("ignores tab commands outside a connection", () => {
    expect(runCommand("tab.close", { navigate: navigate as never })).toBe(false);
  });
});
