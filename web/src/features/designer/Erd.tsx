/**
 * The diagram (`Diagram` board): table cards placed by the saved layout, relations drawn from
 * each foreign-key column to the referenced column. Drag a card's header to move it; drag the
 * background to pan. New tables, columns and relations are marked until applied.
 */
import * as React from "react";
import { KeyRound, Link2, Table2 } from "lucide-react";

import type { Database } from "@/lib/api/mesaAPI.schemas";
import { cn } from "@/lib/utils";

import { fkId, HEAD, NODE_WIDTH, nodeHeight, ROW, tableKey, tablesOf, type Pos } from "./model";

interface Props {
  model: Database;
  positions: Record<string, Pos>;
  onMove: (key: string, pos: Pos) => void;
  onMoveEnd: () => void;
  selected: string | null;
  onSelect: (key: string | null) => void;
  fresh: { tables: Set<string>; columns: Set<string>; edges: Set<string> };
  empty?: React.ReactNode;
}

export function Erd({ model, positions, onMove, onMoveEnd, selected, onSelect, fresh, empty }: Props) {
  const scroller = React.useRef<HTMLDivElement>(null);
  const drag = React.useRef<{ key: string; dx: number; dy: number } | null>(null);
  const pan = React.useRef<{ x: number; y: number; left: number; top: number } | null>(null);
  const tables = tablesOf(model);
  const byKey = new Map(tables.map((t) => [t.key, t]));

  // Bring the selected table into view (a table just added may sit outside the viewport).
  React.useEffect(() => {
    if (!selected || !scroller.current) return;
    const el = scroller.current.querySelector<HTMLElement>(`[data-node="${CSS.escape(selected)}"]`);
    el?.scrollIntoView({ block: "nearest", inline: "nearest", behavior: "smooth" });
  }, [selected, positions[selected ?? ""]?.x, positions[selected ?? ""]?.y]); // eslint-disable-line react-hooks/exhaustive-deps

  let width = 800;
  let height = 600;
  for (const t of tables) {
    const p = positions[t.key];
    if (!p) continue;
    width = Math.max(width, p.x + NODE_WIDTH + 200);
    height = Math.max(height, p.y + nodeHeight(t.table) + 200);
  }

  const colY = (key: string, column: string): number | null => {
    const t = byKey.get(key);
    const p = positions[key];
    if (!t || !p) return null;
    const i = t.table.columns.findIndex((c) => c.name === column);
    return p.y + HEAD + (i < 0 ? 0 : i) * ROW + ROW / 2 + 1;
  };

  const edges: { id: string; d: string; isNew: boolean; from: Pos; to: Pos }[] = [];
  for (const t of tables) {
    const p = positions[t.key];
    if (!p) continue;
    for (const f of t.table.foreignKeys ?? []) {
      const target = tableKey(f.references.schema, f.references.name);
      const tp = positions[target];
      const y1 = colY(t.key, f.columns[0] ?? "");
      const y2 = colY(target, f.refColumns[0] ?? "");
      if (!tp || y1 === null || y2 === null) continue;
      const rightward = tp.x >= p.x + NODE_WIDTH / 2;
      const x1 = rightward ? p.x + NODE_WIDTH : p.x;
      const x2 = rightward ? tp.x : tp.x + NODE_WIDTH;
      const self = target === t.key;
      const bend = self ? 60 : Math.max(40, Math.abs(x2 - x1) / 2);
      const c1 = rightward ? x1 + bend : x1 - bend;
      const c2 = self ? x2 + bend : rightward ? x2 - bend : x2 + bend;
      const d = self ? `M${p.x + NODE_WIDTH},${y1} C${p.x + NODE_WIDTH + 60},${y1} ${p.x + NODE_WIDTH + 60},${y2} ${p.x + NODE_WIDTH},${y2}` : `M${x1},${y1} C${c1},${y1} ${c2},${y2} ${x2},${y2}`;
      edges.push({ id: `${t.key}:${fkId(f)}`, d, isNew: fresh.edges.has(`${t.key}:${fkId(f)}`), from: { x: x1, y: y1 }, to: { x: self ? p.x + NODE_WIDTH : x2, y: y2 } });
    }
  }

  const onPointerMove = (e: React.PointerEvent) => {
    if (drag.current && scroller.current) {
      const rect = scroller.current.getBoundingClientRect();
      const x = e.clientX - rect.left + scroller.current.scrollLeft - drag.current.dx;
      const y = e.clientY - rect.top + scroller.current.scrollTop - drag.current.dy;
      onMove(drag.current.key, { x: Math.max(0, Math.round(x / 8) * 8), y: Math.max(0, Math.round(y / 8) * 8) });
    } else if (pan.current && scroller.current) {
      scroller.current.scrollLeft = pan.current.left - (e.clientX - pan.current.x);
      scroller.current.scrollTop = pan.current.top - (e.clientY - pan.current.y);
    }
  };
  const onPointerUp = () => {
    if (drag.current) onMoveEnd();
    drag.current = null;
    pan.current = null;
  };

  return (
    <div
      ref={scroller}
      className="m-erd"
      style={{ flex: 1, minWidth: 0, minHeight: 0, overflow: "auto", cursor: pan.current ? "grabbing" : "default" }}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
      onPointerLeave={onPointerUp}
      onPointerDown={(e) => {
        if (e.target !== e.currentTarget && !(e.target as HTMLElement).dataset.canvas) return;
        onSelect(null);
        pan.current = { x: e.clientX, y: e.clientY, left: scroller.current!.scrollLeft, top: scroller.current!.scrollTop };
      }}
      role="application"
      aria-label="Schema diagram"
    >
      <div data-canvas="1" style={{ position: "relative", width, height }}>
        <svg className="m-erd-edge" style={{ left: 0, top: 0, width, height }} width={width} height={height} aria-hidden="true">
          {edges.map((e) => (
            <path key={e.id} d={e.d} className={cn(e.isNew && "is-new")} style={e.isNew ? { stroke: "var(--pending)", strokeDasharray: "4 4" } : undefined} />
          ))}
        </svg>
        {edges.map((e) => (
          <React.Fragment key={`c-${e.id}`}>
            <span className="m-erd-card" style={{ left: e.from.x + (e.from.x < e.to.x ? 4 : -14), top: e.from.y - 15 }}>
              *
            </span>
            <span className="m-erd-card" style={{ left: e.to.x + (e.from.x < e.to.x ? -14 : 4), top: e.to.y - 15 }}>
              1
            </span>
          </React.Fragment>
        ))}
        {tables.map(({ key, table, schema }) => {
          const p = positions[key];
          if (!p) return null;
          const pk = new Set(table.primaryKey?.columns ?? []);
          const fk = new Set((table.foreignKeys ?? []).flatMap((f) => f.columns));
          const isNew = fresh.tables.has(key);
          return (
            <div
              key={key}
              data-node={key}
              className={cn("m-erd-node", selected === key && "is-selected", isNew && "is-new")}
              style={{ left: p.x, top: p.y, width: NODE_WIDTH }}
              onPointerDown={(e) => {
                e.stopPropagation();
                onSelect(key);
              }}
              role="button"
              tabIndex={0}
              aria-label={`Table ${table.name}`}
              aria-pressed={selected === key}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") onSelect(key);
              }}
            >
              <div
                className="m-erd-node__head"
                style={{ cursor: "grab" }}
                onPointerDown={(e) => {
                  if (!scroller.current) return;
                  e.stopPropagation();
                  onSelect(key);
                  const rect = scroller.current.getBoundingClientRect();
                  drag.current = { key, dx: e.clientX - rect.left + scroller.current.scrollLeft - p.x, dy: e.clientY - rect.top + scroller.current.scrollTop - p.y };
                  (e.currentTarget as HTMLElement).setPointerCapture?.(e.pointerId);
                }}
                onPointerMove={onPointerMove}
                onPointerUp={onPointerUp}
              >
                <Table2 className="m-icon" aria-hidden="true" />
                <span style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{schema === "public" ? table.name : `${schema}.${table.name}`}</span>
                <span className="m-erd-node__meta">{isNew ? "new" : table.rowEstimate !== undefined && table.rowEstimate > 0 ? compactRows(table.rowEstimate) : ""}</span>
              </div>
              {table.columns.length === 0 ? (
                <div className="m-erd-col m-muted">no columns</div>
              ) : (
                table.columns.map((c) => (
                  <div key={c.name} className={cn("m-erd-col", fresh.columns.has(`${key}.${c.name}`) && "is-new")} title={`${c.name} ${c.type}${c.nullable ? "" : " not null"}${c.default ? ` default ${c.default}` : ""}`}>
                    {pk.has(c.name) ? <KeyRound className="m-icon" aria-label="Primary key" /> : fk.has(c.name) ? <Link2 className="m-icon" aria-label="Foreign key" /> : <span style={{ width: 12, flex: "none" }} />}
                    <span style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", fontWeight: pk.has(c.name) ? 500 : undefined }}>{c.name}</span>
                    <span className="m-erd-col__type">
                      {c.type}
                      {c.nullable ? "" : " !"}
                    </span>
                  </div>
                ))
              )}
            </div>
          );
        })}
        {tables.length === 0 && empty ? (
          <div style={{ position: "absolute", inset: 0, display: "grid", placeItems: "center", pointerEvents: "none" }}>
            <div style={{ pointerEvents: "auto" }}>{empty}</div>
          </div>
        ) : null}
      </div>
    </div>
  );
}

function compactRows(n: number): string {
  if (n >= 1e6) return `${(n / 1e6).toFixed(1).replace(/\.0$/, "")}M`;
  if (n >= 1e3) return `${(n / 1e3).toFixed(1).replace(/\.0$/, "")}K`;
  return String(n);
}
