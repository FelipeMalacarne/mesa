import { HardDrive, Layers, Terminal, Users } from "lucide-react";
import Header from "./app-header";
import {
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarHeader,
  SidebarInset,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
  SidebarTrigger,
} from "./ui/sidebar";
import { NavConnections } from "./nav-connections";
import { NavMain } from "./nav-main";

const data = {
  navMain: [
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
  ],
  navConnections: [
    {
      title: "Production",
      url: "#",
      icon: HardDrive,
      isActive: false,
      databases: [
        {
          title: "app_db",
          url: "#",
          isActive: false,
          tables: [
            {
              title: "users",
              url: "#",
              isActive: false,
            },
            {
              title: "sessions",
              url: "#",
            },
            {
              title: "invoices",
              url: "#",
            },
          ],
        },
        {
          title: "analytics",
          url: "#",
          tables: [
            {
              title: "events",
              url: "#",
            },
            {
              title: "funnels",
              url: "#",
            },
            {
              title: "retention",
              url: "#",
            },
          ],
        },
        {
          title: "postgres",
          url: "#",
        },
      ],
    },
    {
      title: "Development",
      url: "#",
      icon: HardDrive,
      isActive: false,
      databases: [
        {
          title: "app_db",
          url: "#",
          isActive: false,
          tables: [
            {
              title: "users",
              url: "#",
              isActive: false,
            },
          ],
        },
        {
          title: "postgres",
          url: "#",
        },
      ],
    },
  ],
};

export function AppSidebar({ children }: { children: React.ReactNode }) {
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
          <NavMain items={data.navMain} />
          <NavConnections items={data.navConnections} />
        </SidebarContent>
      </Sidebar>
      <SidebarInset>
        <Header />
        <main>{children}</main>
      </SidebarInset>
    </SidebarProvider>
  );
}
