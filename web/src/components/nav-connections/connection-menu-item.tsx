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

import {
  Connection,
  ListDatabasesResponse,
} from "@/api";
import type {
  ConnectionStatus,
  FetchResult,
  TablesByDatabaseKey,
} from "./types";
import { ConnectionDatabases } from "./connection-databases";
import { DisabledSubButton } from "./disabled-sub-button";

export type ConnectionMenuItemProps = {
  connection: Connection;
  isOpen: boolean;
  onToggle: (nextOpen: boolean) => void;
  databaseState?: FetchResult<ListDatabasesResponse>;
  openDatabases: Record<string, boolean>;
  onDatabaseToggle: (key: string, nextOpen: boolean) => void;
  tablesByDatabaseKey: TablesByDatabaseKey;
};

export const ConnectionMenuItem = ({
  connection,
  isOpen,
  onToggle,
  databaseState,
  openDatabases,
  onDatabaseToggle,
  tablesByDatabaseKey,
}: ConnectionMenuItemProps) => {
  const connectionStatus: ConnectionStatus = connection.status ?? "unknown";
  const statusDot =
    connectionStatus === Connection.status.OK
      ? "bg-emerald-500"
      : connectionStatus === Connection.status.ERROR
        ? "bg-rose-500"
        : "bg-muted-foreground/40";

  return (
    <Collapsible
      asChild
      open={isOpen}
      onOpenChange={onToggle}
      className="group/connection-collapsible"
    >
      <SidebarMenuItem>
        <SidebarMenuButton asChild tooltip={connection.name} isActive={isOpen}>
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
                connection={connection}
                databaseState={databaseState}
                openDatabases={openDatabases}
                onDatabaseToggle={onDatabaseToggle}
                tablesByDatabaseKey={tablesByDatabaseKey}
              />
            ) : null}
          </SidebarMenuSub>
        </CollapsibleContent>
      </SidebarMenuItem>
    </Collapsible>
  );
};
