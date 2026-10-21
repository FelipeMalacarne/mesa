/**
 * Helpers for the Value wire form (`{ k, v, t }`, see the Value schema in api/openapi.yaml).
 *
 * - `textOf(value)` renders a value for the grid and inspector (NULL, \x… bytes, raw JSON…).
 * - `parseInput(kind, type, text)` turns what a person typed into a Value for a ChangeSet.
 * - `kindOfType(type)` guesses the kind from an engine type name (for new rows and filters).
 * - `isNumeric(kind)` tells which columns align right.
 */
import type { Value, ValueK } from "./api/mesaAPI.schemas";

export type { Value };
export type ValueKind = ValueK;

export const NULL_VALUE: Value = { k: "null" };

/** The text a person types to set a cell to NULL. */
export const NULL_INPUT = "NULL";

export class ValueInputError extends Error {
  constructor(message: string) {
    super(message);
    this.name = "ValueInputError";
  }
}

export function isNull(value: Value | null | undefined): boolean {
  return !value || value.k === "null" || value.v === undefined || value.v === null;
}

export function isNumeric(kind: string | undefined): boolean {
  return kind === "int" || kind === "float" || kind === "decimal";
}

/** Two values are equal when their kind and payload are equal (the engine type is ignored). */
export function valuesEqual(a: Value | undefined, b: Value | undefined): boolean {
  if (isNull(a) && isNull(b)) return true;
  if (isNull(a) || isNull(b)) return false;
  return a!.k === b!.k && JSON.stringify(a!.v) === JSON.stringify(b!.v);
}

// ---------------------------------------------------------------------------
// Display

export function textOf(value: Value | null | undefined): string {
  if (isNull(value)) return NULL_INPUT;
  const v = value!.v;
  switch (value!.k) {
    case "bool":
      return v === true ? "true" : v === false ? "false" : String(v);
    case "int":
    case "float":
    case "decimal":
      return typeof v === "number" ? numberText(v) : String(v);
    case "bytes":
      return typeof v === "string" ? `\\x${base64ToHex(v)}` : String(v);
    case "array":
      return Array.isArray(v) ? `{${v.map((el) => arrayElementText(el as Value)).join(",")}}` : String(v);
    case "time":
      return typeof v === "string" ? timeText(v) : String(v);
    case "text":
    case "uuid":
    case "json":
    case "other":
    default:
      return typeof v === "string" ? v : typeof v === "object" ? JSON.stringify(v) : String(v);
  }
}

/** "2026-10-21T13:01:58.1167Z" → "2026-10-21 13:01:58.1167+00", as PostgreSQL prints it. */
function timeText(iso: string): string {
  const m = /^(\d{4}-\d{2}-\d{2})T(\d{2}:\d{2}:\d{2}(?:\.\d+)?)(Z|[+-]\d{2}:?\d{2})?$/.exec(iso);
  if (!m) return iso;
  const zone = m[3] === "Z" ? "+00" : m[3] ? m[3].replace(/:00$/, "") : "";
  return `${m[1]} ${m[2]}${zone}`;
}

function numberText(n: number): string {
  if (Number.isNaN(n)) return "NaN";
  if (n === Infinity) return "Inf";
  if (n === -Infinity) return "-Inf";
  return String(n);
}

function arrayElementText(el: Value): string {
  if (isNull(el)) return "NULL";
  const text = textOf(el);
  if (el.k === "text" || el.k === "other" || el.k === "json" || el.k === "time") {
    if (text === "" || /[\s,"{}\\]/.test(text) || text.toUpperCase() === "NULL") {
      return `"${text.replace(/\\/g, "\\\\").replace(/"/g, '\\"')}"`;
    }
  }
  return text;
}

// ---------------------------------------------------------------------------
// Parsing what the person typed

export function parseInput(kind: string, type: string, text: string): Value {
  const k = resolveKind(kind, type);
  if (text === NULL_INPUT) return { k: "null", t: type };
  const t = type || undefined;
  switch (k) {
    case "bool": {
      if (/^(t|true|y|yes|1|on)$/i.test(text.trim())) return withType({ k: "bool", v: true }, t);
      if (/^(f|false|n|no|0|off)$/i.test(text.trim())) return withType({ k: "bool", v: false }, t);
      throw new ValueInputError("Enter true or false");
    }
    case "int": {
      const s = text.trim().replace(/^\+/, "");
      if (!/^-?\d+$/.test(s)) throw new ValueInputError("Enter a whole number");
      const n = Number(s);
      return withType({ k: "int", v: Number.isSafeInteger(n) ? n : s }, t);
    }
    case "float": {
      const s = text.trim();
      if (/^nan$/i.test(s)) return withType({ k: "float", v: "NaN" }, t);
      if (/^[+-]?inf(inity)?$/i.test(s)) return withType({ k: "float", v: s.startsWith("-") ? "-Inf" : "Inf" }, t);
      const n = Number(s);
      if (s === "" || !Number.isFinite(n)) throw new ValueInputError("Enter a number");
      return withType({ k: "float", v: n }, t);
    }
    case "decimal": {
      const s = text.trim();
      if (!/^[+-]?(\d+\.?\d*|\.\d+)([eE][+-]?\d+)?$/.test(s)) throw new ValueInputError("Enter a decimal number");
      return withType({ k: "decimal", v: s }, t);
    }
    case "uuid": {
      const s = text.trim().toLowerCase();
      if (!/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(s)) {
        throw new ValueInputError("Enter a UUID such as 123e4567-e89b-12d3-a456-426614174000");
      }
      return withType({ k: "uuid", v: s }, t);
    }
    case "json": {
      const s = text.trim();
      try {
        JSON.parse(s);
      } catch {
        throw new ValueInputError("Enter valid JSON");
      }
      return withType({ k: "json", v: s }, t);
    }
    case "bytes": {
      const m = /^\\x([0-9a-f]*)$/i.exec(text.trim());
      const b64 = m ? hexToBase64(m[1]!) : utf8ToBase64(text);
      return withType({ k: "bytes", v: b64 }, t);
    }
    case "time": {
      const s = text.trim();
      if (!s) throw new ValueInputError("Enter a date or time, or NULL");
      return withType({ k: "time", v: s }, t);
    }
    case "array": {
      const elementType = elementTypeOf(type);
      const elementKind = kindOfType(elementType);
      const parts = splitArrayLiteral(text.trim());
      const items = parts.map((p) => (p === null ? ({ k: "null" } as Value) : parseInput(elementKind, elementType, p)));
      return withType({ k: "array", v: items }, t);
    }
    case "text":
    case "other":
    default:
      return withType({ k: k === "text" ? "text" : "other", v: text }, t);
  }
}

function withType(v: Value, t: string | undefined): Value {
  return t ? { ...v, t } : v;
}

function resolveKind(kind: string, type: string): ValueKind {
  if (kind && kind !== "null" && isKind(kind)) return kind;
  return kindOfType(type);
}

const KINDS: ReadonlySet<string> = new Set([
  "null", "bool", "int", "float", "decimal", "text", "bytes", "uuid", "json", "time", "array", "other",
]);

function isKind(k: string): k is ValueKind {
  return KINDS.has(k);
}

/** Guess the value kind from an engine type name (`int8`, `character varying(80)`, `_text`, `jsonb`…). */
export function kindOfType(type: string | undefined): ValueKind {
  if (!type) return "other";
  let t = type.trim().toLowerCase().replace(/\(.*\)/, "").trim();
  if (t.endsWith("[]") || t.startsWith("_")) return "array";
  t = t.replace(/\s+/g, " ");
  if (/^(int2|int4|int8|smallint|integer|int|bigint|serial|serial2|serial4|serial8|smallserial|bigserial|oid|tinyint|mediumint)$/.test(t)) return "int";
  if (/^(float4|float8|real|double precision|double|float)$/.test(t)) return "float";
  if (/^(numeric|decimal|money)$/.test(t)) return "decimal";
  if (/^(bool|boolean)$/.test(t)) return "bool";
  if (/^(text|varchar|character varying|char|character|bpchar|name|citext|tinytext|mediumtext|longtext|enum)$/.test(t)) return "text";
  if (/^(bytea|blob|binary|varbinary|tinyblob|mediumblob|longblob)$/.test(t)) return "bytes";
  if (t === "uuid") return "uuid";
  if (/^(json|jsonb)$/.test(t)) return "json";
  if (/^(timestamp|timestamptz|timestamp with time zone|timestamp without time zone|date|time|timetz|time with time zone|time without time zone|interval|datetime|year)$/.test(t)) return "time";
  return "other";
}

function elementTypeOf(type: string): string {
  const t = type.trim();
  if (t.endsWith("[]")) return t.slice(0, -2);
  if (t.startsWith("_")) return t.slice(1);
  return t;
}

/** Splits `{a,"b c",NULL}` or a JSON `[...]` into element texts; `null` marks a NULL element. */
function splitArrayLiteral(text: string): (string | null)[] {
  if (text.startsWith("[")) {
    let parsed: unknown;
    try {
      parsed = JSON.parse(text);
    } catch {
      throw new ValueInputError("Enter an array such as {a,b} or [1, 2]");
    }
    if (!Array.isArray(parsed)) throw new ValueInputError("Enter an array such as {a,b} or [1, 2]");
    return parsed.map((el) => (el === null ? null : typeof el === "string" ? el : JSON.stringify(el)));
  }
  if (!text.startsWith("{") || !text.endsWith("}")) throw new ValueInputError("Enter an array such as {a,b} or [1, 2]");
  const inner = text.slice(1, -1);
  if (inner.trim() === "") return [];
  const out: (string | null)[] = [];
  let cur = "";
  let quoted = false;
  let wasQuoted = false;
  for (let i = 0; i < inner.length; i++) {
    const c = inner[i]!;
    if (quoted) {
      if (c === "\\" && i + 1 < inner.length) {
        cur += inner[++i];
      } else if (c === '"') {
        quoted = false;
      } else {
        cur += c;
      }
    } else if (c === '"') {
      quoted = true;
      wasQuoted = true;
    } else if (c === ",") {
      out.push(wasQuoted ? cur : cur.trim().toUpperCase() === "NULL" ? null : cur.trim());
      cur = "";
      wasQuoted = false;
    } else {
      cur += c;
    }
  }
  out.push(wasQuoted ? cur : cur.trim().toUpperCase() === "NULL" ? null : cur.trim());
  return out;
}

// ---------------------------------------------------------------------------
// Bytes helpers (base64 on the wire, \x hex on screen)

function base64ToHex(b64: string): string {
  const bin = atob(b64);
  let hex = "";
  for (let i = 0; i < bin.length; i++) hex += bin.charCodeAt(i).toString(16).padStart(2, "0");
  return hex;
}

function hexToBase64(hex: string): string {
  if (hex.length % 2 !== 0) throw new ValueInputError("Hex bytes need an even number of digits");
  let bin = "";
  for (let i = 0; i < hex.length; i += 2) bin += String.fromCharCode(parseInt(hex.slice(i, i + 2), 16));
  return btoa(bin);
}

function utf8ToBase64(text: string): string {
  const bytes = new TextEncoder().encode(text);
  let bin = "";
  for (const b of bytes) bin += String.fromCharCode(b);
  return btoa(bin);
}
