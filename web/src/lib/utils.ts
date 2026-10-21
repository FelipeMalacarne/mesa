import { clsx, type ClassValue } from "clsx";
import { twMerge } from "tailwind-merge";

/** Joins class names; Tailwind conflicts resolve to the last one. */
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

/** True on macOS-like platforms, where shortcuts read ⌘ instead of Ctrl. */
export const isMac: boolean =
  typeof navigator !== "undefined" && /Mac|iPhone|iPad/.test(navigator.platform ?? navigator.userAgent ?? "");

/** The modifier key of a keyboard event that counts as ⌘ (Cmd on macOS, Ctrl elsewhere). */
export function isCmd(e: KeyboardEvent | React.KeyboardEvent): boolean {
  return isMac ? e.metaKey : e.ctrlKey;
}

/** A short unique id for client-side objects (staged rows, query tabs). */
export function uid(prefix = "id"): string {
  return `${prefix}_${Math.random().toString(36).slice(2, 10)}`;
}
