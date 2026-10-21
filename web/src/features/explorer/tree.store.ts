/** Which tree nodes are open, per connection, persisted so the tree looks the same after a reload. */
import { create } from "zustand";
import { persist } from "zustand/middleware";

interface TreeState {
  open: Record<string, Record<string, true>>;
  isOpen: (connectionId: string, node: string) => boolean;
  toggle: (connectionId: string, node: string, force?: boolean) => void;
}

export const useTree = create<TreeState>()(
  persist(
    (set, get) => ({
      open: {},
      isOpen: (c, n) => !!get().open[c]?.[n],
      toggle: (c, n, force) => {
        const conn = { ...(get().open[c] ?? {}) };
        const next = force ?? !conn[n];
        if (next) conn[n] = true;
        else delete conn[n];
        set({ open: { ...get().open, [c]: conn } });
      },
    }),
    { name: "mesa.tree" },
  ),
);

export function useNodeOpen(connectionId: string, node: string): boolean {
  return useTree((s) => !!s.open[connectionId]?.[node]);
}
