/**
 * A plain textarea over a highlighted copy of its text: native editing, undo and IME, with the
 * design system's syntax colours. Line numbers scroll with the text.
 */
import * as React from "react";

import { Highlighted } from "@/ui/Code";

interface Props {
  value: string;
  onChange: (value: string) => void;
  onKeyDown?: (e: React.KeyboardEvent<HTMLTextAreaElement>) => void;
  textareaRef?: React.RefObject<HTMLTextAreaElement | null>;
  language?: "sql" | "dbml";
  ariaLabel: string;
  /** Lines to mark (1-based), e.g. a DBML error. */
  markLines?: ReadonlySet<number>;
  placeholder?: string;
}

const FONT: React.CSSProperties = { font: "400 13px/22px var(--font-mono)", fontVariantNumeric: "tabular-nums", tabSize: 2 };

export function Editor({ value, onChange, onKeyDown, textareaRef, language = "sql", ariaLabel, markLines, placeholder }: Props) {
  const localRef = React.useRef<HTMLTextAreaElement>(null);
  const ref = textareaRef ?? localRef;
  const preRef = React.useRef<HTMLPreElement>(null);
  const gutterRef = React.useRef<HTMLDivElement>(null);
  const lines = value.split("\n").length;

  const sync = () => {
    const ta = ref.current;
    if (!ta) return;
    if (preRef.current) {
      preRef.current.scrollTop = ta.scrollTop;
      preRef.current.scrollLeft = ta.scrollLeft;
    }
    if (gutterRef.current) gutterRef.current.scrollTop = ta.scrollTop;
  };

  const handleKey = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    onKeyDown?.(e);
    if (e.defaultPrevented) return;
    if (e.key === "Tab" && !e.metaKey && !e.ctrlKey) {
      e.preventDefault();
      const ta = e.currentTarget;
      const { selectionStart: a, selectionEnd: b } = ta;
      const next = value.slice(0, a) + "  " + value.slice(b);
      onChange(next);
      requestAnimationFrame(() => ta.setSelectionRange(a + 2, a + 2));
    }
  };

  return (
    <div style={{ position: "relative", display: "flex", minHeight: 0, height: "100%", background: "var(--background)" }}>
      <div ref={gutterRef} aria-hidden="true" style={{ ...FONT, flex: "none", width: 48, overflow: "hidden", padding: "8px 0", color: "var(--muted-foreground)", textAlign: "right", userSelect: "none" }}>
        {Array.from({ length: lines }, (_, i) => (
          <div key={i} style={{ paddingRight: 12, background: markLines?.has(i + 1) ? "var(--deleted-surface)" : undefined, color: markLines?.has(i + 1) ? "var(--deleted-foreground)" : undefined }}>
            {i + 1}
          </div>
        ))}
      </div>
      <div style={{ position: "relative", flex: 1, minWidth: 0 }}>
        <pre ref={preRef} aria-hidden="true" style={{ ...FONT, position: "absolute", inset: 0, margin: 0, padding: "8px 16px 8px 4px", overflow: "hidden", whiteSpace: "pre", color: "var(--foreground)", pointerEvents: "none" }}>
          <Highlighted code={value + "\n"} language={language} />
        </pre>
        <textarea
          ref={ref}
          data-main-focus=""
          aria-label={ariaLabel}
          value={value}
          placeholder={placeholder}
          spellCheck={false}
          autoCapitalize="off"
          autoComplete="off"
          autoCorrect="off"
          wrap="off"
          onChange={(e) => onChange(e.target.value)}
          onScroll={sync}
          onKeyDown={handleKey}
          style={{
            ...FONT,
            position: "absolute",
            inset: 0,
            width: "100%",
            height: "100%",
            margin: 0,
            padding: "8px 16px 8px 4px",
            border: 0,
            outline: "none",
            resize: "none",
            background: "transparent",
            color: "transparent",
            caretColor: "var(--foreground)",
            whiteSpace: "pre",
            overflow: "auto",
          }}
        />
      </div>
    </div>
  );
}
