/**
 * What a column's name says about it: `created_at` is a timestamp set on insert, `is_active` a
 * boolean, `customer_id` refers to customers. Suggestions only, in the engine's own types; a
 * type the person picked is never changed.
 */
import type { ColumnType, Database } from "@/lib/api/mesaAPI.schemas";

import { tablesOf, type TableRef } from "./model";

export interface ColumnGuess {
  type?: string;
  default?: string;
  notNull?: boolean;
}

/** The spelling of the first of `names` the engine offers, with `args` in place of its own. */
function pick(types: ColumnType[], names: string[], args?: string): string | undefined {
  for (const n of names) {
    const t = types.find((x) => x.name === n);
    if (t) return args && t.args ? `${t.name}(${args})` : t.spelling;
  }
  return undefined;
}

const firstOf = (types: ColumnType[], group: string) => types.find((t) => t.group === group)?.spelling;
const integer = (types: ColumnType[]) => types.find((t) => t.integer)?.spelling;

const RULES: { test: RegExp; guess: (types: ColumnType[], name: string) => ColumnGuess }[] = [
  { test: /^id$|_id$/, guess: (t) => ({ type: integer(t) }) },
  { test: /(^|_)(uuid|guid)$/, guess: (t) => ({ type: pick(t, ["uuid"]) ?? pick(t, ["char"], "36") }) },
  { test: /^created_at$/, guess: (t) => ({ type: firstOf(t, "Date & time"), default: "CURRENT_TIMESTAMP", notNull: true }) },
  { test: /^updated_at$/, guess: (t) => ({ type: firstOf(t, "Date & time"), default: "CURRENT_TIMESTAMP" }) },
  { test: /_at$/, guess: (t) => ({ type: firstOf(t, "Date & time") }) },
  { test: /(_on|_date|^date|^birthday|^dob)$/, guess: (t) => ({ type: pick(t, ["date"]) }) },
  { test: /^(is|has|can|should|allow|use)_|_(enabled|active|visible|verified|deleted|archived)$|^(active|enabled|verified)$/, guess: (t) => ({ type: pick(t, ["boolean", "tinyint(1)"]), default: "false", notNull: true }) },
  { test: /(^|_)(price|amount|total|cost|balance|fee|salary|subtotal|tax)$/, guess: (t) => ({ type: pick(t, ["numeric", "decimal"], "12,2") }) },
  { test: /(^|_)(count|quantity|qty|position|sort_order|rank|age|attempts|retries|year)$/, guess: (t) => ({ type: pick(t, ["integer", "int"]) }) },
  { test: /(^|_)(email|e_mail)$/, guess: (t) => ({ type: pick(t, ["varchar"], "255") }) },
  { test: /(^|_)(phone|mobile|fax)$/, guess: (t) => ({ type: pick(t, ["varchar"], "32") }) },
  { test: /(^|_)(ip|ip_address)$/, guess: (t) => ({ type: pick(t, ["inet"]) ?? pick(t, ["varchar"], "45") }) },
  { test: /(^|_)(url|link|website|avatar|image|path|file)$/, guess: (t) => ({ type: pick(t, ["text"]) }) },
  { test: /(^|_)(description|notes?|body|content|bio|message|summary|comment)$/, guess: (t) => ({ type: pick(t, ["text"]) }) },
  { test: /(^|_)(data|metadata|meta|payload|settings|config|options|attributes|properties|extra)$/, guess: (t) => ({ type: pick(t, ["jsonb", "json"]) }) },
  { test: /(^|_)(name|title|slug|code|status|state|type|kind|label|username|locale|currency|country)$/, guess: (t) => ({ type: pick(t, ["varchar"], "255") }) },
];

/** A type (and maybe a default) suggested by the column's name, or nothing. */
export function guessColumn(name: string, types: ColumnType[]): ColumnGuess {
  const n = name.trim().toLowerCase();
  if (!n) return {};
  for (const r of RULES) if (r.test.test(n)) return r.guess(types, n);
  return {};
}

/**
 * The table a `<name>_id` column most likely refers to: `customer_id` → customers (or customer,
 * customeres, categories for category_id), in the same schema first. Only tables with a
 * one-column primary key qualify; `self` (the table being made) is skipped.
 */
export function guessReference(column: string, model: Database, schema: string, self?: string): (TableRef & { column: string }) | undefined {
  const m = /^(.+)_id$/i.exec(column.trim());
  if (!m) return undefined;
  const base = m[1]!.toLowerCase();
  const names = [base, `${base}s`, `${base}es`, base.replace(/y$/, "ies")];
  const fits = tablesOf(model).filter((t) => names.includes(t.table.name.toLowerCase()) && t.table.name !== self && t.table.primaryKey?.columns.length === 1);
  const best = fits.find((t) => t.schema === schema) ?? fits[0];
  return best ? { ...best, column: best.table.primaryKey!.columns[0]! } : undefined;
}
