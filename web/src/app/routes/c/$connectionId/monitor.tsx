import { createFileRoute } from "@tanstack/react-router";

import { MonitorPage } from "@/features/monitor/MonitorPage";

export const Route = createFileRoute("/c/$connectionId/monitor")({
  validateSearch: (s: Record<string, unknown>): { view: "overview" | "sessions"; database?: string } => ({
    view: s.view === "sessions" ? "sessions" : "overview",
    ...(typeof s.database === "string" ? { database: s.database } : {}),
  }),
  component: function MonitorRoute() {
    const { view, database } = Route.useSearch();
    return <MonitorPage view={view} database={database} />;
  },
});
