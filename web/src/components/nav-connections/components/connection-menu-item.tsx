import { ChevronRight, HardDrive } from "lucide-react";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import {
  SidebarMenuAction,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSub,
} from "@/components/ui/sidebar";
import { Link } from "@tanstack/react-router";

import { Connection } from "@/api";
import type { ConnectionTreeNode } from "../state/types";
import { ConnectionDatabases } from "./connection-databases";
import { DisabledSubButton } from "./disabled-sub-button";
import { useNavConnectionsState } from "../state/context";

export type ConnectionMenuItemProps = {
  node: ConnectionTreeNode;
  activeState: {
    connectionId?: string;
    databaseName?: string;
    tableName?: string;
  };
};

export const ConnectionMenuItem = ({
  node,
  activeState,
}: ConnectionMenuItemProps) => {
  const { setConnectionOpen } = useNavConnectionsState();
  const connection = node.connection;
  const connectionStatus = node.status;
  const isConnectionActive =
    activeState.connectionId === connection.id && !activeState.databaseName;
  const statusDot =
    connectionStatus === Connection.status.OK
      ? "bg-emerald-500"
      : connectionStatus === Connection.status.ERROR
        ? "bg-rose-500"
        : "bg-muted-foreground/40";

  return (
    <Collapsible
      asChild
      open={node.isOpen}
      onOpenChange={(nextOpen) => setConnectionOpen(connection.id, nextOpen)}
      className="group/connection-collapsible"
    >
      <SidebarMenuItem>
        <SidebarMenuButton
          asChild
          tooltip={connection.name}
          isActive={isConnectionActive}
        >
          <Link
            to="/connections/$connectionId"
            params={{ connectionId: connection.id }}
          >
            <HardDrive />
            <span>{connection.name}</span>
            <span className="ml-auto flex items-center">
              <span className={`h-2 w-2 rounded-full ${statusDot}`} />
            </span>
          </Link>
        </SidebarMenuButton>
        <CollapsibleTrigger asChild>
          <SidebarMenuAction type="button" aria-label={`Toggle ${connection.name}`}>
            <ChevronRight className="transition-transform duration-200 group-data-[state=open]/connection-collapsible:rotate-90" />
          </SidebarMenuAction>
        </CollapsibleTrigger>
        <CollapsibleContent>
          <SidebarMenuSub>
            {connectionStatus === Connection.status.ERROR ? (
              <DisabledSubButton>
                {connection.status_error ?? "Unable to connect to this database"}
              </DisabledSubButton>
            ) : null}
            {connectionStatus === "unknown" ? (
              <DisabledSubButton>Status unavailable</DisabledSubButton>
            ) : null}
            {connectionStatus === Connection.status.OK ? (
              <ConnectionDatabases
                databaseState={node.databaseState}
                connectionId={connection.id}
                databases={node.databases}
                activeState={activeState}
              />
            ) : null}
          </SidebarMenuSub>
        </CollapsibleContent>
      </SidebarMenuItem>
    </Collapsible>
  );
};
