import { Table as TableIcon } from "lucide-react";
import {
  SidebarMenuSubButton,
  SidebarMenuSubItem,
} from "@/components/ui/sidebar";
import { Link } from "@tanstack/react-router";

import type { Table } from "@/api";

export type TableMenuItemProps = {
  connectionId: string;
  databaseName: string;
  table: Table;
  activeState: {
    connectionId?: string;
    databaseName?: string;
    tableName?: string;
  };
};

export const TableMenuItem = ({
  connectionId,
  databaseName,
  table,
  activeState,
}: TableMenuItemProps) => {
  const isActive =
    activeState.connectionId === connectionId &&
    activeState.databaseName === databaseName &&
    activeState.tableName === table.name;
  return (
    <SidebarMenuSubItem>
      <SidebarMenuSubButton
        asChild
        size="sm"
        isActive={isActive}
        className="[&>svg]:text-sidebar-foreground"
      >
        <Link
          to="/connections/$connectionId/databases/$databaseName/tables/$tableName"
          params={{
            connectionId,
            databaseName,
            tableName: table.name,
          }}
        >
          <TableIcon className="text-sidebar-foreground" />
          <span>{table.name}</span>
        </Link>
      </SidebarMenuSubButton>
    </SidebarMenuSubItem>
  );
};
