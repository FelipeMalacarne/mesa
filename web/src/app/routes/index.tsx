import { createFileRoute } from "@tanstack/react-router";

import { ConnectionsPage } from "@/features/connections/ConnectionsPage";

export const Route = createFileRoute("/")({
  component: ConnectionsPage,
});
