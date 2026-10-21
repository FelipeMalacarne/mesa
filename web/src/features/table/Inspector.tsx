/** The row inspector (`Main` board): every column of the selected row with its type and staged value. */
import { PanelRightClose } from "lucide-react";

import type { ResultColumn, Table } from "@/lib/api/mesaAPI.schemas";
import { isNull, textOf } from "@/lib/values";
import { cn } from "@/lib/utils";
import { Button } from "@/ui/Button";

import { cellValue, type GridRow } from "./DataGrid";
import type { TableChanges } from "./changes.store";

export function Inspector({ row, columns, meta, changes, onClose }: { row: GridRow | null; columns: ResultColumn[]; meta?: Table; changes: TableChanges; onClose: () => void }) {
  const pk = new Set(meta?.primaryKey?.columns ?? []);
  return (
    <aside className="m-inspector" aria-label="Row inspector" style={{ width: "var(--size-inspector)", flex: "none" }}>
      <div className="m-inspector__head">
        {row ? (row.kind === "insert" ? "New row" : `Row ${row.key ? Object.values(row.key).map(textOf).join(", ") : ""}`) : "No row selected"}
        <span className="m-spacer" />
        <Button variant="ghost" size="xs" icon aria-label="Close inspector" onClick={onClose}>
          <PanelRightClose className="m-icon m-icon--sm" aria-hidden="true" />
        </Button>
      </div>
      <div className="m-inspector__body">
        {!row ? <p className="m-help">Select a cell to see its row here.</p> : null}
        {row
          ? columns.map((c, i) => {
              const m = meta?.columns.find((x) => x.name === c.name);
              const { value, dirty, unset } = cellValue(row, i, columns, changes);
              const facts = [m?.type ?? c.type, pk.has(c.name) ? "primary key" : null, m?.nullable ? "nullable" : null].filter(Boolean).join(" · ");
              return (
                <div key={c.name} className="m-kv">
                  <span className="m-kv__key">
                    {c.name} <span>{facts}</span>
                  </span>
                  <span className={cn("m-kv__val", dirty && "is-dirty")} style={{ whiteSpace: c.kind === "json" ? "pre-wrap" : undefined }}>
                    {unset ? <span className="m-null">{m?.default || m?.identity ? "DEFAULT" : "NULL"}</span> : isNull(value) ? <span className="m-null">NULL</span> : c.kind === "json" ? prettyJSON(textOf(value)) : textOf(value)}
                  </span>
                </div>
              );
            })
          : null}
      </div>
    </aside>
  );
}

function prettyJSON(text: string): string {
  try {
    return JSON.stringify(JSON.parse(text), null, 2);
  } catch {
    return text;
  }
}
