/** The design system's `.m-btn` as a component: variant, size and icon-only. */
import * as React from "react";

import { cn } from "@/lib/utils";

export type ButtonVariant = "primary" | "secondary" | "outline" | "ghost" | "destructive";
export type ButtonSize = "xs" | "sm" | "md" | "lg";

export interface ButtonProps extends React.ComponentProps<"button"> {
  variant?: ButtonVariant;
  size?: ButtonSize;
  icon?: boolean;
}

export function buttonClass({ variant = "outline", size = "md", icon = false }: Pick<ButtonProps, "variant" | "size" | "icon">, className?: string) {
  return cn("m-btn", `m-btn--${variant}`, size !== "md" && `m-btn--${size}`, icon && "m-btn--icon", className);
}

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  { variant, size, icon, className, type = "button", ...props },
  ref,
) {
  return <button ref={ref} type={type} className={buttonClass({ variant, size, icon }, className)} {...props} />;
});
