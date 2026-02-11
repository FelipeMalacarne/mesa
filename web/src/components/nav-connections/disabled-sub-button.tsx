import type { ComponentPropsWithoutRef, ReactNode } from "react";

import {
  SidebarMenuSubButton,
  SidebarMenuSubItem,
} from "@/components/ui/sidebar";

type DisabledSubButtonProps = {
  children: ReactNode;
  size?: ComponentPropsWithoutRef<typeof SidebarMenuSubButton>["size"];
};

export const DisabledSubButton = ({ children, size }: DisabledSubButtonProps) => (
  <SidebarMenuSubItem>
    <SidebarMenuSubButton size={size} aria-disabled tabIndex={-1}>
      {children}
    </SidebarMenuSubButton>
  </SidebarMenuSubItem>
);
