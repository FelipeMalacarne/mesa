import { describe, expect, it } from "vitest";

import { plainTextFields } from "./plainInput";

const tick = () => new Promise((r) => setTimeout(r, 0));

describe("plainTextFields", () => {
  it("turns off capitalization, autocorrect and spell check on every field", async () => {
    const root = document.createElement("div");
    root.innerHTML = `<input id="a"><textarea id="b"></textarea>`;
    const stop = plainTextFields(root);

    const added = document.createElement("div");
    added.innerHTML = `<input id="c"><input id="d" autocapitalize="words"><textarea id="e" data-prose></textarea>`;
    root.appendChild(added);
    await tick();

    for (const id of ["a", "b", "c"]) {
      const el = root.querySelector(`#${id}`)!;
      expect(el.getAttribute("autocapitalize")).toBe("off");
      expect(el.getAttribute("autocorrect")).toBe("off");
      expect(el.getAttribute("spellcheck")).toBe("false");
    }
    expect(root.querySelector("#d")!.getAttribute("autocapitalize")).toBe("words");
    expect(root.querySelector("#e")!.hasAttribute("autocapitalize")).toBe(false);
    stop();
  });
});
