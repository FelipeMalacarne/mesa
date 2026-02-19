import { ChevronRight, HardDrive, Loader2 } from "lucide-react";
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
import { useQuery } from "@tanstack/react-query";

import { Connection, ConnectionsService } from "@/api";
import { ConnectionDatabases } from "./connection-databases";
import { DisabledSubButton } from "./disabled-sub-button";
import { useNavConnectionsState } from "../state/context";

export type ConnectionMenuItemProps = {
  connection: Connection;
  isOpen: boolean;
  activeState: {
    connectionId?: string;
    databaseName?: string;
    tableName?: string;
  };
};

export const ConnectionMenuItem = ({
  connection,
  isOpen,
  activeState,
}: ConnectionMenuItemProps) => {
  const { setConnectionOpen } = useNavConnectionsState();
  const isConnectionActive =
    activeState.connectionId === connection.id && !activeState.databaseName;

  const { isLoading, isError, error } = useQuery({
    queryKey: ["connections", connection.id, "ping"],
    queryFn: async () => {
      await ConnectionsService.pingConnection({ connectionId: connection.id });
      return true;
    },
    retry: false, // Don't retry if it fails, it's a ping
  });

  const isPingError = isError;
  const isOk = !isLoading && !isError;

  let statusDot = "bg-muted-foreground/40";
  if (isLoading) statusDot = "bg-yellow-500 animate-pulse";
  else if (isOk) statusDot = "bg-emerald-500";
  else if (isPingError) statusDot = "bg-rose-500";

  return (
    <Collapsible
      asChild
      open={isOpen}
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
              {isLoading ? (
                <Loader2 className="h-3 w-3 animate-spin text-muted-foreground" />
              ) : (
                <span className={`h-2 w-2 rounded-full ${statusDot}`} />
              )}
            </span>
          </Link>
        </SidebarMenuButton>
        <CollapsibleTrigger asChild>
          <SidebarMenuAction
            type="button"
            aria-label={`Toggle ${connection.name}`}
            disabled={!isOk}
          >
            <ChevronRight className="transition-transform duration-200 group-data-[state=open]/connection-collapsible:rotate-90" />
          </SidebarMenuAction>
        </CollapsibleTrigger>
        <CollapsibleContent>
          <SidebarMenuSub>
            {isPingError ? (
              <DisabledSubButton>
                {error?.message ?? "Unable to connect to this database"}
              </DisabledSubButton>
            ) : null}
            {isLoading ? (
              <DisabledSubButton>Connecting...</DisabledSubButton>
            ) : null}
            {isOk && isOpen ? (
              <ConnectionDatabases
                connectionId={connection.id}
                activeState={activeState}
              />
            ) : null}
          </SidebarMenuSub>
        </CollapsibleContent>
      </SidebarMenuItem>
    </Collapsible>
  );
};
