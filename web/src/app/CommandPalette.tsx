/**
 * ⌘K: jump to a connection, a table of the open connection, or a tool. Tables come from the
 * schemas already loaded in the tree, so the palette never waits on the network.
 */
import * as React from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useNavigate, useParams } from "@tanstack/react-router";
import { Activity, Command as CommandIcon, Database, Eye, History, LayoutGrid, Moon, SquareTerminal, Table2, Users } from "lucide-react";

import { useListConnections } from "@/lib/api/connections/connections";
import type { Database as DatabaseModel } from "@/lib/api/mesaAPI.schemas";
import { CommandDialog, CommandEmpty, CommandGroup, CommandInput, CommandItem, CommandList } from "@/ui/primitives/command";
import { EnvTag, Kbd } from "@/ui/controls";
import { tabLink, useTabs } from "@/features/explorer/tabs.store";

import { commands, runCommand, usableKeys } from "./commands";
import { label } from "./keys";
import { useIsDesktop } from "./shortcuts";
import { useTheme } from "./theme";
import { useUi } from "./ui.store";

export function CommandPalette() {
  const open = useUi((s) => s.paletteOpen);
  const setOpen = useUi((s) => s.setPaletteOpen);
  const navigate = useNavigate();
  const qc = useQueryClient();
  const params = useParams({ strict: false }) as { connectionId?: string };
  const connectionId = params.connectionId;
  const connections = useListConnections({ query: { enabled: open } });
  const toggleTheme = useTheme((s) => s.toggle);
  const desktop = useIsDesktop();

  const tables = React.useMemo(() => {
    if (!open || !connectionId) return [];
    const out: { database: string; schema: string; name: string; kind: "table" | "view" }[] = [];
    const prefix = `/connections/${connectionId}/databases/`;
    for (const [key, data] of qc.getQueriesData<DatabaseModel>({ queryKey: [] })) {
      const k = typeof key[0] === "string" ? key[0] : "";
      if (!data || !k.startsWith(prefix) || !k.endsWith("/schema")) continue;
      const db = k.slice(prefix.length, -"/schema".length);
      for (const s of data.schemas ?? []) {
        for (const t of s.tables) out.push({ database: decodeURIComponent(db), schema: s.name, name: t.name, kind: "table" });
        for (const v of s.views ?? []) out.push({ database: decodeURIComponent(db), schema: s.name, name: v.name, kind: "view" });
      }
    }
    return out.slice(0, 400);
  }, [open, connectionId, qc]);

  const run = (fn: () => void) => {
    setOpen(false);
    fn();
  };

  return (
    <CommandDialog open={open} onOpenChange={setOpen}>
      <CommandInput placeholder={connectionId ? "Search tables, tools and connections" : "Search connections"} />
      <CommandList>
        <CommandEmpty>Nothing matches.</CommandEmpty>
        {connectionId ? (
          <>
            {tables.length > 0 ? (
              <CommandGroup heading="Tables">
                {tables.map((t) => (
                  <CommandItem
                    key={`${t.database}.${t.schema}.${t.name}`}
                    value={`${t.name} ${t.schema} ${t.database}`}
                    onSelect={() =>
                      run(() =>
                        void navigate({
                          to: "/c/$connectionId/db/$database/t/$schema/$table",
                          params: { connectionId, database: t.database, schema: t.schema, table: t.name },
                          search: { view: "data", kind: t.kind },
                        }),
                      )
                    }
                  >
                    {t.kind === "view" ? <Eye className="m-icon m-icon--sm" aria-hidden="true" /> : <Table2 className="m-icon m-icon--sm" aria-hidden="true" />}
                    {t.name}
                    <span className="m-help m-mono">
                      {t.database}.{t.schema}
                    </span>
                  </CommandItem>
                ))}
              </CommandGroup>
            ) : null}
            <CommandGroup heading="Tools">
              <CommandItem
                value="new query sql editor"
                onSelect={() =>
                  run(() => {
                    const tab = useTabs.getState().newQuery(connectionId);
                    void navigate(tabLink(connectionId, tab));
                  })
                }
              >
                <SquareTerminal className="m-icon m-icon--sm" aria-hidden="true" />
                New query
              </CommandItem>
              <CommandItem value="schema designer erd dbml" onSelect={() => run(() => void navigate({ to: "/c/$connectionId/designer", params: { connectionId }, search: {} }))}>
                <LayoutGrid className="m-icon m-icon--sm" aria-hidden="true" />
                Schema designer
              </CommandItem>
              <CommandItem value="monitor metrics" onSelect={() => run(() => void navigate({ to: "/c/$connectionId/monitor", params: { connectionId }, search: { view: "overview" } }))}>
                <Activity className="m-icon m-icon--sm" aria-hidden="true" />
                Monitor
              </CommandItem>
              <CommandItem value="sessions activity backends" onSelect={() => run(() => void navigate({ to: "/c/$connectionId/monitor", params: { connectionId }, search: { view: "sessions" } }))}>
                <Activity className="m-icon m-icon--sm" aria-hidden="true" />
                Sessions
              </CommandItem>
              <CommandItem value="users roles" onSelect={() => run(() => void navigate({ to: "/c/$connectionId/roles", params: { connectionId } }))}>
                <Users className="m-icon m-icon--sm" aria-hidden="true" />
                Users &amp; roles
              </CommandItem>
              <CommandItem value="activity audit log" onSelect={() => run(() => void navigate({ to: "/c/$connectionId/activity", params: { connectionId } }))}>
                <History className="m-icon m-icon--sm" aria-hidden="true" />
                Activity
              </CommandItem>
            </CommandGroup>
          </>
        ) : null}
        <CommandGroup heading="Connections">
          {(connections.data ?? []).map((c) => (
            <CommandItem key={c.id} value={`${c.name} ${c.host} ${c.environment}`} onSelect={() => run(() => void navigate({ to: "/c/$connectionId", params: { connectionId: c.id } }))}>
              <Database className="m-icon m-icon--sm" aria-hidden="true" />
              {c.name}
              <EnvTag env={c.environment} />
            </CommandItem>
          ))}
          <CommandItem value="all connections home" onSelect={() => run(() => void navigate({ to: "/" }))}>
            <LayoutGrid className="m-icon m-icon--sm" aria-hidden="true" />
            All connections
          </CommandItem>
        </CommandGroup>
        <CommandGroup heading="Commands">
          {commands
            .filter((c) => !c.hidden && c.id !== "palette" && (!c.inConnection || connectionId))
            .map((c) => {
              const key = usableKeys(c, desktop)[0];
              return (
                <CommandItem key={c.id} value={`${c.title} command`} onSelect={() => run(() => void runCommand(c.id, { navigate, connectionId, refresh: () => void qc.invalidateQueries({ refetchType: "active" }) }))}>
                  <CommandIcon className="m-icon m-icon--sm" aria-hidden="true" />
                  {c.title}
                  {key ? <Kbd>{label(key)}</Kbd> : null}
                </CommandItem>
              );
            })}
        </CommandGroup>
        <CommandGroup heading="Appearance">
          <CommandItem value="toggle theme dark light" onSelect={() => run(toggleTheme)}>
            <Moon className="m-icon m-icon--sm" aria-hidden="true" />
            Toggle theme
          </CommandItem>
        </CommandGroup>
      </CommandList>
    </CommandDialog>
  );
}
