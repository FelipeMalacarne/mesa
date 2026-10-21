/**
 * Command palette primitives (cmdk) inside a Dialog, styled through `.m-palette` in mesa.css.
 * Kept from shadcn's command.tsx; the Tailwind selectors moved to CSS so the palette looks like
 * every other popover in the system.
 */
import * as React from "react";
import { Command as CommandPrimitive } from "cmdk";
import { Search } from "lucide-react";

import { cn } from "@/lib/utils";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "./dialog";

const Command = CommandPrimitive;
const CommandList = CommandPrimitive.List;
const CommandEmpty = CommandPrimitive.Empty;
const CommandGroup = CommandPrimitive.Group;
const CommandItem = CommandPrimitive.Item;
const CommandSeparator = CommandPrimitive.Separator;

function CommandDialog({
  title = "Command palette",
  description = "Search connections, open tabs and actions",
  children,
  className,
  ...props
}: React.ComponentProps<typeof Dialog> & { title?: string; description?: string; className?: string }) {
  return (
    <Dialog {...props}>
      <DialogContent className={cn("m-palette", className)} aria-describedby={undefined}>
        <DialogTitle className="sr-only">{title}</DialogTitle>
        <DialogDescription className="sr-only">{description}</DialogDescription>
        <CommandPrimitive loop>{children}</CommandPrimitive>
      </DialogContent>
    </Dialog>
  );
}

function CommandInput({ className, ...props }: React.ComponentProps<typeof CommandPrimitive.Input>) {
  return (
    <div className="flex items-center" cmdk-input-wrapper="">
      <Search className="m-icon m-icon--sm m-muted ml-4 -mr-3" aria-hidden="true" />
      <CommandPrimitive.Input className={cn("pl-4", className)} {...props} />
    </div>
  );
}

export { Command, CommandDialog, CommandInput, CommandList, CommandEmpty, CommandGroup, CommandItem, CommandSeparator };
