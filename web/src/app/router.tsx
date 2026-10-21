/**
 * The router. Routes are file-based under src/app/routes (see vite.config.ts); the tree
 * is generated into src/app/routeTree.gen.ts by the TanStack Router plugin.
 */
import { createRouter } from "@tanstack/react-router";

import { routeTree } from "./routeTree.gen";
import { queryClient } from "./providers";

export const router = createRouter({
  routeTree,
  context: { queryClient },
  defaultPreload: "intent",
  defaultPreloadStaleTime: 0,
  scrollRestoration: false,
  defaultStructuralSharing: true,
});

declare module "@tanstack/react-router" {
  interface Register {
    router: typeof router;
  }
}
