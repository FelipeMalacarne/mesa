/**
 * Small presentational controls over the design system's `.m-*` classes:
 * environment tag, status dot, kbd, native-backed select, switch, choice, field, empty state.
 */
import * as React from "react";
import { ChevronsUpDown } from "lucide-react";

import type { Environment } from "@/lib/api/mesaAPI.schemas";
import { envLabel } from "@/lib/format";
import { cn } from "@/lib/utils";

export function EnvTag({ env, className }: { env: Environment; className?: string }) {
  return <span className={cn("m-env", `m-env--${env}`, className)}>{envLabel(env)}</span>;
}

export type DotStatus = "online" | "degraded" | "down" | "unknown" | "pending";

export function StatusDot({ status, className }: { status: DotStatus; className?: string }) {
  return <span className={cn("m-dot", `m-dot--${status}`, className)} aria-hidden="true" />;
}

export function Kbd({ children, className }: { children: React.ReactNode; className?: string }) {
  return <kbd className={cn("m-kbd", className)}>{children}</kbd>;
}

export interface SelectOption<T extends string> {
  value: T;
  label: string;
}

interface SelectProps<T extends string> {
  value: T;
  options: readonly SelectOption<T>[];
  onChange: (value: T) => void;
  "aria-label": string;
  size?: "sm" | "md";
  mono?: boolean;
  disabled?: boolean;
  className?: string;
  style?: React.CSSProperties;
}

/** A styled select: the design's `.m-select` look with a real `<select>` on top for keyboard and a11y. */
export function Select<T extends string>({ value, options, onChange, size = "md", mono, disabled, className, style, ...rest }: SelectProps<T>) {
  const current = options.find((o) => o.value === value);
  return (
    <span className={cn("m-select", size === "sm" && "m-select--sm", disabled && "is-disabled", className)} style={style}>
      <span className={cn("m-select__label", mono && "m-mono")}>{current?.label ?? value}</span>
      <ChevronsUpDown className="m-icon m-icon--sm" aria-hidden="true" />
      <select
        aria-label={rest["aria-label"]}
        value={value}
        disabled={disabled}
        onChange={(e) => onChange(e.target.value as T)}
      >
        {options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
    </span>
  );
}

interface SwitchProps {
  checked: boolean;
  onChange: (checked: boolean) => void;
  disabled?: boolean;
  "aria-label": string;
}

export function Switch({ checked, onChange, disabled, ...rest }: SwitchProps) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={rest["aria-label"]}
      disabled={disabled}
      className={cn("m-switch", checked && "is-on", disabled && "opacity-50 cursor-not-allowed")}
      onClick={() => onChange(!checked)}
    />
  );
}

interface ChoiceProps extends Omit<React.ComponentProps<"button">, "type"> {
  selected: boolean;
}

/** A card-like radio option (`.m-choice`), used for engine and environment pickers. */
export function Choice({ selected, disabled, className, children, ...props }: ChoiceProps) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={selected}
      aria-disabled={disabled || undefined}
      disabled={disabled}
      className={cn("m-choice", selected && "is-selected", className)}
      {...props}
    >
      {children}
    </button>
  );
}

interface FieldProps {
  label: React.ReactNode;
  help?: React.ReactNode;
  error?: string;
  htmlFor?: string;
  className?: string;
  children: React.ReactNode;
}

export function Field({ label, help, error, htmlFor, className, children }: FieldProps) {
  return (
    <div className={cn("m-field", className)}>
      <label className="m-label" htmlFor={htmlFor}>
        {label}
      </label>
      {children}
      {error ? <span className="m-help m-help--error">{error}</span> : help ? <span className="m-help">{help}</span> : null}
    </div>
  );
}

interface EmptyStateProps {
  icon?: React.ReactNode;
  title: string;
  description?: React.ReactNode;
  actions?: React.ReactNode;
  className?: string;
}

export function EmptyState({ icon, title, description, actions, className }: EmptyStateProps) {
  return (
    <div className={cn("m-empty", className)}>
      {icon ? <div className="m-empty__media">{icon}</div> : null}
      <h2 className="m-empty__title">{title}</h2>
      {description ? <p className="m-empty__desc">{description}</p> : null}
      {actions ? <div className="m-empty__actions">{actions}</div> : null}
    </div>
  );
}

/** A status dot with its word and optional mono meta, e.g. "● Online 12 ms". */
export function StatusLabel({ status, label, meta, className }: { status: DotStatus; label: string; meta?: string; className?: string }) {
  return (
    <span className={cn("m-status", className)}>
      <StatusDot status={status} />
      {label}
      {meta ? <span className="m-status__meta">{meta}</span> : null}
    </span>
  );
}
