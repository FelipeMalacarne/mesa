import { createFileRoute } from "@tanstack/react-router";

import { WorkspaceLayout } from "@/features/workspace/WorkspaceLayout";

export const Route = createFileRoute("/c/$connectionId")({
  component: function Workspace() {
    const { connectionId } = Route.useParams();
    return <WorkspaceLayout connectionId={connectionId} />;
  },
});
