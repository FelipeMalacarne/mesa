import { Plus } from "lucide-react";
import { useState } from "react";
import { useParams } from "@tanstack/react-router";

import {
  SidebarGroup,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar";
import { ResponsiveDialog } from "./responsive-dialog";
import { ConnectionForm } from "./connection-form";
import { Button } from "./ui/button";
import type { Connection } from "@/api";

import {
  NavConnectionsProvider,
  ConnectionMenuItem,
  useNavConnectionsState,
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
  const { openConnections } = useNavConnectionsState();
  const params = useParams({ strict: false });
  const activeConnectionId = params.connectionId;
  const activeDatabaseName = params.databaseName;
  const activeTableName = params.tableName;

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
          {connections.map((connection) => (
            <ConnectionMenuItem
              key={connection.id}
              connection={connection}
              isOpen={openConnections[connection.id] ?? false}
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
