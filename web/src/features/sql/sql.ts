/** Small SQL helpers for the editor: statement at the cursor, write detection, formatting. */
import { tokenize } from "@/ui/Code";

const WRITE_START = new Set([
  "insert", "update", "delete", "merge", "create", "alter", "drop", "truncate", "grant", "revoke", "comment", "copy",
  "refresh", "call", "do", "lock", "reindex", "cluster", "vacuum", "analyze", "security", "import", "rename",
]);

/** Splits on top-level semicolons, keeping quotes, comments and dollar bodies intact. */
export function statements(sql: string): string[] {
  const out: string[] = [];
  let buf = "";
  let i = 0;
  while (i < sql.length) {
    const c = sql[i]!;
    const rest = sql.slice(i);
    let m: RegExpMatchArray | null;
    if (c === "'" || c === '"') {
      let j = i + 1;
      while (j < sql.length) {
        if (sql[j] === c) {
          if (sql[j + 1] === c) j += 2;
          else break;
        } else j++;
      }
      buf += sql.slice(i, j + 1);
      i = j + 1;
    } else if (rest.startsWith("--")) {
      const j = sql.indexOf("\n", i);
      const end = j < 0 ? sql.length : j;
      buf += sql.slice(i, end);
      i = end;
    } else if (rest.startsWith("/*")) {
      const j = sql.indexOf("*/", i + 2);
      const end = j < 0 ? sql.length : j + 2;
      buf += sql.slice(i, end);
      i = end;
    } else if ((m = rest.match(/^\$[A-Za-z_]?[A-Za-z0-9_]*\$/))) {
      const tag = m[0];
      const j = sql.indexOf(tag, i + tag.length);
      const end = j < 0 ? sql.length : j + tag.length;
      buf += sql.slice(i, end);
      i = end;
    } else if (c === ";") {
      if (stripComments(buf).trim()) out.push(buf.trim());
      buf = "";
      i++;
    } else {
      buf += c;
      i++;
    }
  }
  if (stripComments(buf).trim()) out.push(buf.trim());
  return out;
}

export function stripComments(sql: string): string {
  return sql.replace(/--[^\n]*/g, " ").replace(/\/\*[\s\S]*?\*\//g, " ");
}

/** True when a statement changes data, structure or permissions. */
export function isWrite(statement: string): boolean {
  // Literals cannot make a statement a write; blank them so their words do not match.
  const text = stripComments(statement).replace(/'(?:[^']|'')*'/g, "''").trim().toLowerCase();
  const first = /^[a-z]+/.exec(text)?.[0] ?? "";
  if (WRITE_START.has(first)) return true;
  if (first === "with") return /\b(insert|update|delete|merge)\b/.test(text);
  // SELECT … INTO new_table FROM … creates a table.
  if (first === "select") return /^select\b[\s\S]*?\binto\s+(?!strict\b)[a-z_"][\w."]*\s+from\b/.test(text);
  return false;
}

/** The statement the cursor sits in (or right after). */
export function statementAt(sql: string, cursor: number): string {
  let start = 0;
  for (const s of statements(sql)) {
    const at = sql.indexOf(s, start);
    const end = at + s.length;
    if (cursor <= end + 1) return s;
    start = end;
  }
  return sql.trim();
}

const BREAK_BEFORE = /^(select|from|where|group|order|having|limit|offset|union|returning|values|set|on|left|right|inner|full|cross|join|insert|update|delete|with)$/i;

/** Upper-cases keywords and starts the main clauses on their own lines. */
export function formatSql(sql: string): string {
  return statements(sql)
    .map((stmt) => {
      let out = "";
      let prevWord = "";
      for (const t of tokenize(stmt, "sql")) {
        if (t.kind === "k") {
          const upper = t.text.toUpperCase();
          const joinPart = /^(left|right|inner|full|cross)$/i.test(prevWord) && /^(join|outer)$/i.test(t.text);
          const byPart = /^(group|order|partition)$/i.test(prevWord) && /^by$/i.test(t.text);
          if (BREAK_BEFORE.test(t.text) && out.trim() && !joinPart && !byPart && !/^on$/i.test(t.text)) out = out.replace(/\s+$/, "") + "\n";
          out += upper;
          prevWord = t.text;
        } else {
          if (t.kind === "t" && /^\s+$/.test(t.text)) {
            out += out.endsWith("\n") ? "" : " ";
          } else {
            out += t.text;
            if (t.text.trim()) prevWord = t.text;
          }
        }
      }
      return out.trim() + ";";
    })
    .join("\n\n");
}

/** CSV with RFC 4180 quoting. */
export function toCsv(header: string[], rows: string[][]): string {
  const q = (s: string) => (/[",\n\r]/.test(s) ? `"${s.replace(/"/g, '""')}"` : s);
  return [header.map(q).join(","), ...rows.map((r) => r.map(q).join(","))].join("\n");
}
