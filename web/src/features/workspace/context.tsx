/**
 * The connection a workspace is open on. The workspace layout route provides it; every view
 * under /c/$connectionId reads it with `useWorkspace()`.
 */
import * as React from "react";

import type { Connection } from "@/lib/api/mesaAPI.schemas";

export interface Workspace {
  connection: Connection;
  connectionId: string;
  /** True while the connection is read-only and not unlocked. */
  readOnly: boolean;
}

const Ctx = React.createContext<Workspace | null>(null);

export function WorkspaceProvider({ connection, children }: { connection: Connection; children: React.ReactNode }) {
  const value = React.useMemo<Workspace>(
    () => ({
      connection,
      connectionId: connection.id,
      readOnly: connection.readOnly && !(connection.unlockedUntil && Date.parse(connection.unlockedUntil) > Date.now()),
    }),
    [connection],
  );
  return <Ctx.Provider value={value}>{children}</Ctx.Provider>;
}

export function useWorkspace(): Workspace {
  const ws = React.useContext(Ctx);
  if (!ws) throw new Error("useWorkspace outside a workspace");
  return ws;
}
