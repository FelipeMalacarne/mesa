/**
 * Diagram layouts. Each is pure and deterministic: the same model always gives the same
 * positions.
 *
 * - `hierarchy`: referenced tables left of the tables that reference them. Layers are ordered
 *   to cut crossings (barycenter sweeps), a layer taller than the diagram should be is wrapped
 *   into several columns, and tables slide up or down to line up with the rows they relate to.
 * - `clusters`: a force simulation that pulls related tables together. Hubs (a table half the
 *   database refers to) pull less, so groups of tables that belong together stay together.
 * - `grid`: alphabetical, in balanced columns; easiest to scan, ignores relations.
 *
 * Groups of tables that do not relate to each other are laid out apart and packed in rows;
 * tables without any relation come last, in a grid.
 */
import type { Database } from "@/lib/api/mesaAPI.schemas";

import { HEAD, NODE_WIDTH, nodeHeight, ROW, tableKey, tablesOf, type Pos } from "./model";

export type LayoutKind = "hierarchy" | "clusters" | "grid";

const GAP_X = 120; // between layers: room for the curves
const GAP_SUB = 48; // between the columns of one wrapped layer
const GAP_Y = 32;
const GAP_BLOCK = 120; // between groups of tables
const MARGIN = 40;
const ASPECT = 1.6; // width over height the layouts aim for
const GRID = 8;

export interface LayoutNode {
  key: string;
  w: number;
  h: number;
}

/** A relation as the layout sees it: which tables, and where on each the line attaches. */
export interface LayoutLink {
  from: string;
  to: string;
  fromY: number;
  toY: number;
}

export interface LayoutGraph {
  nodes: LayoutNode[];
  links: LayoutLink[];
}

export interface Rect {
  x: number;
  y: number;
  w: number;
  h: number;
}

function rowCenter(columns: { name: string }[], column: string | undefined): number {
  const i = columns.findIndex((c) => c.name === column);
  return HEAD + Math.max(0, i) * ROW + ROW / 2;
}

/** The tables of `db` (or the `only` ones) and the relations between them. */
export function graphOf(db: Database, only?: ReadonlySet<string>): LayoutGraph {
  const refs = tablesOf(db).filter((r) => !only || only.has(r.key));
  const byKey = new Map(refs.map((r) => [r.key, r]));
  const links: LayoutLink[] = [];
  for (const r of refs) {
    for (const f of r.table.foreignKeys ?? []) {
      const to = tableKey(f.references.schema, f.references.name);
      const t = byKey.get(to);
      if (!t || to === r.key) continue;
      links.push({ from: r.key, to, fromY: rowCenter(r.table.columns, f.columns[0]), toY: rowCenter(t.table.columns, f.refColumns[0]) });
    }
  }
  return { nodes: refs.map((r) => ({ key: r.key, w: NODE_WIDTH, h: nodeHeight(r.table) })), links };
}

/** Positions for every table of `db` (or the `only` ones) by the given layout. */
export function layout(db: Database, kind: LayoutKind, only?: ReadonlySet<string>): Record<string, Pos> {
  return layoutGraph(graphOf(db, only), kind);
}

export function layoutGraph(g: LayoutGraph, kind: LayoutKind): Record<string, Pos> {
  if (g.nodes.length === 0) return {};
  if (kind === "grid") return snap(place(gridBlock([...g.nodes].sort((a, b) => a.key.localeCompare(b.key)), Infinity), MARGIN, MARGIN));
  const groups = components(g);
  const isolated = groups.filter((c) => c.nodes.length === 1).map((c) => c.nodes[0]!);
  const blocks = groups.filter((c) => c.nodes.length > 1).map((c) => (kind === "clusters" ? clusters(c) : hierarchy(c)));
  if (isolated.length) blocks.push(gridBlock(isolated.sort((a, b) => a.key.localeCompare(b.key)), widest(blocks)));
  return snap(pack(blocks));
}

/* ── groups ───────────────────────────────────────────────────────────── */

function components(g: LayoutGraph): LayoutGraph[] {
  const parent = new Map(g.nodes.map((n) => [n.key, n.key]));
  const find = (k: string): string => {
    let r = k;
    while (parent.get(r) !== r) r = parent.get(r)!;
    parent.set(k, r);
    return r;
  };
  for (const l of g.links) parent.set(find(l.from), find(l.to));
  const groups = new Map<string, LayoutGraph>();
  for (const n of g.nodes) {
    const r = find(n.key);
    if (!groups.has(r)) groups.set(r, { nodes: [], links: [] });
    groups.get(r)!.nodes.push(n);
  }
  for (const l of g.links) groups.get(find(l.from))!.links.push(l);
  // Biggest first, so the main group sits top left.
  return [...groups.values()].sort((a, b) => b.nodes.length - a.nodes.length || a.nodes[0]!.key.localeCompare(b.nodes[0]!.key));
}

/** A laid-out group: positions relative to its own top left, and its size. */
interface Block {
  w: number;
  h: number;
  pos: Map<string, Pos>;
}

function blockOf(nodes: LayoutNode[], pos: Map<string, Pos>): Block {
  let minX = Infinity;
  let minY = Infinity;
  let maxX = -Infinity;
  let maxY = -Infinity;
  for (const n of nodes) {
    const p = pos.get(n.key)!;
    minX = Math.min(minX, p.x);
    minY = Math.min(minY, p.y);
    maxX = Math.max(maxX, p.x + n.w);
    maxY = Math.max(maxY, p.y + n.h);
  }
  const out = new Map<string, Pos>();
  for (const [k, p] of pos) out.set(k, { x: p.x - minX, y: p.y - minY });
  return { w: maxX - minX, h: maxY - minY, pos: out };
}

function widest(blocks: Block[]): number {
  return blocks.reduce((m, b) => Math.max(m, b.w), 0);
}

/** Shelf packing: blocks left to right in rows about as wide as the whole should be. */
function pack(blocks: Block[]): Record<string, Pos> {
  const area = blocks.reduce((s, b) => s + (b.w + GAP_BLOCK) * (b.h + GAP_BLOCK), 0);
  const width = Math.max(widest(blocks), Math.sqrt(area * ASPECT));
  const out: Record<string, Pos> = {};
  let x = MARGIN;
  let y = MARGIN;
  let shelf = 0;
  for (const b of blocks) {
    if (x > MARGIN && x + b.w > MARGIN + width) {
      x = MARGIN;
      y += shelf + GAP_BLOCK;
      shelf = 0;
    }
    Object.assign(out, place(b, x, y));
    x += b.w + GAP_BLOCK;
    shelf = Math.max(shelf, b.h);
  }
  return out;
}

function place(b: Block, x: number, y: number): Record<string, Pos> {
  const out: Record<string, Pos> = {};
  for (const [k, p] of b.pos) out[k] = { x: p.x + x, y: p.y + y };
  return out;
}

function snap(pos: Record<string, Pos>): Record<string, Pos> {
  for (const k of Object.keys(pos)) pos[k] = { x: Math.round(pos[k]!.x / GRID) * GRID, y: Math.round(pos[k]!.y / GRID) * GRID };
  return pos;
}

/* ── grid ─────────────────────────────────────────────────────────────── */

/** Tables in order, top to bottom then left to right, in columns of about equal height. */
function gridBlock(nodes: LayoutNode[], maxWidth: number): Block {
  const total = nodes.reduce((s, n) => s + n.h + GAP_Y, 0);
  const colW = NODE_WIDTH + GAP_SUB;
  let cols = Math.max(1, Math.round(Math.sqrt((ASPECT * total) / colW)));
  if (Number.isFinite(maxWidth) && maxWidth > 0) cols = Math.min(cols, Math.max(1, Math.floor((maxWidth + GAP_SUB) / colW)));
  cols = Math.min(cols, nodes.length);
  const target = total / cols;
  const pos = new Map<string, Pos>();
  let col = 0;
  let y = 0;
  for (const n of nodes) {
    if (y > 0 && y + n.h / 2 > target && col < cols - 1) {
      col++;
      y = 0;
    }
    pos.set(n.key, { x: col * colW, y });
    y += n.h + GAP_Y;
  }
  return blockOf(nodes, pos);
}

/* ── hierarchy ────────────────────────────────────────────────────────── */

function hierarchy(g: LayoutGraph): Block {
  const keys = g.nodes.map((n) => n.key).sort();
  const size = new Map(g.nodes.map((n) => [n.key, n]));
  const refs = new Map<string, string[]>(keys.map((k) => [k, []]));
  for (const l of g.links) refs.get(l.from)!.push(l.to);

  // Layer = longest chain of references below the table; a cycle is cut where it is found.
  const layerOf = new Map<string, number>();
  const visiting = new Set<string>();
  const visit = (k: string): number => {
    const known = layerOf.get(k);
    if (known !== undefined) return known;
    if (visiting.has(k)) return -1;
    visiting.add(k);
    let d = 0;
    for (const r of refs.get(k)!) {
      const rd = visit(r);
      if (rd >= 0) d = Math.max(d, rd + 1);
    }
    visiting.delete(k);
    layerOf.set(k, d);
    return d;
  };
  for (const k of keys) visit(k);
  const depth = Math.max(...layerOf.values()) + 1;
  const layers: string[][] = Array.from({ length: depth }, () => []);
  for (const k of keys) layers[layerOf.get(k)!]!.push(k);

  orderLayers(layers, layerOf, g.links);

  // Wrap tall layers so the group comes out about ASPECT wide.
  const area = g.nodes.reduce((s, n) => s + (n.w + GAP_X) * (n.h + GAP_Y), 0);
  const tallest = Math.max(...g.nodes.map((n) => n.h));
  const target = Math.max(tallest, Math.sqrt(area / ASPECT));
  const columns: { keys: string[]; x: number }[] = [];
  let x = 0;
  for (const layer of layers) {
    const height = layer.reduce((s, k) => s + size.get(k)!.h + GAP_Y, 0);
    const k = Math.max(1, Math.min(layer.length, Math.ceil(height / target)));
    const cols = Array.from({ length: k }, () => ({ keys: [] as string[], bottom: 0 }));
    for (const key of layer) {
      // Into the shortest column, so each column spans the layer's order top to bottom.
      const c = cols.reduce((best, c) => (c.bottom < best.bottom ? c : best), cols[0]!);
      c.keys.push(key);
      c.bottom += size.get(key)!.h + GAP_Y;
    }
    for (const c of cols) {
      columns.push({ keys: c.keys, x });
      x += NODE_WIDTH + GAP_SUB;
    }
    x += GAP_X - GAP_SUB;
  }

  const pos = new Map<string, Pos>();
  const colOf = new Map<string, number>();
  columns.forEach((c, i) => {
    let y = 0;
    for (const k of c.keys) {
      pos.set(k, { x: c.x, y });
      colOf.set(k, i);
      y += size.get(k)!.h + GAP_Y;
    }
  });
  alignRows(columns.map((c) => c.keys), pos, colOf, size, g.links);
  return blockOf(g.nodes, pos);
}

/** Barycenter sweeps down and up the layers; keeps the order with the fewest crossings. */
function orderLayers(layers: string[][], layerOf: Map<string, number>, links: LayoutLink[]) {
  const below = new Map<string, string[]>(); // referenced, in lower layers
  const above = new Map<string, string[]>(); // referencing, in higher layers
  for (const l of links) {
    const a = layerOf.get(l.from)!;
    const b = layerOf.get(l.to)!;
    if (a === b) continue;
    const [hi, lo] = a > b ? [l.from, l.to] : [l.to, l.from];
    below.set(hi, [...(below.get(hi) ?? []), lo]);
    above.set(lo, [...(above.get(lo) ?? []), hi]);
  }
  const rank = new Map<string, number>();
  const rerank = () => layers.forEach((layer) => layer.forEach((k, i) => rank.set(k, layer.length > 1 ? i / (layer.length - 1) : 0.5)));
  rerank();
  const sortBy = (layer: string[], nbrs: Map<string, string[]>) => {
    const key = new Map(
      layer.map((k) => {
        const ns = nbrs.get(k) ?? [];
        return [k, ns.length ? ns.reduce((s, n) => s + rank.get(n)!, 0) / ns.length : rank.get(k)!];
      }),
    );
    layer.sort((a, b) => key.get(a)! - key.get(b)! || rank.get(a)! - rank.get(b)!);
  };
  let best = layers.map((l) => [...l]);
  let bestCross = crossings(layers, layerOf, links);
  for (let it = 0; it < 8 && bestCross > 0; it++) {
    if (it % 2 === 0) for (let i = 1; i < layers.length; i++) (sortBy(layers[i]!, below), rerank());
    else for (let i = layers.length - 2; i >= 0; i--) (sortBy(layers[i]!, above), rerank());
    const c = crossings(layers, layerOf, links);
    if (c < bestCross) {
      bestCross = c;
      best = layers.map((l) => [...l]);
    }
  }
  best.forEach((l, i) => (layers[i] = l));
}

/** Crossings between relations that join the same two layers. */
export function crossings(layers: string[][], layerOf: Map<string, number>, links: LayoutLink[]): number {
  const idx = new Map<string, number>();
  layers.forEach((l) => l.forEach((k, i) => idx.set(k, i)));
  const byPair = new Map<string, [number, number][]>();
  for (const l of links) {
    const a = layerOf.get(l.from)!;
    const b = layerOf.get(l.to)!;
    if (a === b) continue;
    const [p, q] = a < b ? [l.to, l.from] : [l.from, l.to];
    const pair = `${Math.min(a, b)}:${Math.max(a, b)}`;
    byPair.set(pair, [...(byPair.get(pair) ?? []), [idx.get(q)!, idx.get(p)!]]);
  }
  let n = 0;
  for (const segs of byPair.values()) for (let i = 0; i < segs.length; i++) for (let j = i + 1; j < segs.length; j++) if ((segs[i]![0] - segs[j]![0]) * (segs[i]![1] - segs[j]![1]) < 0) n++;
  return n;
}

/**
 * Slides tables up and down their column so related rows line up: each wants the average
 * height its relations ask for, and the column keeps its order and gaps (isotonic regression,
 * pool adjacent violators).
 */
function alignRows(columns: string[][], pos: Map<string, Pos>, colOf: Map<string, number>, size: Map<string, LayoutNode>, links: LayoutLink[]) {
  const pulls = new Map<string, { other: string; own: number; theirs: number }[]>();
  for (const l of links) {
    if (colOf.get(l.from) === colOf.get(l.to)) continue;
    pulls.set(l.from, [...(pulls.get(l.from) ?? []), { other: l.to, own: l.fromY, theirs: l.toY }]);
    pulls.set(l.to, [...(pulls.get(l.to) ?? []), { other: l.from, own: l.toY, theirs: l.fromY }]);
  }
  for (let it = 0; it < 12; it++) {
    const order = it % 2 === 0 ? columns : [...columns].reverse();
    for (const col of order) {
      const want = col.map((k) => {
        const ps = pulls.get(k);
        if (!ps?.length) return pos.get(k)!.y;
        return ps.reduce((s, p) => s + pos.get(p.other)!.y + p.theirs - p.own, 0) / ps.length;
      });
      const ys = stackNear(
        want,
        col.map((k) => size.get(k)!.h),
      );
      col.forEach((k, i) => (pos.get(k)!.y = ys[i]!));
    }
  }
}

/** Y for items kept in order with GAP_Y between them, as close as possible to `want`. */
export function stackNear(want: number[], heights: number[]): number[] {
  // Shift out the gaps, so the constraint becomes z[i] <= z[i+1].
  const offset: number[] = [];
  let acc = 0;
  for (const h of heights) {
    offset.push(acc);
    acc += h + GAP_Y;
  }
  const blocks: { sum: number; n: number }[] = [];
  for (let i = 0; i < want.length; i++) {
    blocks.push({ sum: want[i]! - offset[i]!, n: 1 });
    while (blocks.length > 1) {
      const b = blocks[blocks.length - 1]!;
      const a = blocks[blocks.length - 2]!;
      if (a.sum / a.n <= b.sum / b.n) break;
      blocks.pop();
      a.sum += b.sum;
      a.n += b.n;
    }
  }
  const out: number[] = [];
  for (const b of blocks) for (let j = 0; j < b.n; j++) out.push(b.sum / b.n + offset[out.length]!);
  return out;
}

/* ── clusters ─────────────────────────────────────────────────────────── */

function clusters(g: LayoutGraph): Block {
  const n = g.nodes.length;
  const keys = g.nodes.map((x) => x.key);
  const idx = new Map(keys.map((k, i) => [k, i]));
  const deg = new Array<number>(n).fill(0);
  const edges: [number, number, number][] = [];
  for (const l of g.links) {
    const a = idx.get(l.from)!;
    const b = idx.get(l.to)!;
    deg[a]!++;
    deg[b]!++;
    edges.push([a, b, 0]);
  }
  // A hub pulls each of its tables less, so specific relations decide the groups.
  for (const e of edges) e[2] = 1 / Math.sqrt(deg[e[0]]! * deg[e[1]]!);

  const w = g.nodes.map((x) => x.w);
  const h = g.nodes.map((x) => x.h);
  const avg = Math.sqrt(g.nodes.reduce((s, x) => s + x.w * x.h, 0) / n);
  const K = avg * 1.4; // the length a relation settles at

  // Start on a sunflower spiral in breadth-first order from the busiest table.
  const nbrs: number[][] = Array.from({ length: n }, () => []);
  for (const [a, b] of edges) (nbrs[a]!.push(b), nbrs[b]!.push(a));
  const start = deg.reduce((best, d, i) => (d > deg[best]! || (d === deg[best]! && keys[i]! < keys[best]!) ? i : best), 0);
  const seen = new Set([start]);
  const queue = [start];
  for (let q = 0; q < queue.length; q++) {
    const sorted = [...nbrs[queue[q]!]!].sort((a, b) => deg[b]! - deg[a]! || keys[a]!.localeCompare(keys[b]!));
    for (const m of sorted) if (!seen.has(m)) (seen.add(m), queue.push(m));
  }
  const x = new Float64Array(n);
  const y = new Float64Array(n);
  const golden = Math.PI * (3 - Math.sqrt(5));
  queue.forEach((v, i) => {
    const r = avg * 0.9 * Math.sqrt(i);
    x[v] = r * Math.cos(i * golden);
    y[v] = r * Math.sin(i * golden);
  });

  const iterations = Math.max(60, Math.min(400, Math.floor(4e6 / (n * n))));
  const dx = new Float64Array(n);
  const dy = new Float64Array(n);
  for (let it = 0; it < iterations; it++) {
    const t = K * 1.5 * (1 - it / iterations) + 2;
    dx.fill(0);
    dy.fill(0);
    for (let i = 0; i < n; i++) {
      for (let j = i + 1; j < n; j++) {
        let ex = x[i]! - x[j]!;
        let ey = y[i]! - y[j]!;
        if (ex === 0 && ey === 0) ex = 0.01 * (i - j);
        const c = Math.hypot(ex, ey);
        // Only near tables push: far ones would spread the whole group thin.
        if (c > 3 * K) continue;
        // Distance between the cards' edges, not their centres, so big tables keep room.
        const d = Math.max(1, c - (Math.min(w[i]!, h[i]!) + Math.min(w[j]!, h[j]!)) / 4);
        const f = (K * K) / d / c;
        ex *= f;
        ey *= f;
        dx[i]! += ex;
        dy[i]! += ey;
        dx[j]! -= ex;
        dy[j]! -= ey;
      }
    }
    for (const [a, b, wt] of edges) {
      const ex = x[a]! - x[b]!;
      const ey = y[a]! - y[b]!;
      const d = Math.hypot(ex, ey) || 1;
      const f = ((d * d) / K / d) * wt;
      dx[a]! -= ex * f;
      dy[a]! -= ey * f;
      dx[b]! += ex * f;
      dy[b]! += ey * f;
    }
    for (let i = 0; i < n; i++) {
      // A weak pull to the middle keeps loosely tied tables from drifting off.
      dx[i]! -= x[i]! * 0.02;
      dy[i]! -= y[i]! * 0.02;
      const d = Math.hypot(dx[i]!, dy[i]!);
      if (d > 0) {
        const step = Math.min(d, t) / d;
        x[i]! += dx[i]! * step;
        y[i]! += dy[i]! * step;
      }
    }
  }

  // Draw the arrangement in so the cards nearly touch, then push apart the ones that overlap.
  const box = bounds(g.nodes.map((nd, i) => ({ x: x[i]! - nd.w / 2, y: y[i]! - nd.h / 2, w: nd.w, h: nd.h })))!;
  const room = g.nodes.reduce((s, nd) => s + (nd.w + GAP_SUB) * (nd.h + GAP_Y), 0) * 1.6;
  const k = Math.min(1, Math.sqrt(room / (box.w * box.h)));
  const rects: Rect[] = g.nodes.map((nd, i) => ({ x: x[i]! * k - nd.w / 2, y: y[i]! * k - nd.h / 2, w: nd.w, h: nd.h }));
  separate(rects, GAP_Y);
  const pos = new Map<string, Pos>();
  rects.forEach((r, i) => pos.set(keys[i]!, { x: r.x, y: r.y }));
  return blockOf(g.nodes, pos);
}

/** Pushes overlapping rectangles apart along the axis they overlap least on. */
export function separate(rects: Rect[], pad: number) {
  for (let pass = 0; pass < 1000; pass++) {
    let moved = false;
    for (let i = 0; i < rects.length; i++) {
      for (let j = i + 1; j < rects.length; j++) {
        const a = rects[i]!;
        const b = rects[j]!;
        const ox = Math.min(a.x + a.w, b.x + b.w) + pad - Math.max(a.x, b.x);
        const oy = Math.min(a.y + a.h, b.y + b.h) + pad - Math.max(a.y, b.y);
        if (ox <= 0 || oy <= 0) continue;
        moved = true;
        const acx = a.x + a.w / 2;
        const bcx = b.x + b.w / 2;
        const acy = a.y + a.h / 2;
        const bcy = b.y + b.h / 2;
        if (ox < oy) {
          const s = (ox / 2 + 0.5) * (acx < bcx || (acx === bcx && i < j) ? 1 : -1);
          a.x -= s;
          b.x += s;
        } else {
          const s = (oy / 2 + 0.5) * (acy < bcy || (acy === bcy && i < j) ? 1 : -1);
          a.y -= s;
          b.y += s;
        }
      }
    }
    if (!moved) return;
  }
}

/* ── placing new tables ───────────────────────────────────────────────── */

function overlaps(a: Rect, b: Rect, pad: number): boolean {
  return a.x < b.x + b.w + pad && b.x < a.x + a.w + pad && a.y < b.y + b.h + pad && b.y < a.y + a.h + pad;
}

/** The free spot for a w×h card nearest to `near`, searching outwards in rings. */
export function freeSpot(taken: Rect[], w: number, h: number, near: Pos): Pos {
  const step = 48;
  for (let ring = 0; ring < 60; ring++) {
    const spots: Pos[] = [];
    if (ring === 0) spots.push(near);
    for (let i = -ring; i <= ring; i++) {
      spots.push({ x: near.x + i * step, y: near.y - ring * step }, { x: near.x + i * step, y: near.y + ring * step });
      if (Math.abs(i) !== ring) spots.push({ x: near.x - ring * step, y: near.y + i * step }, { x: near.x + ring * step, y: near.y + i * step });
    }
    spots.sort((a, b) => Math.hypot(a.x - near.x, a.y - near.y) - Math.hypot(b.x - near.x, b.y - near.y));
    for (const s of spots) {
      if (s.x < 0 || s.y < 0) continue;
      const r = { x: s.x, y: s.y, w, h };
      if (!taken.some((t) => overlaps(r, t, GAP_Y))) return { x: Math.round(s.x / GRID) * GRID, y: Math.round(s.y / GRID) * GRID };
    }
  }
  return near;
}

/**
 * Positions for tables that have none yet. A table that relates to placed tables goes beside
 * them; others go near `near` (the middle of the view) or under everything.
 */
export function placeMissing(db: Database, positions: Record<string, Pos>, near?: Pos): Record<string, Pos> {
  const refs = tablesOf(db);
  const missing = refs.filter((t) => !positions[t.key]);
  if (missing.length === 0) return positions;
  if (refs.every((t) => !positions[t.key])) return layout(db, "hierarchy");
  const out = { ...positions };
  const heights = new Map(refs.map((t) => [t.key, nodeHeight(t.table)]));
  const taken: Rect[] = refs.filter((t) => positions[t.key]).map((t) => ({ ...positions[t.key]!, w: NODE_WIDTH, h: heights.get(t.key)! }));
  let bottom = 0;
  for (const r of taken) bottom = Math.max(bottom, r.y + r.h);
  let row = MARGIN;
  for (const m of missing) {
    const h = heights.get(m.key)!;
    // Beside a table it references, else beside a table that references it.
    const target = (m.table.foreignKeys ?? []).map((f) => tableKey(f.references.schema, f.references.name)).find((k) => out[k] && k !== m.key) ?? refs.find((t) => t.key !== m.key && out[t.key] && (t.table.foreignKeys ?? []).some((f) => tableKey(f.references.schema, f.references.name) === m.key))?.key;
    let start: Pos;
    if (target) start = { x: out[target]!.x + NODE_WIDTH + GAP_X, y: out[target]!.y };
    else if (near) start = { x: near.x - NODE_WIDTH / 2, y: near.y - h / 2 };
    else {
      start = { x: row, y: bottom + 60 };
      row += NODE_WIDTH + 60;
    }
    const p = freeSpot(taken, NODE_WIDTH, h, start);
    out[m.key] = p;
    taken.push({ ...p, w: NODE_WIDTH, h });
  }
  return out;
}

/** The rectangle around the given cards. */
export function bounds(rects: Rect[]): Rect | null {
  if (rects.length === 0) return null;
  let minX = Infinity;
  let minY = Infinity;
  let maxX = -Infinity;
  let maxY = -Infinity;
  for (const r of rects) {
    minX = Math.min(minX, r.x);
    minY = Math.min(minY, r.y);
    maxX = Math.max(maxX, r.x + r.w);
    maxY = Math.max(maxY, r.y + r.h);
  }
  return { x: minX, y: minY, w: maxX - minX, h: maxY - minY };
}
