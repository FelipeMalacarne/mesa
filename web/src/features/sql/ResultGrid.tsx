/**
 * A read-only result table in the grid look, for query results. It takes the keyboard like the
 * data grid: arrows (and h j k l, gg G, 0 $, counts with vim keys on) move between cells, ⌘C or
 * y copies the cell, Y the row (lib/gridKeys.ts).
 */
import * as React from "react";
import { toast } from "sonner";

import type { QueryResult } from "@/lib/api/mesaAPI.schemas";
import { gridKey, newGridKeyState, visibleRows, type Cell } from "@/lib/gridKeys";
import { cn, isCmd } from "@/lib/utils";
import { isNull, isNumeric, textOf, type Value } from "@/lib/values";
import { useVim } from "@/app/vim";

function cellText(v: Value | undefined): string {
  return !v || isNull(v) ? "" : textOf(v);
}

export function ResultGrid({ result }: { result: QueryResult }) {
  const [active, setActive] = React.useState<Cell | null>(null);
  const vim = useVim((s) => s.vim);
  const keys = React.useRef(newGridKeyState());
  const ref = React.useRef<HTMLTableElement>(null);

  // A new result starts without a cell picked.
  React.useEffect(() => setActive(null), [result]);
  React.useEffect(() => {
    if (!active) return;
    ref.current?.querySelector<HTMLElement>(`[data-cell="${active.row}:${active.col}"]`)?.scrollIntoView({ block: "nearest", inline: "nearest" });
  }, [active]);

  const copy = (text: string, what: string) =>
    void navigator.clipboard.writeText(text).then(
      () => what && toast.success(`Copied ${what}`),
      () => toast.error("The clipboard is not available"),
    );

  const onKeyDown = (e: React.KeyboardEvent) => {
    const row = active ? result.rows[active.row] : undefined;
    if (row && active && isCmd(e) && e.key.toLowerCase() === "c" && !window.getSelection()?.toString()) {
      e.preventDefault();
      copy(cellText(row[active.col]), "");
      return;
    }
    if (e.key === "Escape" && active) {
      keys.current = newGridKeyState();
      setActive(null);
      return;
    }
    const action = gridKey(e, active, { rows: result.rows.length, cols: result.columns.length, page: visibleRows(ref.current) }, vim, keys.current);
    if (!action) return;
    e.preventDefault();
    if (action.kind === "move") setActive(action.cell);
    else if (action.kind === "copy-cell" && row && active) copy(cellText(row[active.col]), "");
    else if (action.kind === "copy-row" && row) copy(row.map(cellText).join("\t"), "the row");
  };

  return (
    <table ref={ref} className={cn("m-grid", active && "is-focused")} tabIndex={0} role="grid" aria-rowcount={result.rows.length} data-pane="" onKeyDown={onKeyDown}>
      <thead>
        <tr>
          <th className="m-gutter">#</th>
          {result.columns.map((c, i) => (
            <th key={`${c.name}${i}`} className={cn(isNumeric(c.kind) && "is-num")}>
              <span className="m-th">
                <span>{c.name}</span>
                <span className="m-th__type">{c.type}</span>
              </span>
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {result.rows.map((row, r) => (
          <tr key={r} aria-rowindex={r + 1}>
            <td className="m-gutter" onClick={() => setActive({ row: r, col: active?.col ?? 0 })}>
              {r + 1}
            </td>
            {row.map((v, i) => (
              <td
                key={i}
                data-cell={`${r}:${i}`}
                className={cn(isNumeric(result.columns[i]?.kind) && "is-num", active?.row === r && active.col === i && "is-active")}
                title={isNull(v) ? undefined : textOf(v).slice(0, 500)}
                onMouseDown={() => setActive({ row: r, col: i })}
              >
                {isNull(v) ? <span className="m-null">NULL</span> : result.columns[i]?.kind === "json" ? <span className="m-json">{textOf(v)}</span> : textOf(v)}
              </td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  );
}
