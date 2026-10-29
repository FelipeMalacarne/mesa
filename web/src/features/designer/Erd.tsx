/**
 * The diagram (`Diagram` board). Table cards sit at their saved positions on a canvas that pans
 * and zooms: scroll or pinch to zoom at the pointer (or scroll to pan, from the zoom menu), drag
 * the background to pan, Shift-drag to select an area. Drag a card to move it, with the other
 * selected cards. Drag the dot at the end of a column onto another table to relate them.
 * Hovering or selecting a table lights up its relations and neighbours and fades the rest.
 * Far out, cards show their names only. New, changed and dropped parts are marked until applied.
 */
import * as React from "react";
import { KeyRound, Link2, Maximize, Minus, Plus, Search, Table2 } from "lucide-react";

import { fuzzyMatch } from "@/lib/fuzzy";
import { cn, isMac } from "@/lib/utils";
import { Button } from "@/ui/Button";
import { Kbd } from "@/ui/controls";
import { Popover, PopoverContent, PopoverTrigger } from "@/ui/primitives/popover";
import { label } from "@/app/keys";

import { edgeShape, type Diagram, type DiagramEdge, type DiagramTable } from "./graph";
import { bounds, type Rect } from "./layout";
import { NODE_WIDTH, type Pos } from "./model";
import { animateView, centerOn, detailAt, fitView, toWorld, visibleRect, wheelPan, wheelZoomFactor, zoomAt, zoomStep, type Detail, type View } from "./viewport";

export type RelationsMode = "all" | "selected";
export type WheelMode = "zoom" | "pan";

export interface ErdHandle {
  /** Shows the given tables (all of them when omitted) whole. */
  fit: (keys?: Iterable<string>, animate?: boolean) => void;
  zoom: (dir: 1 | -1) => void;
  zoomTo: (k: number) => void;
  /** Brings a table into view, zooming in if it would be too small to read. */
  reveal: (key: string) => void;
  /** The world point in the middle of the view: where a new table goes. */
  center: () => Pos;
  find: () => void;
}

interface Props {
  diagram: Diagram;
  positions: Record<string, Pos>;
  /** Focus mode: only these tables are drawn. */
  visible: ReadonlySet<string> | null;
  selected: ReadonlySet<string>;
  selectedEdge: string | null;
  relations: RelationsMode;
  wheel: WheelMode;
  onWheelMode: (m: WheelMode) => void;
  onSelect: (keys: Set<string>) => void;
  onSelectEdge: (id: string) => void;
  onMoveStart: () => void;
  onMove: (moves: Record<string, Pos>) => void;
  onMoveEnd: () => void;
  onConnect: (from: { table: string; column: string }, to: { table: string; column?: string }) => void;
  onAddAt: (world: Pos) => void;
  /** Shown over the top of the canvas (focus mode). */
  banner?: React.ReactNode;
  empty?: React.ReactNode;
}

type Gesture =
  | { kind: "pan"; sx: number; sy: number; view: View }
  | { kind: "drag"; keys: string[]; start: Record<string, Pos>; wx: number; wy: number; moved: boolean; key: string; reselect: boolean }
  | { kind: "marquee"; from: Pos; base: Set<string> }
  | { kind: "connect"; table: string; column: string };

interface Link {
  from: Pos;
  to: Pos;
  target: { table: string; column?: string } | null;
}

const GRID = 8;

export const Erd = React.forwardRef<ErdHandle, Props>(function Erd(props, ref) {
  const { diagram, positions, visible, selected, selectedEdge, relations, wheel, empty, banner } = props;
  const box = React.useRef<HTMLDivElement>(null);
  const [size, setSize] = React.useState({ w: 0, h: 0 });
  const [view, setViewState] = React.useState<View>({ x: 0, y: 0, k: 1 });
  const viewRef = React.useRef(view);
  const cancelAnim = React.useRef<() => void>(() => undefined);
  const setView = React.useCallback((v: View) => {
    viewRef.current = v;
    setViewState(v);
  }, []);
  const gesture = React.useRef<Gesture | null>(null);
  const [hover, setHover] = React.useState<string | null>(null);
  const [marquee, setMarquee] = React.useState<Rect | null>(null);
  const [link, setLink] = React.useState<Link | null>(null);
  const [panning, setPanning] = React.useState(false);
  const [finding, setFinding] = React.useState(false);
  const propsRef = React.useRef(props);
  propsRef.current = props;

  const tables = React.useMemo(() => diagram.tables.filter((t) => positions[t.key] && (!visible || visible.has(t.key))), [diagram, positions, visible]);
  const rects = React.useMemo(() => tables.map((t) => ({ key: t.key, x: positions[t.key]!.x, y: positions[t.key]!.y, w: NODE_WIDTH, h: t.h })), [tables, positions]);

  React.useLayoutEffect(() => {
    const el = box.current;
    if (!el) return;
    const ro = new ResizeObserver(() => setSize({ w: el.clientWidth, h: el.clientHeight }));
    ro.observe(el);
    setSize({ w: el.clientWidth, h: el.clientHeight });
    return () => ro.disconnect();
  }, []);

  // Measured when used: a panel opening beside the canvas narrows it before the observer says so.
  const dims = React.useCallback(() => ({ w: box.current?.clientWidth ?? 0, h: box.current?.clientHeight ?? 0 }), []);

  const go = React.useCallback(
    (to: View, animate: boolean) => {
      cancelAnim.current();
      const { w, h } = dims();
      if (animate) cancelAnim.current = animateView(viewRef.current, to, 260, w, h, setView);
      else setView(to);
    },
    [dims, setView],
  );

  const fit = React.useCallback(
    (keys?: Iterable<string>, animate = true) => {
      const only = keys ? new Set(keys) : null;
      const b = bounds(rects.filter((r) => !only || only.has(r.key)));
      const { w, h } = dims();
      if (!b || !w) return;
      go(fitView(b, w, h), animate);
    },
    [rects, dims, go],
  );

  const reveal = React.useCallback(
    (key: string) => {
      const r = rects.find((x) => x.key === key);
      const { w, h } = dims();
      if (!r || !w) return;
      const v = viewRef.current;
      const seen = visibleRect(v, w, h);
      const inside = r.x >= seen.x && r.y >= seen.y && r.x + r.w <= seen.x + seen.w && r.y + Math.min(r.h, 300) <= seen.y + seen.h;
      if (inside && v.k >= 0.6) return;
      const k = Math.max(v.k, 0.8);
      // Centre the card, or its top when it is taller than the view.
      go(centerOn({ x: r.x + r.w / 2, y: r.y + Math.min(r.h, (h - 96) / k) / 2 }, k, w, h), true);
    },
    [rects, dims, go],
  );

  const zoomBy = React.useCallback(
    (dir: 1 | -1) => {
      const { w, h } = dims();
      go(zoomAt(viewRef.current, zoomStep(viewRef.current.k, dir), w / 2, h / 2), true);
    },
    [dims, go],
  );
  const zoomTo = React.useCallback(
    (k: number) => {
      const { w, h } = dims();
      go(zoomAt(viewRef.current, k, w / 2, h / 2), true);
    },
    [dims, go],
  );

  // Show everything the first time there is something to show.
  const fitted = React.useRef(false);
  React.useEffect(() => {
    if (fitted.current || !size.w || rects.length === 0) return;
    fitted.current = true;
    fit(undefined, false);
  }, [size.w, rects.length, fit]);

  React.useImperativeHandle(
    ref,
    () => ({
      fit,
      zoom: zoomBy,
      zoomTo,
      reveal,
      center: () => {
        const { w, h } = dims();
        return toWorld(viewRef.current, w / 2, h / 2);
      },
      find: () => setFinding(true),
    }),
    [fit, zoomBy, zoomTo, reveal, dims],
  );

  // Wheel: zoom at the pointer (ctrl/⌘ or a pinch always zooms), or pan.
  React.useEffect(() => {
    const el = box.current;
    if (!el) return;
    const onWheel = (e: WheelEvent) => {
      if ((e.target as Element).closest?.("[data-erd-overlay]")) return;
      e.preventDefault();
      cancelAnim.current();
      const r = el.getBoundingClientRect();
      const v = viewRef.current;
      if (e.ctrlKey || e.metaKey || (propsRef.current.wheel === "zoom" && !e.shiftKey)) {
        setView(zoomAt(v, v.k * wheelZoomFactor(e), e.clientX - r.left, e.clientY - r.top));
      } else {
        const d = wheelPan(e);
        setView({ ...v, x: v.x - d.x, y: v.y - d.y });
      }
    };
    el.addEventListener("wheel", onWheel, { passive: false });
    return () => el.removeEventListener("wheel", onWheel);
  }, [setView]);

  const worldAt = (e: { clientX: number; clientY: number }): Pos => {
    const r = box.current!.getBoundingClientRect();
    return toWorld(viewRef.current, e.clientX - r.left, e.clientY - r.top);
  };

  const targetAt = (e: { clientX: number; clientY: number }, from: string): Link["target"] => {
    const el = document.elementFromPoint(e.clientX, e.clientY);
    const node = el?.closest<HTMLElement>("[data-node]");
    if (!node) return null;
    const table = node.dataset.node!;
    const row = el?.closest<HTMLElement>("[data-row]");
    if (table === from && !row) return null;
    return { table, column: row?.dataset.row };
  };

  const onPointerDown = (e: React.PointerEvent) => {
    if (e.button !== 0 && e.button !== 1) return;
    const target = e.target as HTMLElement;
    if (target.closest("[data-erd-overlay]")) return;
    cancelAnim.current();
    const p = propsRef.current;
    const handle = target.closest<HTMLElement>("[data-handle]");
    const node = target.closest<HTMLElement>("[data-node]");
    const edge = target.closest<SVGElement>("[data-edge]");
    box.current?.focus({ preventScroll: true });
    if (e.button === 0 && handle && node) {
      const table = node.dataset.node!;
      const column = handle.dataset.handle!;
      gesture.current = { kind: "connect", table, column };
      const w = worldAt(e);
      setLink({ from: w, to: w, target: null });
    } else if (e.button === 0 && node && !node.dataset.ghost) {
      const key = node.dataset.node!;
      const additive = e.shiftKey || e.metaKey || e.ctrlKey;
      let sel = new Set(p.selected);
      let reselect = false;
      if (additive) {
        if (sel.has(key)) sel.delete(key);
        else sel.add(key);
        p.onSelect(sel);
      } else if (!sel.has(key)) {
        sel = new Set([key]);
        p.onSelect(sel);
      } else reselect = sel.size > 1;
      const keys = sel.has(key) ? [...sel].filter((k) => p.positions[k]) : [];
      const w = worldAt(e);
      gesture.current = { kind: "drag", keys, start: Object.fromEntries(keys.map((k) => [k, p.positions[k]!])), wx: w.x, wy: w.y, moved: false, key, reselect };
    } else if (e.button === 0 && node) {
      // A dropped table: select it to restore it, but it does not move.
      p.onSelect(new Set([node.dataset.node!]));
      return;
    } else if (e.button === 0 && edge) {
      p.onSelectEdge(edge.dataset.edge!);
      return;
    } else if (e.button === 0 && e.shiftKey) {
      gesture.current = { kind: "marquee", from: worldAt(e), base: new Set(p.selected) };
    } else {
      if (e.button === 0 && !e.shiftKey) p.onSelect(new Set());
      gesture.current = { kind: "pan", sx: e.clientX, sy: e.clientY, view: viewRef.current };
      setPanning(true);
    }
    (e.currentTarget as HTMLElement).setPointerCapture(e.pointerId);
  };

  const onPointerMove = (e: React.PointerEvent) => {
    const g = gesture.current;
    if (!g) {
      const key = (e.target as HTMLElement).closest<HTMLElement>("[data-node]")?.dataset.node ?? null;
      if (key !== hover) setHover(key);
      return;
    }
    const p = propsRef.current;
    if (g.kind === "pan") {
      setView({ ...g.view, x: g.view.x + e.clientX - g.sx, y: g.view.y + e.clientY - g.sy });
    } else if (g.kind === "drag") {
      const w = worldAt(e);
      const dx = w.x - g.wx;
      const dy = w.y - g.wy;
      if (!g.moved && Math.hypot(dx, dy) * viewRef.current.k < 3) return;
      if (!g.moved) {
        g.moved = true;
        p.onMoveStart();
      }
      const moves: Record<string, Pos> = {};
      for (const k of g.keys) {
        const s = g.start[k]!;
        moves[k] = { x: Math.max(0, Math.round((s.x + dx) / GRID) * GRID), y: Math.max(0, Math.round((s.y + dy) / GRID) * GRID) };
      }
      p.onMove(moves);
    } else if (g.kind === "marquee") {
      const w = worldAt(e);
      const r = { x: Math.min(g.from.x, w.x), y: Math.min(g.from.y, w.y), w: Math.abs(w.x - g.from.x), h: Math.abs(w.y - g.from.y) };
      setMarquee(r);
      const sel = new Set(g.base);
      for (const t of rects) if (t.x < r.x + r.w && r.x < t.x + t.w && t.y < r.y + r.h && r.y < t.y + t.h) sel.add(t.key);
      p.onSelect(sel);
    } else if (g.kind === "connect") {
      const w = worldAt(e);
      setLink((l) => (l ? { ...l, to: w, target: targetAt(e, g.table) } : l));
    }
  };

  const onPointerUp = (e: React.PointerEvent) => {
    const g = gesture.current;
    gesture.current = null;
    setPanning(false);
    setMarquee(null);
    if (!g) return;
    const p = propsRef.current;
    if (g.kind === "drag") {
      if (g.moved) p.onMoveEnd();
      else if (g.reselect) p.onSelect(new Set([g.key]));
    } else if (g.kind === "connect") {
      setLink(null);
      const t = targetAt(e, g.table);
      if (t) p.onConnect({ table: g.table, column: g.column }, t);
    }
  };

  const onDoubleClick = (e: React.MouseEvent) => {
    const t = e.target as HTMLElement;
    if (t.closest("[data-node]") || t.closest("[data-edge]") || t.closest("[data-erd-overlay]")) return;
    propsRef.current.onAddAt(worldAt(e));
  };

  // What lights up: the selection, else the table under the pointer.
  const active = React.useMemo(() => {
    const keys = new Set(selected);
    if (hover && !gesture.current) keys.add(hover);
    return keys;
  }, [selected, hover]);
  const lit = React.useMemo(() => {
    const out = new Set<string>();
    if (active.size === 0 && !selectedEdge) return null;
    for (const k of active) out.add(k);
    for (const e of diagram.edges) {
      if (active.has(e.from)) out.add(e.to);
      if (active.has(e.to)) out.add(e.from);
      if (e.id === selectedEdge) (out.add(e.from), out.add(e.to));
    }
    return out;
  }, [active, diagram.edges, selectedEdge]);

  const detail = detailAt(view.k);
  const dot = 16 * 2 ** Math.max(0, Math.ceil(Math.log2(8 / (16 * view.k))));
  const empty_ = diagram.tables.length === 0 && empty;

  return (
    <div
      ref={box}
      className={cn("m-erd", panning && "is-panning", link && "is-linking")}
      style={{ flex: 1, minWidth: 0, minHeight: 0, backgroundSize: `${dot * view.k}px ${dot * view.k}px`, backgroundPosition: `${view.x}px ${view.y}px` }}
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
      onPointerCancel={onPointerUp}
      onPointerLeave={() => !gesture.current && setHover(null)}
      onDoubleClick={onDoubleClick}
      role="application"
      aria-label="Schema diagram"
      aria-roledescription="diagram"
      tabIndex={0}
    >
      <div className="m-erd__world" style={{ transform: `translate(${view.x}px, ${view.y}px) scale(${view.k})`, ["--erd-k" as string]: view.k }}>
        <Edges diagram={diagram} positions={positions} visible={visible} active={active} lit={lit} selectedEdge={selectedEdge} relations={relations} />
        {tables.map((t) => (
          <Node
            key={t.key}
            t={t}
            x={positions[t.key]!.x}
            y={positions[t.key]!.y}
            selected={selected.has(t.key)}
            tone={!lit ? undefined : active.has(t.key) ? undefined : lit.has(t.key) ? "near" : "dim"}
            detail={detail}
            drop={link?.target?.table === t.key ? (link.target.column ?? "") : undefined}
            title={t.schema === "public" || t.schema === diagram.name ? t.table.name : `${t.schema}.${t.table.name}`}
          />
        ))}
        {link ? (
          <svg className="m-erd-link" aria-hidden="true">
            <path d={`M${link.from.x},${link.from.y} L${link.to.x},${link.to.y}`} />
          </svg>
        ) : null}
        {marquee ? <div className="m-erd-marquee" style={{ left: marquee.x, top: marquee.y, width: marquee.w, height: marquee.h }} /> : null}
      </div>
      {banner ? (
        <div className="m-erd-banner" data-erd-overlay="">
          {banner}
        </div>
      ) : null}
      {diagram.tables.length > 0 ? (
        <>
          <Finder open={finding} onOpenChange={setFinding} tables={diagram.tables} onPick={(key) => {
              props.onSelect(new Set([key]));
              reveal(key);
            }} />
          <div className="m-erd-dock" data-erd-overlay="">
            <Minimap rects={rects} diagram={diagram} selected={selected} view={view} size={size} onCenter={(p) => go(centerOn(p, viewRef.current.k, dims().w, dims().h), false)} />
            <ZoomBar view={view} wheel={wheel} onWheelMode={props.onWheelMode} onZoom={zoomBy} onZoomTo={zoomTo} onFit={() => fit()} onFitSelection={() => selected.size && fit(selected)} />
          </div>
        </>
      ) : null}
      {empty_ ? (
        <div style={{ position: "absolute", inset: 0, display: "grid", placeItems: "center", pointerEvents: "none" }}>
          <div style={{ pointerEvents: "auto" }} data-erd-overlay="">
            {empty}
          </div>
        </div>
      ) : null}
    </div>
  );
});

/* ── cards ────────────────────────────────────────────────────────────── */

interface NodeProps {
  t: DiagramTable;
  x: number;
  y: number;
  selected: boolean;
  tone: "near" | "dim" | undefined;
  detail: Detail;
  /** While relating: the column under the pointer ("" for the card itself). */
  drop: string | undefined;
  title: string;
}

const Node = React.memo(function Node({ t, x, y, selected, tone, detail, drop, title }: NodeProps) {
  const { table, state } = t;
  const meta = state === "new" ? "new" : state === "dropped" ? "drop" : table.rowEstimate !== undefined && table.rowEstimate > 0 ? compactRows(table.rowEstimate) : "";
  const cls = cn("m-erd-node", `is-${detail}`, selected && "is-selected", state !== "same" && `is-${state}`, tone && `is-${tone}`, drop === "" && "is-drop");
  const tip = `${title}${table.comment ? ` — ${table.comment}` : ""}`;
  if (detail === "outline") {
    return (
      <div className={cls} style={{ left: x, top: y, width: NODE_WIDTH, height: t.h }} data-node={t.key} data-ghost={state === "dropped" ? "" : undefined} title={tip}>
        <span className="m-erd-node__label">{title}</span>
      </div>
    );
  }
  return (
    <div className={cls} style={{ left: x, top: y, width: NODE_WIDTH, height: t.h }} data-node={t.key} data-ghost={state === "dropped" ? "" : undefined} role="group" aria-label={`Table ${title}${selected ? ", selected" : ""}${state !== "same" ? `, ${state}` : ""}`}>
      <div className="m-erd-node__head" title={tip}>
        <Table2 className="m-icon" aria-hidden="true" />
        <span className="m-erd-node__name">{title}</span>
        <span className="m-erd-node__meta">{meta}</span>
      </div>
      {detail === "names" ? (
        <div className="m-erd-node__body" />
      ) : t.rows.length === 0 ? (
        <div className="m-erd-col m-muted">no columns</div>
      ) : (
        t.rows.map((r) => (
          <div
            key={r.column.name}
            data-row={r.column.name}
            className={cn("m-erd-col", r.state !== "same" && `is-${r.state}`, drop === r.column.name && "is-drop")}
            title={`${r.column.name} ${r.column.type}${r.column.nullable ? "" : " not null"}${r.column.default ? ` default ${r.column.default}` : ""}${r.unique ? " unique" : ""}${r.column.comment ? ` — ${r.column.comment}` : ""}${r.state === "dropped" ? " (dropped)" : r.state === "changed" ? " (changed)" : ""}`}
          >
            {r.pk ? <KeyRound className="m-icon" aria-label="Primary key" /> : r.fk ? <Link2 className="m-icon" aria-label="Foreign key" /> : <span className="m-erd-col__gap" />}
            <span className="m-erd-col__name" style={{ fontWeight: r.pk ? 500 : undefined }}>
              {r.column.name}
            </span>
            <span className="m-erd-col__type">
              {r.column.type}
              {r.column.nullable ? "" : " !"}
            </span>
            {state !== "dropped" && r.state !== "dropped" ? (
              <>
                <span className="m-erd-handle m-erd-handle--l" data-handle={r.column.name} title="Drag to another table to relate" />
                <span className="m-erd-handle m-erd-handle--r" data-handle={r.column.name} title="Drag to another table to relate" />
              </>
            ) : null}
          </div>
        ))
      )}
    </div>
  );
});

/* ── relations ────────────────────────────────────────────────────────── */

interface EdgesProps {
  diagram: Diagram;
  positions: Record<string, Pos>;
  visible: ReadonlySet<string> | null;
  active: ReadonlySet<string>;
  lit: ReadonlySet<string> | null;
  selectedEdge: string | null;
  relations: RelationsMode;
}

const Edges = React.memo(function Edges({ diagram, positions, visible, active, lit, selectedEdge, relations }: EdgesProps) {
  const drawn: { e: DiagramEdge; on: boolean }[] = [];
  let w = 0;
  let h = 0;
  for (const t of diagram.tables) {
    const p = positions[t.key];
    if (p) (w = Math.max(w, p.x + NODE_WIDTH + 200), h = Math.max(h, p.y + t.h + 200));
  }
  for (const e of diagram.edges) {
    if (!positions[e.from] || !positions[e.to]) continue;
    if (visible && (!visible.has(e.from) || !visible.has(e.to))) continue;
    const on = active.has(e.from) || active.has(e.to) || e.id === selectedEdge;
    if (relations === "selected" && !on && e.state === "same") continue;
    drawn.push({ e, on });
  }
  // Lit relations on top.
  drawn.sort((a, b) => Number(a.on) - Number(b.on));
  return (
    <svg className="m-erd-edges" width={w} height={h} aria-hidden="true">
      {drawn.map(({ e, on }) => {
        const s = edgeShape(positions[e.from]!, e.fromRow, positions[e.to]!, e.toRow, e.from === e.to, e);
        return (
          <g key={e.id} data-edge={e.id} className={cn("m-erd-rel", e.state !== "same" && `is-${e.state}`, on && "is-on", e.id === selectedEdge && "is-selected", lit && !on && "is-dim")}>
            <path className="m-erd-rel__hit" d={s.d} />
            <path className="m-erd-rel__line" d={s.d} />
            <path className="m-erd-rel__marks" d={s.marks} />
            {s.rings.map((r, i) => (
              <circle key={i} className="m-erd-rel__ring" cx={r.x} cy={r.y} r={3.5} />
            ))}
          </g>
        );
      })}
    </svg>
  );
});

/* ── overlays ─────────────────────────────────────────────────────────── */

function Minimap({ rects, diagram, selected, view, size, onCenter }: { rects: (Rect & { key: string })[]; diagram: Diagram; selected: ReadonlySet<string>; view: View; size: { w: number; h: number }; onCenter: (p: Pos) => void }) {
  const W = 184;
  const H = 120;
  const seen = visibleRect(view, size.w, size.h);
  const all = bounds(rects);
  if (!all || !size.w) return null;
  const world = bounds([all, seen])!;
  const s = Math.min(W / world.w, H / world.h);
  const ox = (W - world.w * s) / 2 - world.x * s;
  const oy = (H - world.h * s) / 2 - world.y * s;
  const centerAt = (e: React.PointerEvent<SVGSVGElement>) => {
    const r = e.currentTarget.getBoundingClientRect();
    onCenter({ x: (e.clientX - r.left - ox) / s, y: (e.clientY - r.top - oy) / s });
  };
  return (
    <svg
      className="m-erd-minimap"
      width={W}
      height={H}
      role="img"
      aria-label="Overview; click or drag to move the view"
      onPointerDown={(e) => {
        e.currentTarget.setPointerCapture(e.pointerId);
        centerAt(e);
      }}
      onPointerMove={(e) => e.buttons === 1 && centerAt(e)}
    >
      {rects.map((r) => {
        const st = diagram.byKey.get(r.key)?.state;
        return <rect key={r.key} className={cn("m-erd-minimap__node", selected.has(r.key) && "is-selected", st && st !== "same" && `is-${st}`)} x={r.x * s + ox} y={r.y * s + oy} width={Math.max(1, r.w * s)} height={Math.max(1, r.h * s)} rx={1} />;
      })}
      <rect className="m-erd-minimap__view" x={seen.x * s + ox} y={seen.y * s + oy} width={seen.w * s} height={seen.h * s} />
    </svg>
  );
}

function ZoomBar({ view, wheel, onWheelMode, onZoom, onZoomTo, onFit, onFitSelection }: { view: View; wheel: WheelMode; onWheelMode: (m: WheelMode) => void; onZoom: (d: 1 | -1) => void; onZoomTo: (k: number) => void; onFit: () => void; onFitSelection: () => void }) {
  const [open, setOpen] = React.useState(false);
  const item = (text: string, keys: string | null, run: () => void, on?: boolean) => (
    <button
      type="button"
      className={cn("m-menu__item", on && "is-selected")}
      onClick={() => {
        run();
        setOpen(false);
      }}
    >
      {text}
      {keys ? <Kbd className="m-erd-menu__kbd">{keys}</Kbd> : null}
    </button>
  );
  return (
    <div className="m-erd-zoom">
      <Button variant="ghost" size="xs" icon aria-label="Zoom out" title={`Zoom out (${label({ code: "Minus" })})`} onClick={() => onZoom(-1)}>
        <Minus className="m-icon m-icon--sm" aria-hidden="true" />
      </Button>
      <Popover open={open} onOpenChange={setOpen}>
        <PopoverTrigger asChild>
          <button type="button" className="m-erd-zoom__level" aria-label="Zoom options">
            {Math.round(view.k * 100)}%
          </button>
        </PopoverTrigger>
        <PopoverContent side="top" align="end" className="m-erd-menu" data-erd-overlay="">
          {item("Zoom to fit", label({ code: "Digit1", shift: true }), onFit)}
          {item("Zoom to selection", label({ code: "Digit2", shift: true }), onFitSelection)}
          {item("Zoom to 50%", null, () => onZoomTo(0.5))}
          {item("Zoom to 100%", label({ code: "Digit0", shift: true }), () => onZoomTo(1))}
          {item("Zoom to 200%", null, () => onZoomTo(2))}
          <div className="m-menu__sep" />
          {item("Scroll wheel zooms", null, () => onWheelMode("zoom"), wheel === "zoom")}
          {item("Scroll wheel pans", null, () => onWheelMode("pan"), wheel === "pan")}
          <p className="m-help" style={{ padding: "4px 8px", margin: 0, maxWidth: 240, whiteSpace: "normal" }}>
            {isMac ? "⌘" : "Ctrl+"}scroll or a pinch always zooms; Shift+scroll pans.
          </p>
        </PopoverContent>
      </Popover>
      <Button variant="ghost" size="xs" icon aria-label="Zoom in" title={`Zoom in (${label({ code: "Equal" })})`} onClick={() => onZoom(1)}>
        <Plus className="m-icon m-icon--sm" aria-hidden="true" />
      </Button>
      <Button variant="ghost" size="xs" icon aria-label="Zoom to fit" title={`Zoom to fit (${label({ code: "Digit1", shift: true })})`} onClick={onFit}>
        <Maximize className="m-icon m-icon--sm" aria-hidden="true" />
      </Button>
    </div>
  );
}

function Finder({ open, onOpenChange, tables, onPick }: { open: boolean; onOpenChange: (o: boolean) => void; tables: DiagramTable[]; onPick: (key: string) => void }) {
  const [q, setQ] = React.useState("");
  const [i, setI] = React.useState(0);
  const input = React.useRef<HTMLInputElement>(null);
  React.useEffect(() => {
    if (open) input.current?.focus();
  }, [open]);
  const hits = React.useMemo(() => {
    // Spaces separate words the way underscores do in names: "doc ver" finds document_versions.
    const query = q.trim().toLowerCase().replace(/\s+/g, "");
    if (!query) return [];
    return tables
      .map((t) => ({ t, m: fuzzyMatch(query, t.table.name) ?? fuzzyMatch(query, t.key) }))
      .filter((x) => x.m)
      .sort((a, b) => b.m!.score - a.m!.score || a.t.key.localeCompare(b.t.key))
      .slice(0, 8);
  }, [q, tables]);
  const pick = (key: string) => {
    onPick(key);
    setQ("");
    onOpenChange(false);
    input.current?.blur();
  };
  return (
    <div className={cn("m-erd-find", (open || q) && "is-open")} data-erd-overlay="">
      <Search className="m-icon m-icon--sm m-muted" aria-hidden="true" />
      <input
        ref={input}
        className="m-erd-find__input"
        placeholder="Find a table…"
        aria-label="Find a table"
        value={q}
        onFocus={() => onOpenChange(true)}
        onBlur={() => setTimeout(() => onOpenChange(false), 120)}
        onChange={(e) => {
          setQ(e.target.value);
          setI(0);
        }}
        onKeyDown={(e) => {
          if (e.key === "ArrowDown") (e.preventDefault(), setI((i + 1) % Math.max(1, hits.length)));
          else if (e.key === "ArrowUp") (e.preventDefault(), setI((i - 1 + hits.length) % Math.max(1, hits.length)));
          else if (e.key === "Enter" && hits[i]) (e.preventDefault(), pick(hits[i].t.key));
          else if (e.key === "Escape") {
            e.preventDefault();
            e.stopPropagation();
            setQ("");
            e.currentTarget.blur();
          }
        }}
      />
      {!q ? <Kbd>/</Kbd> : null}
      {hits.length > 0 ? (
        <ul className="m-erd-find__list m-popover" role="listbox" aria-label="Tables">
          {hits.map((h, j) => (
            <li key={h.t.key} role="option" aria-selected={j === i}>
              <button type="button" className={cn("m-menu__item", j === i && "is-selected")} onMouseDown={(e) => e.preventDefault()} onClick={() => pick(h.t.key)}>
                <Table2 className="m-icon m-icon--sm" aria-hidden="true" />
                <span className="m-mono">{h.t.table.name}</span>
                <span className="m-help m-mono" style={{ marginLeft: "auto" }}>
                  {h.t.schema}
                </span>
              </button>
            </li>
          ))}
        </ul>
      ) : q.trim() ? (
        <div className="m-erd-find__list m-popover m-help" style={{ padding: 8 }}>
          No table matches.
        </div>
      ) : null}
    </div>
  );
}

function compactRows(n: number): string {
  if (n >= 1e6) return `${(n / 1e6).toFixed(1).replace(/\.0$/, "")}M`;
  if (n >= 1e3) return `${(n / 1e3).toFixed(1).replace(/\.0$/, "")}K`;
  return String(n);
}
