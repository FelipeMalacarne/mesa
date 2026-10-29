import type { QueryClient } from "@tanstack/react-query";
import { createRootRouteWithContext, Link, Outlet } from "@tanstack/react-router";

import { EmptyState } from "@/ui/controls";
import { errorMessage } from "@/lib/api/fetch";

import { CommandPalette } from "../CommandPalette";
import { SettingsDialog } from "../SettingsDialog";
import { ShortcutsDialog } from "../ShortcutsDialog";
import { useShortcuts } from "../shortcuts";

export const Route = createRootRouteWithContext<{ queryClient: QueryClient }>()({
  component: Root,
  notFoundComponent: () => (
    <div className="m-app" style={{ height: "100%", display: "grid", placeItems: "center", padding: 24 }}>
      <EmptyState
        title="Nothing here"
        description="This page does not exist."
        actions={
          <Link to="/" className="m-btn m-btn--outline">
            All connections
          </Link>
        }
      />
    </div>
  ),
  errorComponent: ({ error }) => (
    <div className="m-app" style={{ height: "100%", display: "grid", placeItems: "center", padding: 24 }}>
      <EmptyState
        title="Something went wrong"
        description={errorMessage(error)}
        actions={
          <Link to="/" className="m-btn m-btn--outline">
            All connections
          </Link>
        }
      />
    </div>
  ),
});

function Root() {
  useShortcuts();
  return (
    <>
      <Outlet />
      <CommandPalette />
      <ShortcutsDialog />
      <SettingsDialog />
    </>
  );
}
