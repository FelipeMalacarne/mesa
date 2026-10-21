import { createFileRoute } from "@tanstack/react-router";

import { RolesPage } from "@/features/roles/RolesPage";

export const Route = createFileRoute("/c/$connectionId/roles")({
  component: RolesPage,
});
