/** WHERE conditions for the data grid (`FilterBar` component): column, operator, value. */
import * as React from "react";
import { Plus, X } from "lucide-react";

import type { ResultColumn, RowsQueryFiltersItem, RowsQueryFiltersItemOp } from "@/lib/api/mesaAPI.schemas";
import { parseInput, ValueInputError } from "@/lib/values";
import { Button } from "@/ui/Button";
import { Select } from "@/ui/controls";

export interface FilterDraft {
  column: string;
  op: RowsQueryFiltersItemOp;
  text: string;
}

const OPS: { value: RowsQueryFiltersItemOp; label: string }[] = [
  { value: "=", label: "=" },
  { value: "!=", label: "≠" },
  { value: "<", label: "<" },
  { value: "<=", label: "≤" },
  { value: ">", label: ">" },
  { value: ">=", label: "≥" },
  { value: "ilike", label: "contains" },
  { value: "like", label: "like" },
  { value: "is null", label: "is null" },
  { value: "is not null", label: "is not null" },
];

/** Turns drafts into API filters; throws ValueInputError naming the bad condition. */
export function toFilters(drafts: FilterDraft[], columns: ResultColumn[]): RowsQueryFiltersItem[] {
  const out: RowsQueryFiltersItem[] = [];
  for (const d of drafts) {
    if (!d.column) continue;
    if (d.op === "is null" || d.op === "is not null") {
      out.push({ column: d.column, op: d.op });
      continue;
    }
    if (d.text === "") continue;
    const col = columns.find((c) => c.name === d.column);
    if (d.op === "ilike" || d.op === "like") {
      const text = d.op === "ilike" && !d.text.includes("%") ? `%${d.text}%` : d.text;
      out.push({ column: d.column, op: d.op, value: { k: "text", v: text } });
      continue;
    }
    try {
      out.push({ column: d.column, op: d.op, value: parseInput(col?.kind ?? "text", col?.type ?? "", d.text.replace(/^'(.*)'$/, "$1")) });
    } catch (e) {
      if (e instanceof ValueInputError) throw new ValueInputError(`${d.column}: ${e.message}`);
      throw e;
    }
  }
  return out;
}

interface Props {
  columns: ResultColumn[];
  drafts: FilterDraft[];
  onChange: (drafts: FilterDraft[]) => void;
  onApply: () => void;
  applied: boolean;
}

export function FilterBar({ columns, drafts, onChange, onApply, applied }: Props) {
  const options = columns.map((c) => ({ value: c.name, label: c.name }));
  const set = (i: number, patch: Partial<FilterDraft>) => onChange(drafts.map((d, j) => (j === i ? { ...d, ...patch } : d)));
  const list = drafts.length ? drafts : [{ column: columns[0]?.name ?? "", op: "=" as const, text: "" }];
  return (
    <div className="m-filter" role="group" aria-label="Filter rows" style={{ flexWrap: "wrap", rowGap: 4 }}>
      {list.map((d, i) => (
        <React.Fragment key={i}>
          <span className="m-filter__where">{i === 0 ? "WHERE" : "AND"}</span>
          <Select size="sm" mono aria-label="Column" value={d.column} options={options} onChange={(v) => set(i, { column: v })} style={{ maxWidth: 160 }} />
          <Select size="sm" mono aria-label="Operator" value={d.op} options={OPS} onChange={(v) => set(i, { op: v })} />
          {d.op !== "is null" && d.op !== "is not null" ? (
            <input
              className="m-input m-input--sm m-input--mono"
              aria-label="Value"
              style={{ width: 160 }}
              value={d.text}
              placeholder="value"
              onChange={(e) => (drafts.length ? set(i, { text: e.target.value }) : onChange([{ ...d, text: e.target.value }]))}
              onKeyDown={(e) => e.key === "Enter" && onApply()}
            />
          ) : null}
          {drafts.length > 1 || (applied && drafts.length === 1) ? (
            <Button
              variant="ghost"
              size="sm"
              icon
              aria-label="Remove condition"
              onClick={() => {
                const next = drafts.filter((_, j) => j !== i);
                onChange(next);
                if (next.length === 0) setTimeout(onApply, 0);
              }}
            >
              <X className="m-icon m-icon--sm" aria-hidden="true" />
            </Button>
          ) : null}
        </React.Fragment>
      ))}
      <Button variant="secondary" size="sm" onClick={onApply}>
        Apply
      </Button>
      <Button variant="ghost" size="sm" icon aria-label="Add condition" onClick={() => onChange([...list, { column: columns[0]?.name ?? "", op: "=", text: "" }])}>
        <Plus className="m-icon m-icon--sm" aria-hidden="true" />
      </Button>
    </div>
  );
}
