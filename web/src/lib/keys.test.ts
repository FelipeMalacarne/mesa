import { describe, expect, it } from "vitest";

import { keyLabel } from "./keys";

describe("keyLabel", () => {
  it("reads symbols on macOS and words elsewhere", () => {
    expect(keyLabel("mod+shift+t", true)).toBe("⌘⇧T");
    expect(keyLabel("mod+shift+t", false)).toBe("Ctrl+Shift+T");
    expect(keyLabel("mod+enter", true)).toBe("⌘↵");
    expect(keyLabel("ctrl+tab", false)).toBe("Ctrl+Tab");
    expect(keyLabel("j", true)).toBe("j");
    expect(keyLabel("G", false)).toBe("G");
  });
});
