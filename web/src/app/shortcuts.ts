/**
 * Wires the commands in commands.ts to the keyboard and to desktop menu events. With vim keys
 * on, outside text fields, gt / gT also switch tabs and `:` opens the palette.
 */
import * as React from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "@tanstack/react-router";

import { useGetSystemInfo } from "@/lib/api/system/system";

import { commands, runCommand } from "./commands";
import { matches, typing } from "./keys";
import { useVim } from "./vim";

/** True inside the desktop app, where ⌘W, ⌘T and friends reach the page. */
export function useIsDesktop(): boolean {
  const info = useGetSystemInfo({ query: { staleTime: Infinity } });
  return info.data?.mode === "desktop";
}

/** A dialog (the palette, a review, Settings) is open. */
function dialogOpen(): boolean {
  return !!document.querySelector(".m-overlay");
}

export function useShortcuts() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const params = useParams({ strict: false }) as { connectionId?: string };
  const connectionId = params.connectionId;
  const lastG = React.useRef(0);

  React.useEffect(() => {
    const ctx = { navigate, connectionId, refresh: () => void queryClient.invalidateQueries({ refetchType: "active" }) };
    const onKey = (e: KeyboardEvent) => {
      if (e.defaultPrevented || e.isComposing) return;
      const inField = typing(e.target);
      const vim = useVim.getState().vim;
      const plain = !inField && !e.metaKey && !e.ctrlKey && !e.altKey;
      if (e.key === "?" && plain) {
        e.preventDefault();
        runCommand("help", ctx);
        return;
      }
      // gt / gT and `:`. The grid and the tree take g for their gg, so it is only noted here.
      if (vim && plain && !dialogOpen()) {
        if (e.key === "g") lastG.current = Date.now();
        else if ((e.key === "t" || e.key === "T") && Date.now() - lastG.current < 1000) {
          lastG.current = 0;
          if (runCommand(e.key === "t" ? "tab.next" : "tab.prev", ctx)) e.preventDefault();
          return;
        } else if (e.key === ":") {
          e.preventDefault();
          runCommand("palette", ctx);
          return;
        }
      }
      for (const c of commands) {
        if (c.inConnection && !connectionId) continue;
        if (c.vim && (!vim || dialogOpen())) continue;
        if (c.notIn && e.target instanceof Element && e.target.closest(c.notIn)) continue;
        for (const b of c.keys) {
          if (b.alt && !b.mod && !b.ctrl && inField) continue;
          if (!matches(e, b)) continue;
          e.preventDefault();
          runCommand(c.id, ctx);
          return;
        }
      }
    };
    const onCommand = (e: Event) => {
      const id = (e as CustomEvent<string>).detail;
      if (typeof id === "string") runCommand(id, ctx);
    };
    window.addEventListener("keydown", onKey, true);
    window.addEventListener("mesa:command", onCommand);
    return () => {
      window.removeEventListener("keydown", onKey, true);
      window.removeEventListener("mesa:command", onCommand);
    };
  }, [navigate, connectionId, queryClient]);
}
