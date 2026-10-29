/** Open objects of the connection as tabs (`Main` board). A dot marks tabs with staged edits. */
import * as React from "react";
import { Link, useNavigate } from "@tanstack/react-router";
import { Activity, Eye, History, LayoutGrid, Plus, SquareTerminal, Table2, Users, X } from "lucide-react";

import { objectPath } from "@/lib/format";
import { keyLabel } from "@/lib/keys";
import { cn } from "@/lib/utils";
import { Button } from "@/ui/Button";
import { Kbd } from "@/ui/controls";
import { hasChanges, tableKey, useChanges } from "@/features/table/changes.store";

import { useWorkspace } from "../workspace/context";
import { closeTab, newQueryTab } from "./tabActions";
import { tabLink, useActiveTab, useConnectionTabs, type Tab } from "./tabs.store";

export function TabIcon({ tab }: { tab: Tab }) {
  const cls = "m-icon m-icon--sm";
  switch (tab.kind) {
    case "table":
      return tab.objectKind === "view" ? <Eye className={cls} aria-hidden="true" /> : <Table2 className={cls} aria-hidden="true" />;
    case "query":
      return <SquareTerminal className={cls} aria-hidden="true" />;
    case "monitor":
      return <Activity className={cls} aria-hidden="true" />;
    case "designer":
      return <LayoutGrid className={cls} aria-hidden="true" />;
    case "roles":
      return <Users className={cls} aria-hidden="true" />;
    case "activity":
      return <History className={cls} aria-hidden="true" />;
  }
}

export function TabStrip() {
  const { connectionId } = useWorkspace();
  const tabs = useConnectionTabs(connectionId);
  const active = useActiveTab(connectionId);
  const staged = useChanges((s) => s.tables);
  const navigate = useNavigate();
  const navRef = React.useRef<HTMLElement>(null);

  // Keep the active tab in sight when the keyboard moves to one scrolled out of view.
  React.useEffect(() => {
    navRef.current?.querySelector<HTMLElement>(".m-tab.is-active")?.scrollIntoView({ block: "nearest", inline: "nearest" });
  }, [active?.id, tabs.length]);

  const close = (tab: Tab) => closeTab(navigate, connectionId, tab.id);

  return (
    <nav ref={navRef} className="m-tabs" aria-label="Open objects" style={{ height: "var(--size-header)", flex: "none", overflowX: "auto", overflowY: "hidden" }}>
      {tabs.map((tab, i) => {
        const dirty = tab.kind === "table" && hasChanges(staged[tableKey(connectionId, tab.database, tab.schema, tab.table)]);
        return (
          <Link
            key={tab.id}
            {...tabLink(connectionId, tab)}
            className={cn("m-tab", active?.id === tab.id && "is-active", dirty && "is-dirty")}
            aria-current={active?.id === tab.id ? "page" : undefined}
            title={`${tab.kind === "table" ? objectPath(tab.database ?? "", tab.schema ?? "", tab.table ?? "") : tab.title}${i < 8 ? ` (${keyLabel(`mod+${i + 1}`)})` : ""}`}
            onAuxClick={(e) => {
              if (e.button === 1) {
                e.preventDefault();
                close(tab);
              }
            }}
          >
            <TabIcon tab={tab} />
            <span className="m-tab__name">{tab.title}</span>
            <span
              role="button"
              tabIndex={-1}
              className="m-tab__close"
              aria-label={`Close ${tab.title}`}
              title={`Close (${keyLabel("mod+w")})`}
              onClick={(e) => {
                e.preventDefault();
                e.stopPropagation();
                close(tab);
              }}
            >
              <X className="m-icon m-icon--sm" aria-hidden="true" />
            </span>
          </Link>
        );
      })}
      <div className="m-tabs__end">
        <Button variant="ghost" size="xs" icon aria-label="New query" onClick={() => newQueryTab(navigate, connectionId)}>
          <Plus className="m-icon m-icon--sm" aria-hidden="true" />
        </Button>
        <Kbd>{keyLabel("mod+t")}</Kbd>
      </div>
    </nav>
  );
}
