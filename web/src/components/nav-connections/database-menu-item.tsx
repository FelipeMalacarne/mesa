import { ChevronRight, Database } from "lucide-react";
import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import {
  SidebarMenuSub,
  SidebarMenuSubButton,
  SidebarMenuSubItem,
} from "@/components/ui/sidebar";
import { Link } from "@tanstack/react-router";

import type { ListTablesResponse } from "@/api";
import type { DatabaseEntry, FetchResult } from "./types";
import { DisabledSubButton } from "./disabled-sub-button";
import { TableMenuItem } from "./table-menu-item";

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
          <button
            type="button"
            aria-label={`Toggle ${database.name}`}
            className="text-sidebar-foreground ring-sidebar-ring hover:bg-sidebar-accent hover:text-sidebar-accent-foreground absolute top-1 right-1 flex size-5 items-center justify-center rounded-md p-0 outline-hidden transition-transform focus-visible:ring-2 group-data-[collapsible=icon]:hidden"
          >
            <ChevronRight className="transition-transform duration-200 group-data-[state=open]/database-collapsible:rotate-90" />
          </button>
        </CollapsibleTrigger>
        <CollapsibleContent>
          <SidebarMenuSub className="ml-4">
            {tableStatus === "loading" ? (
              <DisabledSubButton size="sm">Loading tables...</DisabledSubButton>
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
