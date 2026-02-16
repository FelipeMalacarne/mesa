import { ChevronRight, Database as DatabaseIcon } from "lucide-react";
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

import type { Database } from "@/api";
import { useNavConnectionsState } from "../state/context";
import { DatabaseTables } from "./database-tables";
import { databaseKey } from "../state/utils";

export type DatabaseMenuItemProps = {
  connectionId: string;
  database: Database;
  isOpen: boolean;
  activeState: {
    connectionId?: string;
    databaseName?: string;
    tableName?: string;
  };
};

export const DatabaseMenuItem = ({
  connectionId,
  database,
  isOpen,
  activeState,
}: DatabaseMenuItemProps) => {
  const { setDatabaseOpen } = useNavConnectionsState();
  const key = databaseKey(connectionId, database.name);
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
            <DatabaseIcon className="text-sidebar-foreground" />
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
            {isOpen && (
              <DatabaseTables
                connectionId={connectionId}
                databaseName={database.name}
                activeState={activeState}
              />
            )}
          </SidebarMenuSub>
        </CollapsibleContent>
      </SidebarMenuSubItem>
    </Collapsible>
  );
};
