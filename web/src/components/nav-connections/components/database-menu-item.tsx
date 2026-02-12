import { ChevronRight, Database } from "lucide-react";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import {
  SidebarMenuAction,
  SidebarMenuSub,
  SidebarMenuSubButton,
  SidebarMenuSubItem,
} from "@/components/ui/sidebar";
import { Link } from "@tanstack/react-router";

import type { DatabaseTreeNode } from "../state/types";
import { DisabledSubButton } from "./disabled-sub-button";
import { TableMenuItem } from "./table-menu-item";
import { Spinner } from "../../ui/spinner";
import { useNavConnectionsState } from "../state/context";

export type DatabaseMenuItemProps = {
  connectionId: string;
  node: DatabaseTreeNode;
  activeState: {
    connectionId?: string;
    databaseName?: string;
    tableName?: string;
  };
};

export const DatabaseMenuItem = ({
  connectionId,
  node,
  activeState,
}: DatabaseMenuItemProps) => {
  const { setDatabaseOpen } = useNavConnectionsState();
  const { database, isOpen, tableState, key } = node;
  const tableStatus = tableState?.status ?? "idle";
  const tables = tableState?.data?.tables ?? [];
  const isDatabaseActive =
    activeState.connectionId === connectionId &&
    activeState.databaseName === database.name &&
    !activeState.tableName;

  return (
    <Collapsible
      asChild
      open={isOpen}
      onOpenChange={(nextOpen) => setDatabaseOpen(key, nextOpen)}
      className="group/database-collapsible"
    >
      <SidebarMenuSubItem>
        <SidebarMenuSubButton
          asChild
          isActive={isDatabaseActive}
          className="pr-8 [&>svg]:text-sidebar-foreground"
        >
          <Link
            to="/connections/$connectionId/databases/$databaseName"
            params={{ connectionId, databaseName: database.name }}
          >
            <Database className="text-sidebar-foreground" />
            <span>{database.name}</span>
          </Link>
        </SidebarMenuSubButton>
        <CollapsibleTrigger asChild>
          <SidebarMenuAction
            aria-label={`Toggle ${database.name}`}
            className="top-1"
          >
            <ChevronRight className="transition-transform duration-200 group-data-[state=open]/database-collapsible:rotate-90" />
          </SidebarMenuAction>
        </CollapsibleTrigger>
        <CollapsibleContent>
          <SidebarMenuSub className="ml-4">
            {tableStatus === "loading" ? (
              <DisabledSubButton>
                <Spinner />
              </DisabledSubButton>
            ) : null}
            {tableStatus === "error" ? (
              <DisabledSubButton size="sm">
                {tableState?.errorMessage ?? "Unable to load tables"}
              </DisabledSubButton>
            ) : null}
            {tableStatus === "ok" && tables.length === 0 ? (
              <DisabledSubButton size="sm">No tables found</DisabledSubButton>
            ) : null}
            {tableStatus === "ok"
              ? tables.map((table) => (
                  <TableMenuItem
                    key={table.name}
                    connectionId={connectionId}
                    databaseName={database.name}
                    table={table}
                    activeState={activeState}
                  />
                ))
              : null}
          </SidebarMenuSub>
        </CollapsibleContent>
      </SidebarMenuSubItem>
    </Collapsible>
  );
};
