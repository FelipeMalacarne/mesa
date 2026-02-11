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
            <ConnectionMenuItem key={node.connection.id} node={node} />
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
