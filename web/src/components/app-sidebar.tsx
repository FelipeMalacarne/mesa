import { Layers } from "lucide-react";
import Header from "./app-header";
import {
  Breadcrumb,
  BreadcrumbItem,
  BreadcrumbLink,
  BreadcrumbList,
  BreadcrumbPage,
  BreadcrumbSeparator,
} from "./ui/breadcrumb";
import { Separator } from "./ui/separator";
import {
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarHeader,
  SidebarInset,
  SidebarProvider,
  SidebarTrigger,
} from "./ui/sidebar";

export function AppSidebar({ children }: { children: React.ReactNode }) {
  return (
    <SidebarProvider>
      <Sidebar variant="inset">
        <SidebarHeader>
          <div className="flex h-14 items-center gap-3 border-b border-sidebar-border">
            <div className="flex h-9 w-9 items-center justify-center rounded-md bg-foreground">
              <Layers className="h-4.5 w-4.5 text-background" />
            </div>
            <span className="text-base font-semibold text-foreground tracking-tight">
              Mesa
            </span>
          </div>
          {/* <Layers className="h-4 w-4 text-primary-foreground" /> */}
          {/* <span className="font-semibold text-foreground">Mesa</span> */}
        </SidebarHeader>
        <SidebarContent>
          sas
          <SidebarGroup />
          <SidebarGroup />
        </SidebarContent>
      </Sidebar>
      <SidebarInset>
        <Header />
        <main>{children}</main>
      </SidebarInset>
    </SidebarProvider>
  );
}
