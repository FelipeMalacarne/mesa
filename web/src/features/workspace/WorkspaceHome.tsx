/**
 * What a workspace shows with no tab open: the server at a glance, its databases, shortcuts to
 * the tools, and what was run recently.
 */
import * as React from "react";
import { Link, useNavigate } from "@tanstack/react-router";
import { Activity, Database, History, LayoutGrid, SquareTerminal, Users } from "lucide-react";

import { useListAudit } from "@/lib/api/changes/changes";
import { useListDatabases } from "@/lib/api/explore/explore";
import { useGetMonitorOverview } from "@/lib/api/monitor/monitor";
import { useListHistory } from "@/lib/api/query/query";
import { bytes, relativeTime } from "@/lib/format";
import { useStatusFacts } from "@/app/status.store";
import { EnvTag } from "@/ui/controls";
import { useTabs, tabLink } from "@/features/explorer/tabs.store";

import { useWorkspace } from "./context";

export function WorkspaceHome() {
  const { connection, connectionId } = useWorkspace();
  const navigate = useNavigate();
  const overview = useGetMonitorOverview(connectionId, connection.database, { query: { retry: false } });
  const dbs = useListDatabases(connectionId);
  const history = useListHistory(connectionId, { limit: 6 });
  const audit = useListAudit(connectionId, { limit: 6 });
  useStatusFacts(overview.data ? [`${connection.engine === "postgres" ? "PostgreSQL" : connection.engine} ${overview.data.version}`, `as ${overview.data.role}`] : []);

  // Arriving on the workspace root with tabs open returns to the active one.
  React.useEffect(() => {
    const conn = useTabs.getState().byConnection[connectionId];
    const active = conn?.tabs.find((t) => t.id === conn.activeId);
    if (active) void navigate({ ...tabLink(connectionId, active), replace: true });
  }, [connectionId, navigate]);

  const newQuery = () => {
    const tab = useTabs.getState().newQuery(connectionId);
    void navigate(tabLink(connectionId, tab));
  };

  const tools = [
    { label: "New query", hint: "Run SQL with results, EXPLAIN and history", icon: SquareTerminal, onClick: newQuery },
    { label: "Schema designer", hint: "ERD and DBML, reviewed migrations", icon: LayoutGrid, onClick: () => void navigate({ to: "/c/$connectionId/designer", params: { connectionId }, search: {} }) },
    { label: "Monitor", hint: "Throughput, connections, cache, engine panel", icon: Activity, onClick: () => void navigate({ to: "/c/$connectionId/monitor", params: { connectionId }, search: { view: "overview" } }) },
    { label: "Users & roles", hint: "Logins and groups on this server", icon: Users, onClick: () => void navigate({ to: "/c/$connectionId/roles", params: { connectionId } }) },
  ];

  return (
    <div style={{ flex: 1, minHeight: 0, overflow: "auto" }}>
      <div className="m-page" style={{ maxWidth: 1080 }}>
        <header className="m-page__head">
          <div className="m-page__title">
            <h1 className="text-page-title">{connection.name}</h1>
            <EnvTag env={connection.environment} />
          </div>
          <span className="m-muted m-mono">
            {connection.user}@{connection.host}:{connection.port}
          </span>
        </header>
        <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(220px, 1fr))", gap: 12 }}>
          {tools.map((t) => (
            <button key={t.label} type="button" className="m-card" style={{ textAlign: "left", padding: 16, display: "grid", gap: 6, cursor: "pointer" }} onClick={t.onClick}>
              <t.icon className="m-icon" style={{ color: "var(--primary)" }} aria-hidden="true" />
              <span className="text-ui-strong">{t.label}</span>
              <span className="m-help">{t.hint}</span>
            </button>
          ))}
        </div>
        <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(320px, 1fr))", gap: 12 }}>
          <section className="m-card">
            <div className="m-card__head">
              <h2 className="m-card__title">Databases</h2>
              <span className="m-badge m-badge--count">{dbs.data?.length ?? 0}</span>
            </div>
            <div className="m-card__body">
              <div className="m-grid-wrap">
                <table className="m-grid m-grid--ui" style={{ width: "100%" }}>
                  <tbody>
                    {(dbs.data ?? []).map((d) => (
                      <tr key={d.name} className="is-link" onClick={() => void navigate({ to: "/c/$connectionId/designer", params: { connectionId }, search: { database: d.name } })}>
                        <td>
                          <span className="m-row" style={{ flexWrap: "nowrap" }}>
                            <Database className="m-icon m-icon--sm m-muted" aria-hidden="true" />
                            {d.name}
                          </span>
                        </td>
                        <td className="m-muted">{d.owner}</td>
                        <td className="m-mono is-num">{bytes(d.sizeBytes)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          </section>
          <section className="m-card">
            <div className="m-card__head">
              <h2 className="m-card__title">Recent changes</h2>
              <Link to="/c/$connectionId/activity" params={{ connectionId }} className="m-btn m-btn--ghost m-btn--xs" style={{ marginLeft: "auto" }}>
                <History className="m-icon m-icon--sm" aria-hidden="true" />
                Activity
              </Link>
            </div>
            <div className="m-card__body" style={{ display: "grid", gap: 8 }}>
              {(audit.data ?? []).length === 0 ? <p className="m-help">Nothing written through Mesa yet.</p> : null}
              {(audit.data ?? []).map((a) => (
                <div key={a.id} className="m-row" style={{ flexWrap: "nowrap" }}>
                  <span className={`m-dot ${a.status === "applied" ? "m-dot--online" : a.status === "failed" ? "m-dot--down" : "m-dot--pending"}`} />
                  <span style={{ flex: 1, minWidth: 0, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{a.summary}</span>
                  <span className="m-help">{relativeTime(a.startedAt)}</span>
                </div>
              ))}
            </div>
          </section>
          <section className="m-card">
            <div className="m-card__head">
              <h2 className="m-card__title">Recent queries</h2>
            </div>
            <div className="m-card__body" style={{ display: "grid", gap: 8 }}>
              {(history.data ?? []).length === 0 ? <p className="m-help">Queries you run appear here.</p> : null}
              {(history.data ?? []).map((h) => (
                <button
                  key={h.id}
                  type="button"
                  className="m-row"
                  style={{ flexWrap: "nowrap", textAlign: "left", width: "100%", minWidth: 0 }}
                  title={h.sql}
                  onClick={() => {
                    const tab = useTabs.getState().newQuery(connectionId);
                    useTabs.getState().update(connectionId, tab.id, { sql: h.sql, database: h.database });
                    void navigate(tabLink(connectionId, tab));
                  }}
                >
                  <span className="m-mono" style={{ flex: 1, minWidth: 0, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                    {h.sql}
                  </span>
                  <span className="m-help">{relativeTime(h.ranAt)}</span>
                </button>
              ))}
            </div>
          </section>
        </div>
      </div>
    </div>
  );
}
