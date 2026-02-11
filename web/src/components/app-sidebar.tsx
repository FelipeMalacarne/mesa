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

export function AppSidebar({ children }: { children: React.ReactNode }) {
  const { data: connections = [], isLoading } = useQuery({
    queryKey: ["connections"],
    queryFn: async () => (await ConnectionsService.listConnections()) ?? [],
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
          <NavConnections connections={connections} isLoading={isLoading} />
        </SidebarContent>
      </Sidebar>
      <SidebarInset>
        <Header />
        <main>{children}</main>
      </SidebarInset>
    </SidebarProvider>
  );
}
