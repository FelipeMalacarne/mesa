/**
 * The diagram's camera: a screen point is `world * k + (x, y)`. Pure helpers for zooming at a
 * point, fitting a rectangle, stepping through zoom levels and reading wheel events.
 */
import type { Pos } from "./model";
import type { Rect } from "./layout";

export interface View {
  x: number;
  y: number;
  k: number;
}

export const MIN_ZOOM = 0.05;
export const MAX_ZOOM = 2.5;

/** Below these zooms a card shows less: names only, then a block with a large name. */
export const DETAIL_NAMES = 0.45;
export const DETAIL_OUTLINE = 0.2;

export type Detail = "full" | "names" | "outline";

export function detailAt(k: number): Detail {
  return k >= DETAIL_NAMES ? "full" : k >= DETAIL_OUTLINE ? "names" : "outline";
}

export function clampZoom(k: number): number {
  return Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, k));
}

export function toWorld(v: View, sx: number, sy: number): Pos {
  return { x: (sx - v.x) / v.k, y: (sy - v.y) / v.k };
}

/** Zoom to `k` keeping the world point under screen point (sx, sy) where it is. */
export function zoomAt(v: View, k: number, sx: number, sy: number): View {
  const nk = clampZoom(k);
  const w = toWorld(v, sx, sy);
  return { k: nk, x: sx - w.x * nk, y: sy - w.y * nk };
}

const STEPS = [0.05, 0.1, 0.15, 0.25, 0.33, 0.5, 0.67, 0.8, 1, 1.25, 1.5, 2, 2.5];

/** The next zoom level in or out from `k`. */
export function zoomStep(k: number, dir: 1 | -1): number {
  if (dir > 0) return STEPS.find((s) => s > k + 1e-3) ?? MAX_ZOOM;
  return [...STEPS].reverse().find((s) => s < k - 1e-3) ?? MIN_ZOOM;
}

/** The view that shows `r` whole in a w×h viewport, centred, no closer than `maxK`. */
export function fitView(r: Rect, w: number, h: number, pad = 48, maxK = 1): View {
  const k = clampZoom(Math.min(maxK, (w - pad * 2) / Math.max(1, r.w), (h - pad * 2) / Math.max(1, r.h)));
  return { k, x: (w - r.w * k) / 2 - r.x * k, y: (h - r.h * k) / 2 - r.y * k };
}

/** The view centred on world point p at zoom k. */
export function centerOn(p: Pos, k: number, w: number, h: number): View {
  const nk = clampZoom(k);
  return { k: nk, x: w / 2 - p.x * nk, y: h / 2 - p.y * nk };
}

/**
 * How much a wheel event zooms, as a factor. Trackpad pinches arrive as ctrl+wheel with small
 * deltas; mouse wheels as lines or ~100px notches.
 */
export function wheelZoomFactor(e: { deltaY: number; deltaMode: number; ctrlKey: boolean }): number {
  const unit = e.deltaMode === 1 ? 0.05 : e.deltaMode === 2 ? 1 : 0.002;
  const delta = Math.max(-1, Math.min(1, -e.deltaY * unit * (e.ctrlKey ? 5 : 1)));
  return 2 ** delta;
}

/** Wheel deltas in pixels (lines and pages scaled), for panning. */
export function wheelPan(e: { deltaX: number; deltaY: number; deltaMode: number; shiftKey: boolean }): Pos {
  const unit = e.deltaMode === 1 ? 16 : e.deltaMode === 2 ? 400 : 1;
  // Shift turns a vertical wheel sideways, as browsers do for pages.
  if (e.shiftKey && e.deltaX === 0) return { x: e.deltaY * unit, y: 0 };
  return { x: e.deltaX * unit, y: e.deltaY * unit };
}

/** The visible world rectangle. */
export function visibleRect(v: View, w: number, h: number): Rect {
  const a = toWorld(v, 0, 0);
  return { x: a.x, y: a.y, w: w / v.k, h: h / v.k };
}

/** Eases a view towards another over `ms` in a w×h viewport; returns a cancel function. */
export function animateView(from: View, to: View, ms: number, w: number, h: number, set: (v: View) => void): () => void {
  if (typeof requestAnimationFrame === "undefined" || ms <= 0) {
    set(to);
    return () => undefined;
  }
  const t0 = performance.now();
  let raf = 0;
  // The zoom moves on a log scale so zooming in and out feel the same, and the world point at
  // the middle of the screen moves in a straight line.
  const lk0 = Math.log(from.k);
  const lk1 = Math.log(to.k);
  const c0 = toWorld(from, w / 2, h / 2);
  const c1 = toWorld(to, w / 2, h / 2);
  const tick = (now: number) => {
    const t = Math.min(1, (now - t0) / ms);
    if (t >= 1) return set(to);
    const e = 1 - (1 - t) ** 3;
    const k = Math.exp(lk0 + (lk1 - lk0) * e);
    set(centerOn({ x: c0.x + (c1.x - c0.x) * e, y: c0.y + (c1.y - c0.y) * e }, k, w, h));
    raf = requestAnimationFrame(tick);
  };
  raf = requestAnimationFrame(tick);
  return () => cancelAnimationFrame(raf);
}
