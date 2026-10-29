import { describe, expect, it } from "vitest";

import { matchShortcut } from "./commands";

function key(k: string, mods: { meta?: boolean; ctrl?: boolean; shift?: boolean; alt?: boolean; code?: string } = {}) {
  return { key: k, code: mods.code ?? "", metaKey: !!mods.meta, ctrlKey: !!mods.ctrl, shiftKey: !!mods.shift, altKey: !!mods.alt };
}

describe("matchShortcut", () => {
  it("reads ⌘ on macOS and Ctrl elsewhere", () => {
    expect(matchShortcut(key("w", { meta: true }), true)).toBe("tab.close");
    expect(matchShortcut(key("w", { ctrl: true }), false)).toBe("tab.close");
    expect(matchShortcut(key("w", { ctrl: true }), true)).toBeNull();
    expect(matchShortcut(key("w"), true)).toBeNull();
  });

  it("knows the tab keys", () => {
    expect(matchShortcut(key("T", { meta: true, shift: true }), true)).toBe("tab.reopen");
    expect(matchShortcut(key("}", { meta: true, shift: true }), true)).toBe("tab.next");
    expect(matchShortcut(key("{", { meta: true, shift: true }), true)).toBe("tab.prev");
    expect(matchShortcut(key("Tab", { ctrl: true }), true)).toBe("tab.next");
    expect(matchShortcut(key("Tab", { ctrl: true, shift: true }), false)).toBe("tab.prev");
    expect(matchShortcut(key("3", { meta: true }), true)).toBe("tab.3");
    expect(matchShortcut(key("9", { meta: true }), true)).toBe("tab.last");
    expect(matchShortcut(key("˜", { meta: true, alt: true, code: "KeyN" }), true)).toBe("tab.new");
  });

  it("leaves zoom keys to the browser or the desktop menu", () => {
    expect(matchShortcut(key("=", { meta: true }), true)).toBeNull();
    expect(matchShortcut(key("-", { meta: true }), true)).toBeNull();
    expect(matchShortcut(key("0", { meta: true }), true)).toBeNull();
  });

  it("opens settings and the shortcut list", () => {
    expect(matchShortcut(key(",", { meta: true }), true)).toBe("settings");
    expect(matchShortcut(key("/", { ctrl: true }), false)).toBe("shortcuts");
  });
});
