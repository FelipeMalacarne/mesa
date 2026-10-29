/**
 * Keyboard bindings: what a shortcut is, whether an event matches it, and how to print it.
 * `mod` is ⌘ on macOS and Ctrl elsewhere; `ctrl` is the Control key itself (⌃Tab on macOS).
 * Keys are physical codes (KeyW, Digit1, BracketRight), so ⌥ shortcuts still match on macOS,
 * where ⌥W types "∑".
 */
import { isMac } from "@/lib/utils";

export interface Binding {
  code: string;
  mod?: boolean;
  ctrl?: boolean;
  alt?: boolean;
  shift?: boolean;
  /** Browsers keep this shortcut for themselves (⌘W, ⌘T…); it works in the desktop app only. */
  desktopOnly?: boolean;
}

export function matches(e: KeyboardEvent, b: Binding): boolean {
  if (e.code !== b.code) return false;
  if (e.altKey !== !!b.alt || e.shiftKey !== !!b.shift) return false;
  if (isMac) return e.metaKey === !!b.mod && e.ctrlKey === !!b.ctrl;
  return !e.metaKey && e.ctrlKey === (!!b.mod || !!b.ctrl);
}

const names: Record<string, string> = {
  Tab: "Tab",
  Slash: "/",
  Comma: ",",
  Period: ".",
  Equal: "=",
  Minus: "-",
  Space: "Space",
  Home: "Home",
  End: "End",
  PageUp: "PgUp",
  PageDown: "PgDn",
  BracketLeft: "[",
  BracketRight: "]",
  Enter: "↵",
  Escape: "Esc",
  Backspace: "⌫",
  ArrowUp: "↑",
  ArrowDown: "↓",
  ArrowLeft: "←",
  ArrowRight: "→",
};

function keyName(code: string): string {
  if (code.startsWith("Key")) return code.slice(3);
  if (code.startsWith("Digit")) return code.slice(5);
  return names[code] ?? code;
}

/** A shortcut as this platform prints it; ⌘ (Ctrl elsewhere) unless other modifiers are given. */
export function shortcut(code: string, mods: Omit<Binding, "code"> = { mod: true }): string {
  return label({ code, ...mods });
}

/** "⇧⌘T" on macOS, "Ctrl+Shift+T" elsewhere. */
export function label(b: Binding): string {
  const key = keyName(b.code);
  if (isMac) return `${b.ctrl ? "⌃" : ""}${b.alt ? "⌥" : ""}${b.shift ? "⇧" : ""}${b.mod ? "⌘" : ""}${key}`;
  const parts = [b.mod || b.ctrl ? "Ctrl" : "", b.alt ? "Alt" : "", b.shift ? "Shift" : "", key].filter(Boolean);
  return parts.join("+");
}

/** True when focus is in a field that takes typed text. */
export function typing(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false;
  if (target.isContentEditable) return true;
  if (target instanceof HTMLTextAreaElement) return true;
  if (target instanceof HTMLInputElement) return !["checkbox", "radio", "button", "submit", "range", "color", "file"].includes(target.type);
  return false;
}
