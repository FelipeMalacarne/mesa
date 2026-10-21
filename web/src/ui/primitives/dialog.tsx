/**
 * Dialog primitive (Radix) dressed in the design system's `.m-dialog` classes.
 * Kept from shadcn's dialog.tsx with the Tailwind class soup replaced by `.m-*` classes.
 */
import * as React from "react";
import { Dialog as DialogPrimitive } from "radix-ui";

import { cn } from "@/lib/utils";

const Dialog = DialogPrimitive.Root;
const DialogTrigger = DialogPrimitive.Trigger;
const DialogClose = DialogPrimitive.Close;

type ContentProps = React.ComponentProps<typeof DialogPrimitive.Content> & {
  /** Max width in px; the design uses 560 by default, 600 for the connection form, 640 for review. */
  width?: number;
};

function DialogContent({ className, children, width, style, ...props }: ContentProps) {
  return (
    <DialogPrimitive.Portal>
      <DialogPrimitive.Overlay className="m-overlay" />
      <DialogPrimitive.Content
        className={cn("m-dialog m-dialog--fixed", className)}
        style={width ? { ...style, maxWidth: width } : style}
        {...props}
      >
        {children}
      </DialogPrimitive.Content>
    </DialogPrimitive.Portal>
  );
}

function DialogHead({ className, ...props }: React.ComponentProps<"div">) {
  return <div className={cn("m-dialog__head", className)} {...props} />;
}

function DialogTitle({ className, ...props }: React.ComponentProps<typeof DialogPrimitive.Title>) {
  return <DialogPrimitive.Title className={cn("m-dialog__title", className)} {...props} />;
}

function DialogDescription({ className, ...props }: React.ComponentProps<typeof DialogPrimitive.Description>) {
  return <DialogPrimitive.Description className={cn("m-dialog__desc", className)} {...props} />;
}

function DialogBody({ className, ...props }: React.ComponentProps<"div">) {
  return <div className={cn("m-dialog__body", className)} {...props} />;
}

function DialogFoot({ className, ...props }: React.ComponentProps<"div">) {
  return <div className={cn("m-dialog__foot", className)} {...props} />;
}

export { Dialog, DialogTrigger, DialogClose, DialogContent, DialogHead, DialogTitle, DialogDescription, DialogBody, DialogFoot };
