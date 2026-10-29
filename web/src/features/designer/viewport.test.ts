import { describe, expect, it } from "vitest";

import { clampZoom, detailAt, fitView, MAX_ZOOM, MIN_ZOOM, toWorld, wheelPan, wheelZoomFactor, zoomAt, zoomStep } from "./viewport";

describe("designer viewport", () => {
  it("keeps the point under the cursor in place when zooming", () => {
    const v = { x: 30, y: -20, k: 0.8 };
    const before = toWorld(v, 400, 300);
    const after = toWorld(zoomAt(v, 1.7, 400, 300), 400, 300);
    expect(after.x).toBeCloseTo(before.x);
    expect(after.y).toBeCloseTo(before.y);
  });

  it("clamps the zoom", () => {
    expect(clampZoom(100)).toBe(MAX_ZOOM);
    expect(clampZoom(0)).toBe(MIN_ZOOM);
  });

  it("steps through zoom levels", () => {
    expect(zoomStep(1, 1)).toBe(1.25);
    expect(zoomStep(1, -1)).toBe(0.8);
    expect(zoomStep(0.9, 1)).toBe(1);
    expect(zoomStep(MAX_ZOOM, 1)).toBe(MAX_ZOOM);
  });

  it("fits a rectangle in the middle, never past 100%", () => {
    const v = fitView({ x: 100, y: 100, w: 2000, h: 1000 }, 1000, 600, 0);
    expect(v.k).toBeCloseTo(0.5);
    expect(toWorld(v, 500, 300)).toEqual({ x: 1100, y: 600 });
    expect(fitView({ x: 0, y: 0, w: 10, h: 10 }, 1000, 600).k).toBe(1);
  });

  it("reads wheels: notches zoom by steps, pinches gently, shift pans sideways", () => {
    expect(wheelZoomFactor({ deltaY: -100, deltaMode: 0, ctrlKey: false })).toBeGreaterThan(1.1);
    expect(wheelZoomFactor({ deltaY: 100, deltaMode: 0, ctrlKey: false })).toBeLessThan(0.9);
    expect(wheelZoomFactor({ deltaY: 2, deltaMode: 0, ctrlKey: true })).toBeGreaterThan(0.95);
    expect(wheelPan({ deltaX: 0, deltaY: 3, deltaMode: 1, shiftKey: true })).toEqual({ x: 48, y: 0 });
  });

  it("shows less of a card as it gets small", () => {
    expect(detailAt(1)).toBe("full");
    expect(detailAt(0.3)).toBe("names");
    expect(detailAt(0.1)).toBe("outline");
  });
});
