/**
 * The left rail: connection switcher, object search, nested tree and the connection's tools.
 * With text in the search the rail lists the matches (ObjectSearch) in place of the tree.
 * ⌘P focuses the search, ⌘⇧E the tree; ↓ from an empty search goes into the tree and Esc
 * from it goes back to where focus was.
 */
import * as React from "react";
import { Link, useNavigate, useRouterState } from "@tanstack/react-router";
import { Activity, ChevronsUpDown, Database, History, LayoutGrid, Network, Search, SquareTerminal, Users } from "lucide-react";

import { useListConnections } from "@/lib/api/connections/connections";
import { cn, isMac } from "@/lib/utils";
import { EnvTag, Kbd } from "@/ui/controls";
import { Popover, PopoverContent, PopoverTrigger } from "@/ui/primitives/popover";
import { useUi } from "@/app/ui.store";
import { shortcut } from "@/app/keys";

import { useWorkspace } from "../workspace/context";
import { NewDatabaseDialog } from "./NewDatabaseDialog";
import { ObjectSearch, type ObjectSearchHandle } from "./ObjectSearch";
import { ObjectTree } from "./ObjectTree";

export function Rail() {
  const { connection, connectionId } = useWorkspace();
  const collapsed = useUi((s) => s.railCollapsed);
  const [filter, setFilterText] = React.useState("");
  const [active, setActive] = React.useState(0);
  const [newDb, setNewDb] = React.useState(false);
  const filterRef = React.useRef<HTMLInputElement>(null);
  const treeRef = React.useRef<HTMLDivElement>(null);
  const searchRef = React.useRef<ObjectSearchHandle>(null);
  const returnFocus = React.useRef<HTMLElement | null>(null);
  const path = useRouterState({ select: (s) => s.location.pathname });
  const searchView = useRouterState({ select: (s) => (s.location.search as { view?: string }).view });
  const query = filter.trim();
  const listId = React.useId();

  const setFilter = (text: string) => {
    setFilterText(text);
    setActive(0);
  };

  // ⌘P and ⌘⇧E (commands.ts) ask through the ui store; a request older than this rail is not ours.
  const focusRequest = useUi((s) => s.focusRequest);
  const seen = React.useRef(useUi.getState().focusRequest?.n ?? 0);
  React.useEffect(() => {
    if (!focusRequest || focusRequest.n === seen.current) return;
    seen.current = focusRequest.n;
    const from = document.activeElement as HTMLElement | null;
    if (from && !from.closest(".m-rail")) returnFocus.current = from;
    if (focusRequest.target === "filter") {
      filterRef.current?.focus();
      filterRef.current?.select();
    } else {
      if (query) setFilter("");
      requestAnimationFrame(() => treeRef.current?.focus());
    }
  }, [focusRequest]); // eslint-disable-line react-hooks/exhaustive-deps

  const leave = () => {
    const back = returnFocus.current;
    returnFocus.current = null;
    if (back?.isConnected) back.focus();
    else document.querySelector<HTMLElement>("main")?.focus();
  };

  const onFilterKey = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.metaKey || e.altKey) return;
    // Ctrl+P and Ctrl+K are ⌘P and ⌘K outside macOS; there only Ctrl+N and Ctrl+J move.
    const down = e.key === "ArrowDown" || (e.ctrlKey && (e.key === "n" || e.key === "j"));
    const up = e.key === "ArrowUp" || (e.ctrlKey && isMac && (e.key === "p" || e.key === "k"));
    let handled = true;
    if (!query) {
      if (down) treeRef.current?.focus();
      else if (e.key === "Escape") leave();
      else handled = false;
    } else if (down) searchRef.current?.move(1);
    else if (up) searchRef.current?.move(-1);
    else if (e.key === "PageDown") searchRef.current?.move(10);
    else if (e.key === "PageUp") searchRef.current?.move(-10);
    else if (e.key === "Enter") handled = searchRef.current?.open(e.shiftKey) ?? false;
    else if (e.key === "Escape") setFilter("");
    else handled = false;
    if (handled) e.preventDefault();
  };

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
                placeholder="Find tables, views, columns…"
                aria-label="Find objects"
                role="combobox"
                aria-expanded={!!query}
                aria-controls={query ? listId : undefined}
                aria-activedescendant={query ? `${listId}-${active}` : undefined}
                aria-autocomplete="list"
                value={filter}
                onChange={(e) => setFilter(e.target.value)}
                onKeyDown={onFilterKey}
              />
              {filter ? null : <Kbd>{shortcut("KeyP")}</Kbd>}
            </div>
          </div>
          {query ? (
            <ObjectSearch ref={searchRef} id={listId} query={query} active={active} onActive={setActive} onDone={() => setFilter("")} />
          ) : (
            <ObjectTree treeRef={treeRef} onNewDatabase={() => setNewDb(true)} onSearch={() => filterRef.current?.focus()} />
          )}
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
