import { createFileRoute } from "@tanstack/react-router";

import { SqlPage } from "@/features/sql/SqlPage";

export const Route = createFileRoute("/c/$connectionId/sql")({
  validateSearch: (s: Record<string, unknown>): { tab?: string } => (typeof s.tab === "string" ? { tab: s.tab } : {}),
  component: function SqlRoute() {
    const { tab } = Route.useSearch();
    return <SqlPage tabId={tab} />;
  },
});
