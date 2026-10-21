/** The workspace frame (`Main` board): env stripe, rail, tabs, the active view and the status bar. */
import * as React from "react";
import { Link, Outlet } from "@tanstack/react-router";
import { useQueryClient } from "@tanstack/react-query";
import { useEffect } from "react";

import { useGetConnection } from "@/lib/api/connections/connections";
import { errorMessage } from "@/lib/api/fetch";
import { EmptyState } from "@/ui/controls";
import { useUi } from "@/app/ui.store";
import { Rail } from "@/features/explorer/Rail";
import { StatusBar } from "@/features/explorer/StatusBar";
import { TabStrip } from "@/features/explorer/TabStrip";

import { WorkspaceProvider } from "./context";

export function WorkspaceLayout({ connectionId }: { connectionId: string }) {
  const conn = useGetConnection(connectionId, { query: { refetchInterval: 60_000 } });
  const collapsed = useUi((s) => s.railCollapsed);
  const qc = useQueryClient();

  // The unlock window ends on the server; refresh the connection when it does.
  const until = conn.data?.unlockedUntil;
  useEffect(() => {
    if (!until) return;
    const ms = Date.parse(until) - Date.now();
    if (ms <= 0) return;
    const t = setTimeout(() => void conn.refetch(), ms + 500);
    return () => clearTimeout(t);
  }, [until, conn]);

  if (conn.isPending) return <div className="m-app" style={{ height: "100%" }} />;
  if (conn.isError || !conn.data) {
    return (
      <div className="m-app" style={{ height: "100%", display: "grid", placeItems: "center", padding: 24 }}>
        <EmptyState
          title="Connection not found"
          description={errorMessage(conn.error)}
          actions={
            <Link to="/" className="m-btn m-btn--outline" onClick={() => void qc.invalidateQueries()}>
              All connections
            </Link>
          }
        />
      </div>
    );
  }
  const env = conn.data.environment;
  return (
    <WorkspaceProvider connection={conn.data}>
      <div className="m-app" style={{ height: "100%", display: "flex", flexDirection: "column", overflow: "hidden" }}>
        <div className={`m-env-stripe m-env--${env}`} aria-hidden="true" />
        <div style={{ flex: 1, minHeight: 0, display: "grid", gridTemplateColumns: `${collapsed ? "var(--size-sidebar-collapsed)" : "var(--size-sidebar)"} minmax(0, 1fr)` }}>
          <Rail />
          <main style={{ minWidth: 0, minHeight: 0, display: "flex", flexDirection: "column" }}>
            <TabStrip />
            <div style={{ flex: 1, minHeight: 0, display: "flex", flexDirection: "column" }}>
              <React.Suspense fallback={null}>
                <Outlet />
              </React.Suspense>
            </div>
            <StatusBar />
          </main>
        </div>
      </div>
    </WorkspaceProvider>
  );
}
