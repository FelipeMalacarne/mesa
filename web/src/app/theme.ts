/**
 * Theme: dark is the default and the app opens in it. The choice is stored in
 * localStorage (`mesa.theme`) and applied as the `.dark` class plus `data-theme` on <html>.
 * index.html applies the stored choice before React loads so there is no flash.
 */
import { create } from "zustand";

export type Theme = "dark" | "light";

const KEY = "mesa.theme";

function readStored(): Theme {
  try {
    return localStorage.getItem(KEY) === "light" ? "light" : "dark";
  } catch {
    return "dark";
  }
}

export function applyTheme(theme: Theme): void {
  if (typeof document === "undefined") return;
  const el = document.documentElement;
  el.classList.toggle("dark", theme === "dark");
  el.dataset.theme = theme;
}

interface ThemeState {
  theme: Theme;
  setTheme: (theme: Theme) => void;
  toggle: () => void;
}

export const useTheme = create<ThemeState>((set, get) => ({
  theme: readStored(),
  setTheme: (theme) => {
    try {
      localStorage.setItem(KEY, theme);
    } catch {
      // Private mode or blocked storage: the choice lasts for this page only.
    }
    applyTheme(theme);
    set({ theme });
  },
  toggle: () => get().setTheme(get().theme === "dark" ? "light" : "dark"),
}));

applyTheme(useTheme.getState().theme);
