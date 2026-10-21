import { createFileRoute } from "@tanstack/react-router";

import { DesignerPage } from "@/features/designer/DesignerPage";

export const Route = createFileRoute("/c/$connectionId/designer")({
  validateSearch: (s: Record<string, unknown>): { database?: string; fresh?: boolean } => ({
    ...(typeof s.database === "string" ? { database: s.database } : {}),
    ...(s.fresh === true || s.fresh === "true" ? { fresh: true } : {}),
  }),
  component: function DesignerRoute() {
    const { database, fresh } = Route.useSearch();
    return <DesignerPage database={database} fresh={!!fresh} />;
  },
});
