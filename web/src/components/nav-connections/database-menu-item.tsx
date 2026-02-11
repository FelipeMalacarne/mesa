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

import type { ListTablesResponse } from "@/api";
import type { DatabaseEntry, FetchResult } from "./types";
import { DisabledSubButton } from "./disabled-sub-button";
import { TableMenuItem } from "./table-menu-item";
import { Spinner } from "../ui/spinner";

export type DatabaseMenuItemProps = {
  connectionId: string;
  database: DatabaseEntry;
  isOpen: boolean;
  onToggle: (nextOpen: boolean) => void;
  tableState?: FetchResult<ListTablesResponse>;
};

export const DatabaseMenuItem = ({
  connectionId,
  database,
  isOpen,
  onToggle,
  tableState,
}: DatabaseMenuItemProps) => {
  const tableStatus = tableState?.status ?? "idle";
  const tables = tableState?.data?.tables ?? [];

  return (
    <Collapsible
      asChild
      open={isOpen}
      onOpenChange={onToggle}
      className="group/database-collapsible"
    >
      <SidebarMenuSubItem>
        <SidebarMenuSubButton
          asChild
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
                  />
                ))
              : null}
          </SidebarMenuSub>
        </CollapsibleContent>
      </SidebarMenuSubItem>
    </Collapsible>
  );
};
