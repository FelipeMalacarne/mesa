/** Wires the commands in commands.ts to the keyboard and to desktop menu events. */
import * as React from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "@tanstack/react-router";

import { useGetSystemInfo } from "@/lib/api/system/system";

import { commands, runCommand } from "./commands";
import { matches, typing } from "./keys";

/** True inside the desktop app, where ⌘W, ⌘T and friends reach the page. */
export function useIsDesktop(): boolean {
  const info = useGetSystemInfo({ query: { staleTime: Infinity } });
  return info.data?.mode === "desktop";
}

export function useShortcuts() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const params = useParams({ strict: false }) as { connectionId?: string };
  const connectionId = params.connectionId;

  React.useEffect(() => {
    const ctx = { navigate, connectionId, refresh: () => void queryClient.invalidateQueries({ refetchType: "active" }) };
    const onKey = (e: KeyboardEvent) => {
      if (e.defaultPrevented || e.isComposing) return;
      const inField = typing(e.target);
      if (e.key === "?" && !inField && !e.metaKey && !e.ctrlKey && !e.altKey) {
        e.preventDefault();
        runCommand("help", ctx);
        return;
      }
      for (const c of commands) {
        if (c.inConnection && !connectionId) continue;
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
