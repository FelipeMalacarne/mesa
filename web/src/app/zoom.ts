/**
 * The desktop window's zoom. WKWebView and WebKitGTK have none of their own, so the desktop app
 * zooms the window from its View menu and serves the level at /desktop/zoom
 * (internal/platform/desktop/zoom.go). In a browser that path is not there and the browser's
 * own zoom applies.
 */
import * as React from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";

export interface DesktopZoom {
  level: number;
  /** False where the webview zooms by itself (WebView2 on Windows). */
  native: boolean;
}

const KEY = ["desktop-zoom"] as const;
const ZOOM_URL = "/desktop/zoom";

async function readZoom(init?: RequestInit): Promise<DesktopZoom | null> {
  const res = await fetch(ZOOM_URL, { ...init, headers: { "Content-Type": "application/json", ...init?.headers } });
  // The service answers unknown paths with the app's index.html.
  if (!res.ok || !res.headers.get("Content-Type")?.includes("application/json")) {
    if (init?.method === "POST") throw new Error((await res.text()) || `zoom: HTTP ${res.status}`);
    return null;
  }
  return (await res.json()) as DesktopZoom;
}

/** The zoom when Mesa runs as the desktop app, null in a browser; `step` changes it. */
export function useDesktopZoom() {
  const qc = useQueryClient();
  // Read again whenever Settings opens: the View menu may have changed it meanwhile.
  const query = useQuery({ queryKey: KEY, queryFn: () => readZoom(), staleTime: 0, retry: false });

  // While Settings is open, the View menu's changes arrive as `mesa:zoom` events.
  React.useEffect(() => {
    const onZoom = (e: Event) => {
      const level = (e as CustomEvent<unknown>).detail;
      if (typeof level === "number") qc.setQueryData<DesktopZoom | null>(KEY, (z) => (z ? { ...z, level } : z));
    };
    window.addEventListener("mesa:zoom", onZoom);
    return () => window.removeEventListener("mesa:zoom", onZoom);
  }, [qc]);

  const step = async (to: "in" | "out" | "reset") => {
    const z = await readZoom({ method: "POST", body: JSON.stringify({ step: to }) });
    qc.setQueryData(KEY, z);
  };
  return { zoom: query.data ?? null, loading: query.isPending, step };
}
