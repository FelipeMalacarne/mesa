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
        <SidebarHeader />
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
