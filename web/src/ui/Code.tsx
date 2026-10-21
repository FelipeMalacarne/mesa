/**
 * Read-only code blocks with the design system's syntax colours (`.sx-*`).
 * The tokenizer is small on purpose: keywords, strings, numbers, comments and function
 * names are enough for review screens, the SQL editor overlay and DBML.
 */
import * as React from "react";

import { cn } from "@/lib/utils";

export type Language = "sql" | "dbml";

const SQL_KEYWORDS = new Set(
  `select from where and or not in is null as on join left right inner outer full cross group by order having limit offset
  insert into values update set delete returning create alter drop table column add constraint primary key foreign references
  unique index check default if exists cascade restrict type enum schema database role with without time zone begin commit rollback
  transaction distinct union all case when then else end asc desc nulls first last explain analyze using view materialized
  grant revoke to owner rename true false like ilike between generated always identity by comment truncate sequence function
  returns language replace temporary temp concurrently only lateral over partition window filter within`
    .split(/\s+/)
    .filter(Boolean),
);

const DBML_KEYWORDS = new Set(
  `table ref enum indexes note project tablegroup pk primary key unique not null increment default name type as delete update
  cascade restrict set no action true false`
    .split(/\s+/)
    .filter(Boolean),
);

export interface Token {
  kind: "k" | "s" | "n" | "c" | "f" | "m" | "t";
  text: string;
}

const TOKEN = /(--[^\n]*|\/\/[^\n]*|\/\*[\s\S]*?\*\/)|('''[\s\S]*?'''|'(?:[^']|'')*'?|"(?:[^"]|"")*"?|`[^`]*`?)|(\b\d+(?:\.\d+)?\b)|([A-Za-z_][A-Za-z0-9_$]*)(\s*\()?|(\s+|[^\sA-Za-z0-9_'"`-]+|-)/g;

/** Splits code into tokens; joining their text gives the input back. */
export function tokenize(code: string, language: Language = "sql"): Token[] {
  const keywords = language === "dbml" ? DBML_KEYWORDS : SQL_KEYWORDS;
  const out: Token[] = [];
  TOKEN.lastIndex = 0;
  let m: RegExpExecArray | null;
  let last = 0;
  while ((m = TOKEN.exec(code))) {
    if (m.index > last) out.push({ kind: "t", text: code.slice(last, m.index) });
    if (m[1]) out.push({ kind: "c", text: m[1] });
    else if (m[2]) out.push({ kind: language === "dbml" && m[2].startsWith("`") ? "f" : m[2].startsWith('"') && language === "sql" ? "t" : "s", text: m[2] });
    else if (m[3]) out.push({ kind: "n", text: m[3] });
    else if (m[4]) {
      const word = m[4];
      if (m[5]) {
        out.push({ kind: keywords.has(word.toLowerCase()) ? "k" : "f", text: word });
        out.push({ kind: "t", text: m[5] });
      } else {
        out.push({ kind: keywords.has(word.toLowerCase()) ? "k" : "t", text: word });
      }
    } else out.push({ kind: "t", text: m[0] });
    last = TOKEN.lastIndex;
    if (m[0] === "") TOKEN.lastIndex++;
  }
  if (last < code.length) out.push({ kind: "t", text: code.slice(last) });
  return out;
}

/** Renders tokens as spans; plain text stays unwrapped. */
export function Highlighted({ code, language = "sql" }: { code: string; language?: Language }) {
  const tokens = React.useMemo(() => tokenize(code, language), [code, language]);
  return (
    <>
      {tokens.map((t, i) => (t.kind === "t" ? <React.Fragment key={i}>{t.text}</React.Fragment> : <span key={i} className={`sx-${t.kind}`}>{t.text}</span>))}
    </>
  );
}

interface CodeBlockProps {
  code: string;
  language?: Language;
  className?: string;
  style?: React.CSSProperties;
  /** Line numbers (1-based) to mark as new. */
  newLines?: ReadonlySet<number>;
  /** Line number to scroll into view and highlight. */
  focusLine?: number;
}

/** A numbered, highlighted block (`.m-code`), one `.m-code__line` per line. */
export function CodeBlock({ code, language = "sql", className, style, newLines, focusLine }: CodeBlockProps) {
  const lines = code.replace(/\n$/, "").split("\n");
  const ref = React.useRef<HTMLPreElement>(null);
  React.useEffect(() => {
    if (!focusLine || !ref.current) return;
    const el = ref.current.querySelector<HTMLElement>(`[data-line="${focusLine}"]`);
    el?.scrollIntoView({ block: "nearest" });
  }, [focusLine]);
  return (
    <pre ref={ref} className={cn("m-code", className)} style={style}>
      {lines.map((line, i) => (
        <span key={i} data-line={i + 1} className={cn("m-code__line", newLines?.has(i + 1) && "is-new", focusLine === i + 1 && "m-code__block is-hl")}>
          <Highlighted code={line || " "} language={language} />
        </span>
      ))}
    </pre>
  );
}
