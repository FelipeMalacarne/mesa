/** Number, size and time formatting that follows the design system's content rules. */
import type { Engine, Environment } from "./api/mesaAPI.schemas";

/** 1284 → "1,284" */
export function thousands(n: number): string {
  return n.toLocaleString("en-US");
}

/** 1284 → "1.3K", 48000 → "48K", 2_400_000 → "2.4M" */
export function compact(n: number): string {
  const abs = Math.abs(n);
  if (abs < 1000) return String(n);
  const units: [number, string][] = [
    [1e12, "T"],
    [1e9, "B"],
    [1e6, "M"],
    [1e3, "K"],
  ];
  for (const [div, suffix] of units) {
    if (abs >= div) {
      const v = n / div;
      const text = v >= 100 ? v.toFixed(0) : v.toFixed(1).replace(/\.0$/, "");
      return `${text}${suffix}`;
    }
  }
  return String(n);
}

/** 319488 → "312 kB" (1024-based, as PostgreSQL prints sizes) */
export function bytes(n: number | undefined): string {
  if (n === undefined || n === null) return "";
  if (n < 1024) return `${n} B`;
  const units = ["kB", "MB", "GB", "TB", "PB"];
  let v = n / 1024;
  let i = 0;
  while (v >= 1024 && i < units.length - 1) {
    v /= 1024;
    i++;
  }
  const text = v >= 100 ? v.toFixed(0) : v >= 10 ? v.toFixed(1).replace(/\.0$/, "") : v.toFixed(1);
  return `${text} ${units[i]}`;
}

/** Milliseconds as "42 ms", "1.2 s" */
export function duration(ms: number): string {
  if (ms < 1) return "<1 ms";
  if (ms < 1000) return `${Math.round(ms)} ms`;
  return `${(ms / 1000).toFixed(1).replace(/\.0$/, "")} s`;
}

export function nsToMs(ns: number): number {
  return ns / 1e6;
}

/** "2 h ago", "Yesterday", "3 d ago", "Just now", "Never" */
export function relativeTime(iso: string | undefined, now: number = Date.now()): string {
  if (!iso) return "Never";
  const t = Date.parse(iso);
  if (Number.isNaN(t)) return iso;
  const s = Math.max(0, Math.round((now - t) / 1000));
  if (s < 60) return "Just now";
  const m = Math.floor(s / 60);
  if (m < 60) return `${m} min ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h} h ago`;
  const d = Math.floor(h / 24);
  if (d === 1) return "Yesterday";
  if (d < 30) return `${d} d ago`;
  return new Date(t).toISOString().slice(0, 10);
}

/** "14:02" in the viewer's local time */
export function clockTime(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

export function plural(n: number, one: string, many = `${one}s`): string {
  return `${thousands(n)} ${n === 1 ? one : many}`;
}

export const ENVIRONMENTS: readonly Environment[] = ["local", "development", "staging", "production"];

export function envLabel(env: Environment | undefined): string {
  switch (env) {
    case "development":
      return "Development";
    case "staging":
      return "Staging";
    case "production":
      return "Production";
    case "local":
    default:
      return "Local";
  }
}

export function engineLabel(engine: Engine): string {
  return engine === "mysql" ? "MySQL" : "PostgreSQL";
}

export function engineShort(engine: Engine): string {
  return engine === "mysql" ? "mysql" : "pg";
}

export function defaultPort(engine: Engine): number {
  return engine === "mysql" ? 3306 : 5432;
}

/** `{ Schema, Name }` → "schema.name" */
export function qualifiedName(schema: string, name: string): string {
  return `${schema}.${name}`;
}
