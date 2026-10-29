/**
 * Which tree nodes are open, per connection, persisted so the tree looks the same after a reload.
 * `reveal` opens the branches above an object and asks the tree to scroll to it and focus it;
 * `treeNode` names objects the way the tree's `data-node` attributes do.
 */
import { create } from "zustand";
import { persist } from "zustand/middleware";

export type TreeObjectKind = "table" | "view" | "enum" | "function" | "sequence" | "column";

export interface RevealRequest {
  connectionId: string;
  node: string;
  n: number;
}

interface TreeState {
  open: Record<string, Record<string, true>>;
  /** The node to scroll to and focus; not persisted. */
  revealing: RevealRequest | null;
  isOpen: (connectionId: string, node: string) => boolean;
  toggle: (connectionId: string, node: string, force?: boolean) => void;
  reveal: (connectionId: string, kind: TreeObjectKind, database: string, schema: string, name: string, table?: string) => void;
  /** Asks the tree to focus a node that is already visible. */
  focusNode: (connectionId: string, node: string) => void;
}

/** The `data-node` id of an object in the tree. */
export function treeNode(kind: TreeObjectKind, database: string, schema: string, name: string, table?: string): string {
  return kind === "column" ? `column:${database}.${schema}.${table}.${name}` : `${kind}:${database}.${schema}.${name}`;
}

const FOLDERS: Record<Exclude<TreeObjectKind, "column">, string> = { table: "tables", view: "views", enum: "enums", function: "functions", sequence: "sequences" };

export const useTree = create<TreeState>()(
  persist(
    (set, get) => ({
      open: {},
      revealing: null,
      isOpen: (c, n) => !!get().open[c]?.[n],
      toggle: (c, n, force) => {
        const conn = { ...(get().open[c] ?? {}) };
        const next = force ?? !conn[n];
        if (next) conn[n] = true;
        else delete conn[n];
        set({ open: { ...get().open, [c]: conn } });
      },
      reveal: (c, kind, database, schema, name, table) => {
        const conn = { ...(get().open[c] ?? {}) };
        const schemaId = `schema:${database}.${schema}`;
        const folder = FOLDERS[kind === "column" ? "table" : kind];
        conn[`db:${database}`] = true;
        conn[schemaId] = true;
        delete conn[`closed:${schemaId}`];
        // Tables is open unless closed; the other folders are closed unless opened.
        conn[`${schemaId}/${folder}`] = true;
        delete conn[`closed:${schemaId}/${folder}`];
        if (kind === "column") conn[`table:${database}.${schema}.${table}`] = true;
        set({ open: { ...get().open, [c]: conn }, revealing: { connectionId: c, node: treeNode(kind, database, schema, name, table), n: (get().revealing?.n ?? 0) + 1 } });
      },
      focusNode: (c, node) => set({ revealing: { connectionId: c, node, n: (get().revealing?.n ?? 0) + 1 } }),
    }),
    { name: "mesa.tree", partialize: (s) => ({ open: s.open }) },
  ),
);

export function useNodeOpen(connectionId: string, node: string): boolean {
  return useTree((s) => !!s.open[connectionId]?.[node]);
}
