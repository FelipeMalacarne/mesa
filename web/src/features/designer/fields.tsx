/**
 * Fields shared by the new-table dialog and the inspector: a column type picker fed by the
 * engine's own types, a default picker with the usual values for the type, and the editor for
 * unique constraints and indexes over one or several columns.
 */
import * as React from "react";
import { ChevronsUpDown, Plus, X } from "lucide-react";

import type { ColumnType } from "@/lib/api/mesaAPI.schemas";
import { useGetSystemInfo } from "@/lib/api/system/system";
import { cn } from "@/lib/utils";
import { Button } from "@/ui/Button";
import { Popover, PopoverAnchor, PopoverContent } from "@/ui/primitives/popover";

import { useWorkspace } from "../workspace/context";
import type { KeyKind } from "./model";

/** The column types the connection's engine offers, most used first. */
export function useColumnTypes(): ColumnType[] {
  const { connection } = useWorkspace();
  const info = useGetSystemInfo({ query: { staleTime: Infinity } });
  return React.useMemo(() => info.data?.engines.find((e) => e.name === connection.engine)?.capabilities.types ?? [], [info.data, connection.engine]);
}

/** The type name without its arguments: "varchar(255)" → "varchar". */
export function baseType(type: string): string {
  return type.replace(/\(.*?\)/, "").trim().toLowerCase();
}

export function isIntegerType(type: string, types: ColumnType[]): boolean {
  const base = baseType(type);
  const known = types.find((t) => t.name === base);
  if (known) return !!known.integer;
  return /^(big|small|tiny|medium)?int(eger)?\d?( unsigned)?$/.test(base);
}

interface Option {
  value: string;
  label: string;
  group: string;
  hint?: string;
}

interface ComboProps {
  value: string;
  onChange: (value: string) => void;
  options: Option[];
  /** After picking, select this part of the value (a type's arguments). */
  selectAfterPick?: (value: string) => [number, number] | null;
  placeholder?: string;
  invalid?: boolean;
  id?: string;
  "aria-label"?: string;
  /** Shown under the list when the typed value is none of the options. */
  freeHint?: string;
}

/** A text field with a filtered list of suggestions; any text stays allowed. */
function Combo({ value, onChange, options, selectAfterPick, placeholder, invalid, id, freeHint, ...rest }: ComboProps) {
  const [open, setOpen] = React.useState(false);
  const [filter, setFilter] = React.useState<string | null>(null);
  const [hi, setHi] = React.useState(0);
  const input = React.useRef<HTMLInputElement>(null);
  const list = React.useRef<HTMLDivElement>(null);
  const q = (filter ?? "").trim().toLowerCase();
  const shown = q ? options.filter((o) => o.value.toLowerCase().includes(q) || o.label.toLowerCase().includes(q)) : options;
  const known = options.some((o) => o.value.toLowerCase() === value.trim().toLowerCase() || o.label === baseType(value));

  React.useEffect(() => {
    list.current?.querySelector(".is-selected")?.scrollIntoView({ block: "nearest" });
  }, [hi]);

  const pick = (o: Option) => {
    onChange(o.value);
    setOpen(false);
    setFilter(null);
    const range = selectAfterPick?.(o.value);
    requestAnimationFrame(() => {
      input.current?.focus();
      if (range) input.current?.setSelectionRange(range[0], range[1]);
    });
  };

  let lastGroup = "";
  return (
    <Popover open={open && (shown.length > 0 || (!!freeHint && !!value.trim() && !known))} onOpenChange={setOpen}>
      <PopoverAnchor asChild>
        <span className={cn("m-combo", invalid && "is-invalid")}>
          <input
            ref={input}
            id={id}
            className="m-input m-input--sm m-input--mono m-combo__input"
            value={value}
            placeholder={placeholder}
            autoComplete="off"
            spellCheck={false}
            role="combobox"
            aria-expanded={open}
            aria-autocomplete="list"
            aria-label={rest["aria-label"]}
            onChange={(e) => {
              onChange(e.target.value);
              setFilter(e.target.value);
              setHi(0);
              setOpen(true);
            }}
            onFocus={(e) => {
              setFilter(null);
              setHi(Math.max(0, options.findIndex((o) => o.value === value)));
              // Typing replaces the value; the arrows still move inside it.
              const el = e.currentTarget;
              requestAnimationFrame(() => document.activeElement === el && el.selectionStart === el.selectionEnd && el.select());
            }}
            onClick={() => setOpen(true)}
            onBlur={() => setTimeout(() => setOpen(false), 100)}
            onKeyDown={(e) => {
              if (e.key === "ArrowDown" || e.key === "ArrowUp") {
                e.preventDefault();
                if (!open) return setOpen(true);
                const n = shown.length;
                if (n) setHi((h) => (h + (e.key === "ArrowDown" ? 1 : -1) + n) % n);
              } else if (e.key === "Enter" && open && shown[hi]) {
                e.preventDefault();
                pick(shown[hi]!);
              } else if (e.key === "Escape" && open) {
                e.preventDefault();
                e.stopPropagation();
                setOpen(false);
              } else if (e.key === "Tab") setOpen(false);
            }}
          />
          <button type="button" tabIndex={-1} className="m-combo__toggle" aria-label="Show suggestions" onMouseDown={(e) => e.preventDefault()} onClick={() => (setFilter(null), setOpen(!open), input.current?.focus())}>
            <ChevronsUpDown className="m-icon m-icon--sm" aria-hidden="true" />
          </button>
        </span>
      </PopoverAnchor>
      <PopoverContent
        className="m-combo__list"
        align="start"
        onOpenAutoFocus={(e) => e.preventDefault()}
        onCloseAutoFocus={(e) => e.preventDefault()}
        onInteractOutside={(e) => {
          if (input.current && e.target instanceof Node && input.current.parentElement?.contains(e.target)) e.preventDefault();
        }}
      >
        <div ref={list} role="listbox">
          {shown.map((o, i) => {
            const head = o.group !== lastGroup ? o.group : null;
            lastGroup = o.group;
            return (
              <React.Fragment key={`${o.group}:${o.value}`}>
                {head ? <div className="m-combo__group">{head}</div> : null}
                <button type="button" role="option" aria-selected={i === hi} className={cn("m-menu__item", i === hi && "is-selected")} onMouseDown={(e) => e.preventDefault()} onMouseEnter={() => setHi(i)} onClick={() => pick(o)}>
                  <span className="m-mono">{o.label}</span>
                  {o.hint ? <span className="m-help m-mono" style={{ marginLeft: "auto" }}>{o.hint}</span> : null}
                </button>
              </React.Fragment>
            );
          })}
          {shown.length === 0 && !known && value.trim() && freeHint ? <div className="m-help" style={{ padding: "6px 8px", maxWidth: 280 }}>{freeHint}</div> : null}
        </div>
      </PopoverContent>
    </Popover>
  );
}

interface TypeFieldProps {
  value: string;
  onChange: (value: string) => void;
  types: ColumnType[];
  /** User-defined types in the model (PostgreSQL enums). */
  enums?: string[];
  id?: string;
  "aria-label"?: string;
}

/** The column type: pick one of the engine's types (arguments selected to type over) or write any. */
export function TypeField({ value, onChange, types, enums = [], ...rest }: TypeFieldProps) {
  const options = React.useMemo<Option[]>(
    () => [
      ...types.map((t) => ({ value: t.spelling, label: t.name, group: t.group, hint: t.args ? argsHint(t.args) : undefined })),
      ...enums.map((e) => ({ value: e, label: e, group: "Enums" })),
    ],
    [types, enums],
  );
  return (
    <Combo
      {...rest}
      value={value}
      onChange={onChange}
      options={options}
      invalid={!value.trim()}
      placeholder="type"
      freeHint="Not one of the listed types; the database checks it when you review."
      selectAfterPick={(v) => {
        const a = v.indexOf("(");
        const b = v.lastIndexOf(")");
        return a >= 0 && b > a ? [a + 1, b] : null;
      }}
    />
  );
}

function argsHint(args: string): string {
  return args === "length" ? "(length)" : args === "precision" ? "(precision, scale)" : "(values)";
}

/** Usual defaults for a type; any expression can be written. */
export function defaultsFor(type: string, types: ColumnType[]): string[] {
  const base = baseType(type);
  const group = types.find((t) => t.name === base)?.group;
  if (group === "Date & time" || /date|time/.test(base)) return base === "date" ? ["CURRENT_DATE"] : ["CURRENT_TIMESTAMP"];
  if (group === "Boolean" || base === "boolean" || base === "tinyint(1)") return ["true", "false"];
  if (base === "uuid") return ["gen_random_uuid()"];
  if (group === "JSON") return ["'{}'", "'[]'"];
  if (group === "Numeric") return ["0"];
  if (group === "Text") return ["''"];
  return [];
}

export function DefaultField({ value, onChange, type, types, ...rest }: { value: string; onChange: (v: string) => void; type: string; types: ColumnType[]; id?: string; "aria-label"?: string }) {
  const options = React.useMemo(() => defaultsFor(type, types).map((d) => ({ value: d, label: d, group: "Usual for this type" })), [type, types]);
  return <Combo {...rest} value={value} onChange={onChange} options={options} placeholder="none" />;
}

/* ── unique constraints and indexes ──────────────────────────────────── */

export interface KeyItem {
  kind: KeyKind;
  columns: string[];
  name?: string;
}

/**
 * Unique constraints and indexes over one or several columns. Adding one: choose the kind,
 * click the columns in the order the key should have, then Add.
 */
export function KeyList({ keys, columns, onAdd, onRemove }: { keys: KeyItem[]; columns: string[]; onAdd: (kind: KeyKind, columns: string[]) => void; onRemove: (i: number) => void }) {
  const [draft, setDraft] = React.useState<{ kind: KeyKind; columns: string[] } | null>(null);
  const usable = columns.filter(Boolean);
  return (
    <div className="m-keys">
      {keys.length === 0 && !draft ? <span className="m-help">None. A unique key or index can span several columns.</span> : null}
      {keys.map((k, i) => (
        <div key={i} className="m-keys__item">
          <span className={cn("m-badge m-badge--mono", k.kind === "index" && "m-badge--outline")}>{k.kind === "unique" ? "unique" : "index"}</span>
          <span className="m-mono m-keys__cols">({k.columns.join(", ")})</span>
          {k.name ? <span className="m-help m-mono m-keys__name">{k.name}</span> : null}
          <Button variant="ghost" size="xs" icon aria-label={`Remove ${k.kind} (${k.columns.join(", ")})`} onClick={() => onRemove(i)} style={{ marginLeft: "auto" }}>
            <X className="m-icon m-icon--sm" aria-hidden="true" />
          </Button>
        </div>
      ))}
      {draft ? (
        <div className="m-keys__draft">
          <div className="m-row" style={{ flexWrap: "nowrap" }}>
            <div className="m-seg" role="radiogroup" aria-label="Kind">
              {(["unique", "index"] as KeyKind[]).map((kind) => (
                <button key={kind} type="button" role="radio" aria-checked={draft.kind === kind} className={cn("m-seg__item", draft.kind === kind && "is-active")} onClick={() => setDraft({ ...draft, kind })}>
                  {kind === "unique" ? "Unique" : "Index"}
                </button>
              ))}
            </div>
            <span className="m-help">Click columns in key order</span>
          </div>
          <div className="m-keys__pick" role="group" aria-label="Columns">
            {usable.map((c) => {
              const at = draft.columns.indexOf(c);
              return (
                <button key={c} type="button" aria-pressed={at >= 0} className={cn("m-chip", at >= 0 && "is-on")} onClick={() => setDraft({ ...draft, columns: at >= 0 ? draft.columns.filter((x) => x !== c) : [...draft.columns, c] })}>
                  {at >= 0 ? <span className="m-chip__n">{at + 1}</span> : null}
                  {c}
                </button>
              );
            })}
          </div>
          <div className="m-row">
            <Button
              variant="secondary"
              size="xs"
              disabled={draft.columns.length === 0}
              onClick={() => {
                onAdd(draft.kind, draft.columns);
                setDraft(null);
              }}
            >
              Add {draft.kind === "unique" ? "unique key" : "index"}
              {draft.columns.length ? ` (${draft.columns.join(", ")})` : ""}
            </Button>
            <Button variant="ghost" size="xs" onClick={() => setDraft(null)}>
              Cancel
            </Button>
          </div>
        </div>
      ) : (
        <div className="m-row" style={{ gap: 4 }}>
          <Button variant="ghost" size="sm" disabled={usable.length === 0} onClick={() => setDraft({ kind: "unique", columns: [] })}>
            <Plus className="m-icon m-icon--sm" aria-hidden="true" />
            Unique key
          </Button>
          <Button variant="ghost" size="sm" disabled={usable.length === 0} onClick={() => setDraft({ kind: "index", columns: [] })}>
            <Plus className="m-icon m-icon--sm" aria-hidden="true" />
            Index
          </Button>
        </div>
      )}
    </div>
  );
}
