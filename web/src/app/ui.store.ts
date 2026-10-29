/**
 * Workspace chrome state that is not tied to a route: the inspector, the collapsed rail,
 * the command palette, the shortcuts sheet and the review dialog. Inspector and rail choices persist per viewer.
 */
import { create } from "zustand";
import { persist } from "zustand/middleware";

interface UiState {
  inspectorOpen: boolean;
  railCollapsed: boolean;
  paletteOpen: boolean;
  helpOpen: boolean;
  reviewOpen: boolean;
  setInspectorOpen: (open: boolean) => void;
  toggleInspector: () => void;
  toggleRail: () => void;
  setPaletteOpen: (open: boolean) => void;
  setHelpOpen: (open: boolean) => void;
  setReviewOpen: (open: boolean) => void;
}

const wideEnough = typeof window === "undefined" ? true : window.innerWidth >= 1280;

export const useUi = create<UiState>()(
  persist(
    (set, get) => ({
      inspectorOpen: wideEnough,
      railCollapsed: false,
      paletteOpen: false,
      helpOpen: false,
      reviewOpen: false,
      setInspectorOpen: (inspectorOpen) => set({ inspectorOpen }),
      toggleInspector: () => set({ inspectorOpen: !get().inspectorOpen }),
      toggleRail: () => set({ railCollapsed: !get().railCollapsed }),
      setPaletteOpen: (paletteOpen) => set({ paletteOpen }),
      setHelpOpen: (helpOpen) => set({ helpOpen }),
      setReviewOpen: (reviewOpen) => set({ reviewOpen }),
    }),
    {
      name: "mesa.ui",
      partialize: (s) => ({ inspectorOpen: s.inspectorOpen, railCollapsed: s.railCollapsed }),
    },
  ),
);
