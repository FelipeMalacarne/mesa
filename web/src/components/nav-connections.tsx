import {
  ChevronRight,
  Database,
  HardDrive,
  Table as TableIcon,
  type LucideIcon,
} from "lucide-react";

import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import {
  SidebarGroup,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSub,
  SidebarMenuSubButton,
  SidebarMenuSubItem,
} from "@/components/ui/sidebar";
import { AddConnectionDialog } from "./add-connection-dialog";

export function NavConnections({
  items,
}: {
  items: {
    title: string;
    url?: string;
    isActive?: boolean;
    databases?: {
      title: string;
      url?: string;
      isActive?: boolean;
      tables?: {
        title: string;
        url?: string;
        isActive?: boolean;
      }[];
    }[];
  }[];
}) {
  return (
    <SidebarGroup>
      <SidebarGroupLabel className="flex items-center justify-between">
        Connections
        <AddConnectionDialog />
      </SidebarGroupLabel>
      <SidebarMenu>
        {items.map((item) => {
          const isConnectionOpen =
            item.isActive ||
            item.databases?.some(
              (database) =>
                database.isActive ||
                database.tables?.some((table) => table.isActive),
            );
          const hasDatabases = (item.databases?.length ?? 0) > 0;

          return (
            <Collapsible
              key={item.title}
              asChild
              defaultOpen={isConnectionOpen}
              className="group/connection-collapsible"
            >
              <SidebarMenuItem>
                <CollapsibleTrigger asChild>
                  <SidebarMenuButton
                    tooltip={item.title}
                    isActive={item.isActive}
                  >
                    <HardDrive />
                    <span>{item.title}</span>
                    {hasDatabases ? (
                      <ChevronRight className="ml-auto transition-transform duration-200 group-data-[state=open]/connection-collapsible:rotate-90" />
                    ) : null}
                  </SidebarMenuButton>
                </CollapsibleTrigger>
                {hasDatabases ? (
                  <CollapsibleContent>
                    <SidebarMenuSub>
                      {item.databases?.map((database) => {
                        const isDatabaseOpen =
                          database.isActive ||
                          database.tables?.some((table) => table.isActive);
                        const hasTables = (database.tables?.length ?? 0) > 0;

                        if (!hasTables) {
                          return (
                            <SidebarMenuSubItem key={database.title}>
                              <SidebarMenuSubButton
                                isActive={database.isActive}
                                href={database.url}
                                className="[&>svg]:text-sidebar-foreground"
                              >
                                <Database className="text-sidebar-foreground" />
                                <span>{database.title}</span>
                              </SidebarMenuSubButton>
                            </SidebarMenuSubItem>
                          );
                        }

                        return (
                          <Collapsible
                            key={database.title}
                            asChild
                            defaultOpen={isDatabaseOpen}
                            className="group/database-collapsible"
                          >
                            <SidebarMenuSubItem>
                              <CollapsibleTrigger asChild>
                                <SidebarMenuSubButton
                                  isActive={database.isActive}
                                  className="[&>svg]:text-sidebar-foreground"
                                >
                                  <Database className="text-sidebar-foreground" />
                                  <span>{database.title}</span>
                                  <ChevronRight className="ml-auto transition-transform duration-200 group-data-[state=open]/database-collapsible:rotate-90" />
                                </SidebarMenuSubButton>
                              </CollapsibleTrigger>
                              <CollapsibleContent>
                                <SidebarMenuSub className="ml-4">
                                  {database.tables?.map((table) => (
                                    <SidebarMenuSubItem key={table.title}>
                                      <SidebarMenuSubButton
                                        size="sm"
                                        isActive={table.isActive}
                                        href={table.url}
                                        className="[&>svg]:text-sidebar-foreground"
                                      >
                                        <TableIcon className="text-sidebar-foreground" />
                                        <span>{table.title}</span>
                                      </SidebarMenuSubButton>
                                    </SidebarMenuSubItem>
                                  ))}
                                </SidebarMenuSub>
                              </CollapsibleContent>
                            </SidebarMenuSubItem>
                          </Collapsible>
                        );
                      })}
                    </SidebarMenuSub>
                  </CollapsibleContent>
                ) : null}
              </SidebarMenuItem>
            </Collapsible>
          );
        })}
      </SidebarMenu>
    </SidebarGroup>
  );
}
