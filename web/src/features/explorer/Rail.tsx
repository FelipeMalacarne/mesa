/** The left rail: connection switcher, object filter, nested tree and the connection's tools. */
import * as React from "react";
import { Link, useNavigate, useRouterState } from "@tanstack/react-router";
import { Activity, ChevronsUpDown, Database, History, LayoutGrid, Network, Search, SquareTerminal, Users } from "lucide-react";

import { useListConnections } from "@/lib/api/connections/connections";
import { cn } from "@/lib/utils";
import { EnvTag, Kbd } from "@/ui/controls";
import { Popover, PopoverContent, PopoverTrigger } from "@/ui/primitives/popover";
import { useUi } from "@/app/ui.store";
import { shortcut } from "@/app/keys";

import { useWorkspace } from "../workspace/context";
import { NewDatabaseDialog } from "./NewDatabaseDialog";
import { ObjectTree } from "./ObjectTree";
import { focusFirstItem, openFirstObject } from "./treeKeys";

export function Rail() {
  const { connection, connectionId } = useWorkspace();
  const collapsed = useUi((s) => s.railCollapsed);
  const [filter, setFilter] = React.useState("");
  const [newDb, setNewDb] = React.useState(false);
  const filterRef = React.useRef<HTMLInputElement>(null);
  const path = useRouterState({ select: (s) => s.location.pathname });
  const searchView = useRouterState({ select: (s) => (s.location.search as { view?: string }).view });

  React.useEffect(() => {
    const onFocus = () => filterRef.current?.focus();
    window.addEventListener("mesa:focus-filter", onFocus);
    return () => window.removeEventListener("mesa:focus-filter", onFocus);
  }, []);

  const tools = [
    { to: "/c/$connectionId/monitor" as const, label: "Monitor", icon: Activity, active: path.endsWith("/monitor") && searchView !== "sessions", search: { view: "overview" as const } },
    { to: "/c/$connectionId/monitor" as const, label: "Sessions", icon: Network, active: path.endsWith("/monitor") && searchView === "sessions", search: { view: "sessions" as const } },
    { to: "/c/$connectionId/roles" as const, label: "Users & roles", icon: Users, active: path.endsWith("/roles") },
    { to: "/c/$connectionId/designer" as const, label: "Schema designer", icon: LayoutGrid, active: path.endsWith("/designer") },
    { to: "/c/$connectionId/sql" as const, label: "SQL editor", icon: SquareTerminal, active: path.endsWith("/sql") },
    { to: "/c/$connectionId/activity" as const, label: "Activity", icon: History, active: path.endsWith("/activity") },
  ];

  return (
    <aside className={cn("m-rail", collapsed && "m-rail--collapsed")} aria-label="Objects" style={{ minHeight: 0 }}>
      <ConnectionSwitcher collapsed={collapsed} />
      {!collapsed ? (
        <>
          <div style={{ padding: "0 8px 4px" }}>
            <div className="m-input-group">
              <Search className="m-icon m-icon--sm" aria-hidden="true" />
              <input
                ref={filterRef}
                className="m-input m-input--sm"
                placeholder="Filter objects"
                aria-label="Filter objects"
                value={filter}
                onChange={(e) => setFilter(e.target.value)}
                onKeyDown={(e) => {
                  const tree = e.currentTarget.closest(".m-rail")?.querySelector<HTMLElement>(".m-tree") ?? null;
                  if (e.key === "Escape") setFilter("");
                  else if (e.key === "ArrowDown") {
                    e.preventDefault();
                    focusFirstItem(tree);
                  } else if (e.key === "Enter" && filter.trim()) {
                    e.preventDefault();
                    openFirstObject(tree);
                  }
                }}
              />
              <Kbd>{shortcut("KeyP")}</Kbd>
            </div>
          </div>
          <ObjectTree filter={filter.trim()} onNewDatabase={() => setNewDb(true)} />
        </>
      ) : (
        <div style={{ flex: 1 }} />
      )}
      <nav className="m-rail__foot" aria-label="Connection tools">
        {tools.map((t) => (
          <Link
            key={t.label}
            to={t.to}
            params={{ connectionId }}
            search={t.search as never}
            className={cn("m-tree__item", t.active && "is-selected")}
            title={collapsed ? t.label : undefined}
            aria-label={collapsed ? t.label : undefined}
          >
            <t.icon className="m-icon m-icon--sm" aria-hidden="true" />
            {!collapsed ? <span className="m-tree__name">{t.label}</span> : null}
          </Link>
        ))}
      </nav>
      <NewDatabaseDialog open={newDb} onOpenChange={setNewDb} connection={connection} />
    </aside>
  );
}

function ConnectionSwitcher({ collapsed }: { collapsed: boolean }) {
  const { connection } = useWorkspace();
  const all = useListConnections({ query: { staleTime: 30_000 } });
  const navigate = useNavigate();
  const [open, setOpen] = React.useState(false);
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button className="m-conn" type="button" aria-haspopup="listbox" aria-label="Switch connection" title={collapsed ? connection.name : undefined}>
          <span className="m-conn__icon">
            <Database className="m-icon m-icon--sm" aria-hidden="true" />
          </span>
          {!collapsed ? (
            <>
              <span style={{ minWidth: 0, flex: 1, display: "flex", flexDirection: "column" }}>
                <span style={{ display: "flex", alignItems: "center", gap: 6, minWidth: 0 }}>
                  <span className="m-conn__name">{connection.name}</span>
                  <EnvTag env={connection.environment} />
                </span>
                <span className="m-conn__sub">
                  {connection.host}:{connection.port}
                </span>
              </span>
              <ChevronsUpDown className="m-icon m-icon--sm m-muted" aria-hidden="true" />
            </>
          ) : null}
        </button>
      </PopoverTrigger>
      <PopoverContent style={{ width: 280 }} role="listbox">
        {(all.data ?? []).map((c) => (
          <button
            key={c.id}
            type="button"
            role="option"
            aria-selected={c.id === connection.id}
            className={cn("m-menu__item", c.id === connection.id && "is-selected")}
            onClick={() => {
              setOpen(false);
              void navigate({ to: "/c/$connectionId", params: { connectionId: c.id } });
            }}
          >
            <Database className="m-icon m-icon--sm" aria-hidden="true" />
            <span style={{ flex: 1, minWidth: 0, overflow: "hidden", textOverflow: "ellipsis" }}>{c.name}</span>
            <EnvTag env={c.environment} />
          </button>
        ))}
        <div className="m-menu__sep" />
        <button
          type="button"
          className="m-menu__item"
          onClick={() => {
            setOpen(false);
            void navigate({ to: "/" });
          }}
        >
          <LayoutGrid className="m-icon m-icon--sm" aria-hidden="true" />
          All connections
        </button>
      </PopoverContent>
    </Popover>
  );
}
