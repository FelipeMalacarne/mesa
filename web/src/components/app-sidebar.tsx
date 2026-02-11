import { Layers, Terminal, Users } from "lucide-react";
import Header from "./app-header";
import {
  Sidebar,
  SidebarContent,
  SidebarHeader,
  SidebarInset,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
} from "./ui/sidebar";
import { NavConnections } from "./nav-connections";
import { NavMain } from "./nav-main";
import { useQuery } from "@tanstack/react-query";
import { ConnectionsService } from "@/api";
import {
  listDatabases,
  listTables,
  type Database,
  type Table,
} from "@/lib/inspector-api";

const navMain = [
  {
    name: "SQL Editor",
    url: "#",
    icon: Terminal,
  },
  {
    name: "Users",
    url: "#",
    icon: Users,
  },
];

const loadNavConnections = async () => {
  const connections = await ConnectionsService.listConnections();

  return Promise.all(
    connections.map(async (connection) => {
      let databases: Database[] = [];
      try {
        databases = await listDatabases(connection.id);
      } catch {
        databases = [];
      }

      const databaseItems = await Promise.all(
        databases.map(async (database) => {
          let tables: Table[] = [];
          try {
            tables = await listTables(connection.id, database.name);
          } catch {
            tables = [];
          }

          return {
            title: database.name,
            url: "#",
            tables: tables.map((table) => ({
              title: table.name,
              url: "#",
            })),
          };
        }),
      );

      return {
        title: connection.name,
        url: "#",
        databases: databaseItems,
      };
    }),
  );
};

export function AppSidebar({ children }: { children: React.ReactNode }) {
  const { data: navConnections = [] } = useQuery({
    queryKey: ["connections-tree"],
    queryFn: loadNavConnections,
  });

  return (
    <SidebarProvider>
      <Sidebar collapsible="icon" variant="inset">
        <SidebarHeader>
          <SidebarMenu>
            <SidebarMenuItem>
              <SidebarMenuButton
                asChild
                size={"lg"}
                className="data-[state=open]:bg-sidebar-accent data-[state=open]:text-sidebar-accent-foreground"
              >
                <a href="#">
                  <div className="bg-sidebar-primary text-sidebar-primary-foreground flex aspect-square size-8 items-center justify-center rounded-lg">
                    <Layers className="size-4" />
                  </div>
                  <span className="text-base font-semibold text-foreground tracking-tight">
                    Mesa
                  </span>
                </a>
              </SidebarMenuButton>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarHeader>
        <SidebarContent>
          <NavMain items={navMain} />
          <NavConnections items={navConnections} />
        </SidebarContent>
      </Sidebar>
      <SidebarInset>
        <Header />
        <main>{children}</main>
      </SidebarInset>
    </SidebarProvider>
  );
}
