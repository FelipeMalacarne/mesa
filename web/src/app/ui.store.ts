/**
 * Workspace chrome state that is not tied to a route: the inspector, the collapsed rail,
 * the command palette, the review and settings dialogs, and requests to move keyboard focus
 * into the rail. Inspector and rail choices persist per viewer.
 */
import { create } from "zustand";
import { persist } from "zustand/middleware";

export type SettingsSection = "appearance" | "keyboard" | "about";

/** Where in the rail keyboard focus should go; `n` changes on every request so a repeat still fires. */
export interface FocusRequest {
  target: "filter" | "tree";
  n: number;
}

interface UiState {
  inspectorOpen: boolean;
  railCollapsed: boolean;
  paletteOpen: boolean;
  reviewOpen: boolean;
  settingsOpen: boolean;
  settingsSection: SettingsSection;
  focusRequest: FocusRequest | null;
  setInspectorOpen: (open: boolean) => void;
  toggleInspector: () => void;
  toggleRail: () => void;
  setPaletteOpen: (open: boolean) => void;
  setReviewOpen: (open: boolean) => void;
  setSettingsOpen: (open: boolean) => void;
  openSettings: (section: SettingsSection) => void;
  /** Opens the rail when it is collapsed and asks it to focus its filter or its tree. */
  focusRail: (target: FocusRequest["target"]) => void;
}

const wideEnough = typeof window === "undefined" ? true : window.innerWidth >= 1280;

export const useUi = create<UiState>()(
  persist(
    (set, get) => ({
      inspectorOpen: wideEnough,
      railCollapsed: false,
      paletteOpen: false,
      reviewOpen: false,
      settingsOpen: false,
      settingsSection: "appearance",
      focusRequest: null,
      setInspectorOpen: (inspectorOpen) => set({ inspectorOpen }),
      toggleInspector: () => set({ inspectorOpen: !get().inspectorOpen }),
      toggleRail: () => set({ railCollapsed: !get().railCollapsed }),
      setPaletteOpen: (paletteOpen) => set({ paletteOpen }),
      setReviewOpen: (reviewOpen) => set({ reviewOpen }),
      setSettingsOpen: (settingsOpen) => set({ settingsOpen }),
      openSettings: (settingsSection) => set({ settingsOpen: true, settingsSection, paletteOpen: false }),
      focusRail: (target) => set({ railCollapsed: false, focusRequest: { target, n: (get().focusRequest?.n ?? 0) + 1 } }),
    }),
    {
      name: "mesa.ui",
      partialize: (s) => ({ inspectorOpen: s.inspectorOpen, railCollapsed: s.railCollapsed }),
    },
  ),
);
