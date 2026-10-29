/**
 * Vim keys (Settings → Keyboard): with them on, grids move with h j k l and edit with i, tabs
 * switch with gt / gT, Ctrl+h/j/k/l (Alt outside macOS) move between the sidebar and the
 * view's panes, and `:` opens the palette. Off by default: in a grid they take the place of
 * typing to edit. The tree and the lists take j/k either way. Stored in localStorage (`mesa.vim`).
 */
import { create } from "zustand";

const KEY = "mesa.vim";

function readStored(): boolean {
  try {
    return localStorage.getItem(KEY) === "on";
  } catch {
    return false;
  }
}

interface VimState {
  vim: boolean;
  setVim: (on: boolean) => void;
}

export const useVim = create<VimState>((set) => ({
  vim: readStored(),
  setVim: (vim) => {
    try {
      localStorage.setItem(KEY, vim ? "on" : "off");
    } catch {
      // Private mode or blocked storage: the choice lasts for this page only.
    }
    set({ vim });
  },
}));
