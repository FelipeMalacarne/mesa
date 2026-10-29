/**
 * Tab moves shared by the tab strip, the keyboard and the palette: each updates the tab store
 * and navigates to the tab that ends up active. They return false when there was nothing to do.
 */
import type { useNavigate } from "@tanstack/react-router";

import { focusMain } from "../workspace/focus";
import { tabLink, useTabs, type Tab } from "./tabs.store";

type Navigate = ReturnType<typeof useNavigate>;

export function activeTab(connectionId: string): Tab | null {
  const conn = useTabs.getState().byConnection[connectionId];
  return conn?.tabs.find((t) => t.id === conn.activeId) ?? null;
}

export function goToTab(navigate: Navigate, connectionId: string, tab: Tab): void {
  void navigate(tabLink(connectionId, tab));
}

/** Opens a fresh query tab with the caret in its editor. */
export function newQueryTab(navigate: Navigate, connectionId: string): void {
  goToTab(navigate, connectionId, useTabs.getState().newQuery(connectionId));
  focusMain();
}

/**
 * Closes a tab. Closing the active one goes to its neighbour (or home) first and removes the
 * tab once there: the view still on screen would otherwise see its tab vanish, and the SQL
 * editor answers a missing tab by opening a fresh one.
 */
export function closeTab(navigate: Navigate, connectionId: string, id: string): boolean {
  const tabs = useTabs.getState();
  const conn = tabs.byConnection[connectionId];
  const index = conn?.tabs.findIndex((t) => t.id === id) ?? -1;
  if (!conn || index < 0) return false;
  if (conn.activeId !== id) {
    tabs.close(connectionId, id);
    return true;
  }
  const next = conn.tabs[index + 1] ?? conn.tabs[index - 1];
  let leaving: Promise<void>;
  if (next) leaving = navigate(tabLink(connectionId, next));
  else {
    // Home sends you back to the active tab, so there must be none.
    tabs.activate(connectionId, null);
    leaving = navigate({ to: "/c/$connectionId", params: { connectionId } });
  }
  void leaving.finally(() => useTabs.getState().close(connectionId, id));
  return true;
}

export function reopenTab(navigate: Navigate, connectionId: string): boolean {
  const tab = useTabs.getState().reopen(connectionId);
  if (tab) goToTab(navigate, connectionId, tab);
  return !!tab;
}

/** Moves `delta` tabs from the active one, wrapping around the ends. */
export function cycleTab(navigate: Navigate, connectionId: string, delta: number): boolean {
  const tabs = useTabs.getState().byConnection[connectionId]?.tabs ?? [];
  if (tabs.length === 0) return false;
  const at = tabs.findIndex((t) => t.id === activeTab(connectionId)?.id);
  const next = tabs[(((at < 0 ? (delta > 0 ? -1 : 0) : at) + delta) % tabs.length + tabs.length) % tabs.length];
  goToTab(navigate, connectionId, next);
  return true;
}

/** Goes to the n-th tab (1-based); n = -1 is the last one. */
export function goToTabAt(navigate: Navigate, connectionId: string, n: number): boolean {
  const tabs = useTabs.getState().byConnection[connectionId]?.tabs ?? [];
  const tab = n < 0 ? tabs[tabs.length + n] : tabs[n - 1];
  if (!tab) return false;
  goToTab(navigate, connectionId, tab);
  return true;
}
