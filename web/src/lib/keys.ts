/**
 * Shortcut labels for the platform: `keyLabel("mod+shift+t")` reads ⌘⇧T on macOS and
 * Ctrl+Shift+T elsewhere. `mod` is ⌘ on macOS and Ctrl elsewhere, as in `isCmd`.
 */
import { isMac } from "./utils";

const MAC: Record<string, string> = { mod: "⌘", shift: "⇧", alt: "⌥", ctrl: "⌃", enter: "↵", up: "↑", down: "↓", left: "←", right: "→", tab: "⇥", esc: "Esc", space: "Space", backspace: "⌫" };
const OTHER: Record<string, string> = { mod: "Ctrl", shift: "Shift", alt: "Alt", ctrl: "Ctrl", enter: "Enter", up: "↑", down: "↓", left: "←", right: "→", tab: "Tab", esc: "Esc", space: "Space", backspace: "Backspace" };

/** A combo's label. A lone letter keeps its case ("j", "G" are vim keys); after modifiers it reads upper-case. */
export function keyLabel(combo: string, mac: boolean = isMac): string {
  const names = mac ? MAC : OTHER;
  const raw = combo === "+" ? ["+"] : combo.split("+");
  const parts = raw.map((p) => names[p] ?? (raw.length > 1 && p.length === 1 ? p.toUpperCase() : p));
  return parts.join(mac ? "" : "+");
}
