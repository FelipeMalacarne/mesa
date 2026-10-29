/**
 * Row density for the grid and the tree: compact (the default, 28px rows) or comfortable (36px).
 * Stored in localStorage (`mesa.density`) and applied as `data-density` on <html>, which swaps
 * `--size-row` for `--size-row-comfortable`; index.html applies it before paint like the theme.
 */
import { create } from "zustand";

export type Density = "compact" | "comfortable";

const KEY = "mesa.density";

function readStored(): Density {
  try {
    return localStorage.getItem(KEY) === "comfortable" ? "comfortable" : "compact";
  } catch {
    return "compact";
  }
}

export function applyDensity(density: Density): void {
  if (typeof document === "undefined") return;
  document.documentElement.dataset.density = density;
}

interface DensityState {
  density: Density;
  setDensity: (density: Density) => void;
}

export const useDensity = create<DensityState>((set) => ({
  density: readStored(),
  setDensity: (density) => {
    try {
      localStorage.setItem(KEY, density);
    } catch {
      // Private mode or blocked storage: the choice lasts for this page only.
    }
    applyDensity(density);
    set({ density });
  },
}));

applyDensity(useDensity.getState().density);
