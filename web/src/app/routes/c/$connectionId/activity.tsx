import { createFileRoute } from "@tanstack/react-router";

import { ActivityPage } from "@/features/activity/ActivityPage";

export const Route = createFileRoute("/c/$connectionId/activity")({
  component: ActivityPage,
});
