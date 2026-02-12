import { Plus } from "lucide-react";

import {
  SidebarGroup,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar";
import { ResponsiveDialog } from "./responsive-dialog";
import { useState } from "react";
import { ConnectionForm } from "./connection-form";
import { Button } from "./ui/button";
import type { Connection } from "@/api";
import { useRouterState } from "@tanstack/react-router";

import {
  NavConnectionsProvider,
  useConnectionTreeData,
  ConnectionMenuItem,
} from "./nav-connections/index";

export function NavConnections({
  connections,
  isLoading,
}: {
  connections: Connection[];
  isLoading?: boolean;
}) {
  return (
    <NavConnectionsProvider>
      <NavConnectionsContent connections={connections} isLoading={isLoading} />
    </NavConnectionsProvider>
  );
}

function NavConnectionsContent({
  connections,
  isLoading,
}: {
  connections: Connection[];
  isLoading?: boolean;
}) {
  const [createConnectionOpen, setCreateConnectionOpen] = useState(false);
  const { connectionNodes } = useConnectionTreeData(connections);
  const currentPath = useRouterState({
    select: (state) => state.location.pathname,
  });
  const pathSegments = currentPath.split("/").filter(Boolean);
  let activeConnectionId: string | undefined;
  let activeDatabaseName: string | undefined;
  let activeTableName: string | undefined;

  if (pathSegments[0] === "connections" && pathSegments[1]) {
    activeConnectionId = decodeURIComponent(pathSegments[1]);
    if (pathSegments[2] === "databases" && pathSegments[3]) {
      activeDatabaseName = decodeURIComponent(pathSegments[3]);
      if (pathSegments[4] === "tables" && pathSegments[5]) {
        activeTableName = decodeURIComponent(pathSegments[5]);
      }
    }
  }

  return (
    <>
      <SidebarGroup>
        <SidebarGroupLabel className="flex items-center justify-between">
          Connections
          <Button
            onClick={() => setCreateConnectionOpen(true)}
            variant="ghost"
            size="icon"
            className="size-6"
          >
            <Plus />
          </Button>
        </SidebarGroupLabel>
        <SidebarMenu>
          {isLoading ? (
            <SidebarMenuItem>
              <SidebarMenuButton disabled>
                Loading connections...
              </SidebarMenuButton>
            </SidebarMenuItem>
          ) : null}
          {connectionNodes.map((node) => (
            <ConnectionMenuItem
              key={node.connection.id}
              node={node}
              activeState={{
                connectionId: activeConnectionId,
                databaseName: activeDatabaseName,
                tableName: activeTableName,
              }}
            />
          ))}
        </SidebarMenu>
      </SidebarGroup>

      <ResponsiveDialog
        title="Add Connection"
        isOpen={createConnectionOpen}
        setIsOpen={setCreateConnectionOpen}
      >
        <ConnectionForm onSuccess={() => setCreateConnectionOpen(false)} />
      </ResponsiveDialog>
    </>
  );
}
