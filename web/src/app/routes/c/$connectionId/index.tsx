import { createFileRoute } from "@tanstack/react-router";

import { WorkspaceHome } from "@/features/workspace/WorkspaceHome";

export const Route = createFileRoute("/c/$connectionId/")({
  component: WorkspaceHome,
});
