/**
 * Workspace tabs, one list per connection, persisted so they survive reloads.
 * Routes open their tab with `useTabs.getState().open(connectionId, tab)` on mount, so a tab
 * exists even when the person arrives by URL; `tabLink` says where each tab lives.
 */
import { create } from "zustand";
import { persist } from "zustand/middleware";

export type TabKind = "table" | "query" | "monitor" | "designer" | "roles" | "activity";

export type TableView = "data" | "structure" | "indexes";

interface TabBase {
  id: string;
  kind: TabKind;
  title: string;
}

export interface TableTab extends TabBase {
  kind: "table";
  database: string;
  schema: string;
  table: string;
  /** Views open as read-only table tabs. */
  objectKind: "table" | "view";
  view: TableView;
}

export interface QueryTab extends TabBase {
  kind: "query";
  /** The editor's text, kept with the tab so it survives reloads. */
  sql?: string;
  /** The database the query runs on; the connection's default when absent. */
  database?: string;
  /** Set when the tab holds a saved query. */
  savedQueryId?: string;
}

export interface MonitorTab extends TabBase {
  kind: "monitor";
  view: "overview" | "sessions";
}

export interface DesignerTab extends TabBase {
  kind: "designer";
  database: string;
}

export interface RolesTab extends TabBase {
  kind: "roles";
}

export interface ActivityTab extends TabBase {
  kind: "activity";
}

export type Tab = TableTab | QueryTab | MonitorTab | DesignerTab | RolesTab | ActivityTab;

export function tableTabId(database: string, schema: string, table: string): string {
  return `table:${database}.${schema}.${table}`;
}

interface ConnectionTabs {
  tabs: Tab[];
  activeId: string | null;
  nextQuery: number;
  /** Recently closed tabs, newest last, for reopening. */
  closed?: Tab[];
}

const empty: ConnectionTabs = { tabs: [], activeId: null, nextQuery: 1 };
const keepClosed = 20;

interface TabsState {
  byConnection: Record<string, ConnectionTabs>;
  /** Adds the tab when missing (merging fields when present), activates it and returns it. */
  open: (connectionId: string, tab: Tab) => Tab;
  activate: (connectionId: string, id: string) => void;
  /** Closes a tab; returns the tab that is active afterwards (null when none are left). */
  close: (connectionId: string, id: string) => Tab | null;
  /** Reopens the most recently closed tab that is not open; returns it, or null. */
  reopen: (connectionId: string) => Tab | null;
  update: (connectionId: string, id: string, patch: Partial<Tab>) => void;
  /** Opens a fresh "Query N" tab and returns it. */
  newQuery: (connectionId: string) => QueryTab;
}

export const useTabs = create<TabsState>()(
  persist(
    (set, get) => ({
      byConnection: {},
      open: (connectionId, tab) => {
        const conn = get().byConnection[connectionId] ?? empty;
        const existing = conn.tabs.find((t) => t.id === tab.id);
        const merged = existing ? ({ ...existing, ...tab } as Tab) : tab;
        const tabs = existing ? conn.tabs.map((t) => (t.id === tab.id ? merged : t)) : [...conn.tabs, merged];
        set({ byConnection: { ...get().byConnection, [connectionId]: { ...conn, tabs, activeId: tab.id } } });
        return merged;
      },
      activate: (connectionId, id) => {
        const conn = get().byConnection[connectionId] ?? empty;
        if (conn.activeId === id) return;
        set({ byConnection: { ...get().byConnection, [connectionId]: { ...conn, activeId: id } } });
      },
      close: (connectionId, id) => {
        const conn = get().byConnection[connectionId] ?? empty;
        const index = conn.tabs.findIndex((t) => t.id === id);
        if (index < 0) return conn.tabs.find((t) => t.id === conn.activeId) ?? null;
        const tabs = conn.tabs.filter((t) => t.id !== id);
        let activeId = conn.activeId;
        if (activeId === id) {
          const next = tabs[Math.min(index, tabs.length - 1)];
          activeId = next ? next.id : null;
        }
        const closed = [...(conn.closed ?? []).filter((t) => t.id !== id), conn.tabs[index]].slice(-keepClosed);
        set({ byConnection: { ...get().byConnection, [connectionId]: { ...conn, tabs, activeId, closed } } });
        return tabs.find((t) => t.id === activeId) ?? null;
      },
      reopen: (connectionId) => {
        const conn = get().byConnection[connectionId] ?? empty;
        const closed = [...(conn.closed ?? [])];
        let tab: Tab | undefined;
        while ((tab = closed.pop()) && conn.tabs.some((t) => t.id === tab!.id));
        set({ byConnection: { ...get().byConnection, [connectionId]: { ...conn, closed } } });
        return tab ? get().open(connectionId, tab) : null;
      },
      update: (connectionId, id, patch) => {
        const conn = get().byConnection[connectionId] ?? empty;
        const tabs = conn.tabs.map((t) => (t.id === id ? ({ ...t, ...patch } as Tab) : t));
        set({ byConnection: { ...get().byConnection, [connectionId]: { ...conn, tabs } } });
      },
      newQuery: (connectionId) => {
        const conn = get().byConnection[connectionId] ?? empty;
        const n = conn.nextQuery;
        const tab: QueryTab = { id: `query:${n}`, kind: "query", title: `Query ${n}` };
        set({
          byConnection: {
            ...get().byConnection,
            [connectionId]: { ...conn, tabs: [...conn.tabs, tab], activeId: tab.id, nextQuery: n + 1 },
          },
        });
        return tab;
      },
    }),
    { name: "mesa.tabs" },
  ),
);

const noTabs: Tab[] = [];

export function useConnectionTabs(connectionId: string): Tab[] {
  return useTabs((s) => s.byConnection[connectionId]?.tabs ?? noTabs);
}

export function useActiveTab(connectionId: string): Tab | null {
  return useTabs((s) => {
    const conn = s.byConnection[connectionId];
    if (!conn || !conn.activeId) return null;
    return conn.tabs.find((t) => t.id === conn.activeId) ?? null;
  });
}

/** Where a tab lives, as router navigation options. */
export function tabLink(connectionId: string, tab: Tab) {
  switch (tab.kind) {
    case "table":
      return {
        to: "/c/$connectionId/db/$database/t/$schema/$table" as const,
        params: { connectionId, database: tab.database, schema: tab.schema, table: tab.table },
        search: { view: tab.view, kind: tab.objectKind },
      };
    case "query":
      return { to: "/c/$connectionId/sql" as const, params: { connectionId }, search: { tab: tab.id } };
    case "monitor":
      return { to: "/c/$connectionId/monitor" as const, params: { connectionId }, search: { view: tab.view } };
    case "designer":
      return { to: "/c/$connectionId/designer" as const, params: { connectionId }, search: { database: tab.database } };
    case "roles":
      return { to: "/c/$connectionId/roles" as const, params: { connectionId }, search: {} };
    case "activity":
      return { to: "/c/$connectionId/activity" as const, params: { connectionId }, search: {} };
  }
}
