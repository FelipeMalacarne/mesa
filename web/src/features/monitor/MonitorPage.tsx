/**
 * Monitor (`Monitor` board): general metrics every engine reports, then the engine's own panel
 * (PostgreSQL: replication, vacuum, bloat, statements…). Sessions lists backends with cancel and
 * terminate. Samples come every two seconds from the server's ring buffer.
 */
import * as React from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { Activity, Ban, Database as DatabaseIcon, Pause, Play, Power, RefreshCw } from "lucide-react";
import { toast } from "sonner";

import { useListDatabases } from "@/lib/api/explore/explore";
import type { ActiveSession, Panel, Snapshot } from "@/lib/api/mesaAPI.schemas";
import { getListSessionsQueryKey, getMonitorSnapshot, killSession, useGetEnginePanel, useGetMonitorOverview, useListSessions } from "@/lib/api/monitor/monitor";
import { errorMessage } from "@/lib/api/fetch";
import { bytes, clockTime, compact, duration, engineLabel, nsToMs, plural, relativeTime } from "@/lib/format";
import { cn } from "@/lib/utils";
import { useStatusFacts } from "@/app/status.store";
import { Button } from "@/ui/Button";
import { LineChart, Sparkline } from "@/ui/charts";
import { EmptyState, Select } from "@/ui/controls";
import { Dialog, DialogBody, DialogContent, DialogDescription, DialogFoot, DialogHead, DialogTitle } from "@/ui/primitives/dialog";
import { useTabs } from "@/features/explorer/tabs.store";

import { useWorkspace } from "../workspace/context";

export function MonitorPage({ view, database: requested }: { view: "overview" | "sessions"; database?: string }) {
  const { connectionId, connection } = useWorkspace();
  const navigate = useNavigate();
  const database = requested ?? connection.database;
  const dbs = useListDatabases(connectionId);
  const [paused, setPaused] = React.useState(false);
  const [range, setRange] = React.useState<Range>("live");

  React.useEffect(() => {
    useTabs.getState().open(connectionId, { id: "monitor", kind: "monitor", title: view === "sessions" ? "Sessions" : "Monitor", view });
  }, [connectionId, view]);

  const go = (patch: { view?: "overview" | "sessions"; database?: string }) =>
    void navigate({ to: "/c/$connectionId/monitor", params: { connectionId }, search: { view: patch.view ?? view, ...(patch.database ?? requested ? { database: patch.database ?? requested } : {}) } });

  return (
    <>
      <div className="m-toolbar" style={{ height: "var(--size-toolbar)", flex: "none" }}>
        <nav className="m-seg" aria-label="Monitor view">
          <button type="button" className={cn("m-seg__item", view === "overview" && "is-active")} onClick={() => go({ view: "overview" })}>
            <Activity className="m-icon m-icon--sm" aria-hidden="true" />
            Overview
          </button>
          <button type="button" className={cn("m-seg__item", view === "sessions" && "is-active")} onClick={() => go({ view: "sessions" })}>
            Sessions
          </button>
        </nav>
        <span className="m-sep" style={{ height: 20, alignSelf: "center" }} />
        <span style={{ display: "inline-flex", alignItems: "center", gap: 6 }}>
          <DatabaseIcon className="m-icon m-icon--sm m-muted" aria-hidden="true" />
          <Select size="sm" mono aria-label="Database" value={database} options={(dbs.data ?? [{ name: database }]).map((d) => ({ value: d.name, label: d.name }))} onChange={(v) => go({ database: v })} />
        </span>
        <span className="m-spacer" />
        {view === "overview" ? (
          <div className="m-seg" role="tablist" aria-label="Time range">
            {RANGES.map((r) => (
              <button key={r.value} type="button" role="tab" aria-selected={range === r.value} className={cn("m-seg__item", range === r.value && "is-active")} onClick={() => setRange(r.value)}>
                {r.label}
              </button>
            ))}
          </div>
        ) : null}
        <span className="m-help">{paused ? "Paused" : range === "live" || view === "sessions" ? "Live · every 2 s" : "Every 30 s"}</span>
        <Button variant="ghost" size="sm" icon aria-label={paused ? "Resume" : "Pause"} onClick={() => setPaused(!paused)}>
          {paused ? <Play className="m-icon m-icon--sm" aria-hidden="true" /> : <Pause className="m-icon m-icon--sm" aria-hidden="true" />}
        </Button>
      </div>
      {view === "overview" ? <Overview database={database} paused={paused} range={range} /> : <Sessions database={database} paused={paused} />}
    </>
  );
}

type Range = "live" | "15m" | "1h" | "6h" | "24h" | "7d";

const RANGES: { value: Range; label: string }[] = [
  { value: "live", label: "Live" },
  { value: "15m", label: "15m" },
  { value: "1h", label: "1h" },
  { value: "6h", label: "6h" },
  { value: "24h", label: "24h" },
  { value: "7d", label: "7d" },
];

function Overview({ database, paused, range }: { database: string; paused: boolean; range: Range }) {
  const { connectionId, connection } = useWorkspace();
  const overview = useGetMonitorOverview(connectionId, database);
  const snap = useQuery({
    queryKey: ["snapshot", connectionId, database, range],
    queryFn: () => getMonitorSnapshot(connectionId, database, { range }),
    refetchInterval: paused ? false : range === "live" ? 2000 : 30_000,
    retry: false,
    placeholderData: (prev) => prev,
  });
  const panel = useGetEnginePanel(connectionId, database, { query: { refetchInterval: paused ? false : 15_000, retry: false } });
  useStatusFacts(overview.data ? [`${engineLabel(connection.engine)} ${overview.data.version}`, `up ${relativeTime(overview.data.startedAt).replace(" ago", "")}`] : []);

  if (snap.isError) return <div style={{ padding: 24 }}><EmptyState title="Metrics unavailable" description={errorMessage(snap.error)} /></div>;
  const s = snap.data;
  return (
    <div style={{ flex: 1, minHeight: 0, overflow: "auto" }}>
      <div style={{ padding: "16px 24px", display: "flex", flexDirection: "column", gap: 24, maxWidth: 1400 }}>
        <section style={{ display: "flex", flexDirection: "column", gap: 12 }} aria-labelledby="general-h">
          <h2 id="general-h" className="m-section-title">
            General
          </h2>
          {s ? <GeneralTiles s={s} /> : <p className="m-help">Sampling…</p>}
          {s ? <Charts s={s} range={range} /> : null}
        </section>
        <section style={{ display: "flex", flexDirection: "column", gap: 12 }} aria-labelledby="engine-h">
          <h2 id="engine-h" className="m-section-title">
            {panel.data?.title ?? "Engine"} <span className="m-help" style={{ fontWeight: 400 }}>engine-specific</span>
          </h2>
          {panel.isError ? <p className="m-help m-help--error">{errorMessage(panel.error)}</p> : null}
          {panel.data ? <EnginePanel panel={panel.data} /> : null}
        </section>
      </div>
    </div>
  );
}

function statusDot(status?: string) {
  return status === "critical" ? "m-dot--down" : status === "warning" ? "m-dot--degraded" : status === "ok" ? "m-dot--online" : "";
}

function GeneralTiles({ s }: { s: Snapshot }) {
  const m = s.metrics;
  const used = m.connectionsActive + m.connectionsIdle + m.connectionsIdleTx;
  const usage = m.connectionsMax ? used / m.connectionsMax : 0;
  const cache = m.cacheHitRatio * 100;
  const tiles: { label: string; value: string; unit?: string; foot: React.ReactNode; spark?: number[]; series?: 1 | 2 | 3 }[] = [
    { label: "Transactions / s", value: s.tps.toFixed(s.tps < 10 ? 1 : 0), foot: `${compact(m.commits)} commits · ${compact(m.rollbacks)} rollbacks`, spark: s.series.map((p) => p.tps), series: 1 },
    {
      label: "Connections",
      value: String(used),
      unit: `/ ${m.connectionsMax}`,
      foot: (
        <>
          <span className={cn("m-dot", usage > 0.9 ? "m-dot--down" : usage > 0.75 ? "m-dot--degraded" : "m-dot--online")} />
          {m.connectionsActive} active · {m.connectionsIdleTx} idle in tx
        </>
      ),
      spark: s.series.map((p) => p.active + p.idle + p.idleTx),
      series: 2,
    },
    {
      label: "Cache hit ratio",
      value: cache.toFixed(cache >= 99.95 ? 0 : 1),
      unit: "%",
      foot: (
        <>
          <span className={cn("m-dot", cache < 90 ? "m-dot--down" : cache < 99 ? "m-dot--degraded" : "m-dot--online")} />
          {cache < 99 ? "below 99%: working set may not fit" : "working set fits in memory"}
        </>
      ),
    },
    { label: "Database size", value: bytes(m.databaseSizeBytes).split(" ")[0]!, unit: bytes(m.databaseSizeBytes).split(" ")[1], foot: s.sizeDelta7d ? `${s.sizeDelta7d > 0 ? "+" : ""}${bytes(Math.abs(s.sizeDelta7d))} in 7 days` : "since Mesa started watching" },
    {
      label: "Locks waiting",
      value: String(m.locksWaiting),
      foot: (
        <>
          <span className={cn("m-dot", m.locksWaiting > 5 ? "m-dot--down" : m.locksWaiting > 0 ? "m-dot--degraded" : "m-dot--online")} />
          {m.locksWaiting ? "sessions blocked by another" : "nothing blocked"}
        </>
      ),
    },
    {
      label: "Longest transaction",
      value: m.longestTxSeconds >= 60 ? (m.longestTxSeconds / 60).toFixed(1) : Math.max(0, Math.round(m.longestTxSeconds)).toString(),
      unit: m.longestTxSeconds >= 60 ? "min" : "s",
      foot: (
        <>
          <span className={cn("m-dot", m.longestTxSeconds > 600 ? "m-dot--down" : m.longestTxSeconds > 60 ? "m-dot--degraded" : "m-dot--online")} />
          {m.longestTxSeconds > 60 ? "holds back vacuum" : "short transactions"}
        </>
      ),
    },
    { label: "Round trip", value: s.latencyMs.toFixed(s.latencyMs < 10 ? 1 : 0), unit: "ms", foot: "from Mesa to the server", spark: s.series.map((p) => p.latencyMs), series: 3 },
  ];
  return (
    <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(200px, 1fr))", gap: 12 }}>
      {tiles.map((t) => (
        <div key={t.label} className="m-stat">
          <span className="m-stat__label">{t.label}</span>
          <span className="m-stat__value">
            {t.value}
            {t.unit ? <span className="m-stat__unit">{t.unit}</span> : null}
          </span>
          <span className="m-stat__foot">{t.foot}</span>
          {t.spark ? <Sparkline values={t.spark} series={t.series} /> : null}
        </div>
      ))}
    </div>
  );
}

function Charts({ s, range }: { s: Snapshot; range: Range }) {
  const labels = s.series.map((p) => (range === "24h" || range === "7d" ? dayTime(p.at) : clockTime(p.at)));
  if (s.series.length < 2) {
    return (
      <p className="m-help">
        {range === "live" ? "Collecting samples…" : "No history for this range yet. Mesa keeps sampling a database once its Monitor has been opened, while the server or desktop app runs."}
      </p>
    );
  }
  return (
    <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(420px, 1fr))", gap: 12 }}>
      <section className="m-card">
        <div className="m-card__head">
          <h3 className="m-card__title">Transactions per second</h3>
        </div>
        <div className="m-card__body">
          <LineChart ariaLabel="Transactions per second" series={[{ name: "tps", values: s.series.map((p) => p.tps) }]} labels={labels} format={(v) => (v < 10 ? v.toFixed(1) : String(Math.round(v)))} />
        </div>
      </section>
      <section className="m-card">
        <div className="m-card__head">
          <h3 className="m-card__title">Connections by state</h3>
          <span className="m-legend" style={{ marginLeft: "auto" }}>
            <span className="m-legend__key" style={{ ["--k" as string]: "var(--chart-1)" }}>active</span>
            <span className="m-legend__key" style={{ ["--k" as string]: "var(--chart-2)" }}>idle</span>
            <span className="m-legend__key" style={{ ["--k" as string]: "var(--chart-3)" }}>idle in tx</span>
          </span>
        </div>
        <div className="m-card__body">
          <LineChart
            ariaLabel="Connections by state: active, idle, idle in transaction"
            series={[
              { name: "active", values: s.series.map((p) => p.active) },
              { name: "idle", values: s.series.map((p) => p.idle) },
              { name: "idle in tx", values: s.series.map((p) => p.idleTx) },
            ]}
            labels={labels}
          />
        </div>
      </section>
    </div>
  );
}

function EnginePanel({ panel }: { panel: Panel }) {
  return (
    <>
      {(panel.tiles?.length ?? 0) > 0 ? (
        <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(200px, 1fr))", gap: 12 }}>
          {panel.tiles!.map((t) => (
            <div key={t.label} className="m-stat">
              <span className="m-stat__label">{t.label}</span>
              <span className="m-stat__value">
                {t.value}
                {t.unit ? <span className="m-stat__unit">{t.unit}</span> : null}
              </span>
              {t.note || t.status ? (
                <span className="m-stat__foot">
                  {t.status ? <span className={cn("m-dot", statusDot(t.status))} /> : null}
                  {t.note}
                </span>
              ) : null}
            </div>
          ))}
        </div>
      ) : null}
      {(panel.meters?.length ?? 0) > 0 ? (
        <section className="m-card">
          <div className="m-card__body" style={{ display: "grid", gap: 16, paddingTop: 16 }}>
            {panel.meters!.map((m) => (
              <div key={m.label} className="m-meter-row">
                <div className="m-meter-row__top">
                  <span className="text-ui-strong">{m.label}</span>
                  {m.note ? <span className="m-help">{m.note}</span> : null}
                  <span className="m-meter-row__val">{m.text}</span>
                </div>
                <div className="m-meter" role="meter" aria-valuenow={m.value} aria-valuemin={0} aria-valuemax={m.max} aria-label={m.label}>
                  <div className={cn("m-meter__fill", m.status === "warning" && "m-meter__fill--warning", m.status === "critical" && "m-meter__fill--critical")} style={{ width: `${Math.min(100, (m.value / (m.max || 1)) * 100)}%` }} />
                </div>
              </div>
            ))}
          </div>
        </section>
      ) : null}
      {(panel.tables ?? []).map((t) => (
        <section key={t.title} className="m-card">
          <div className="m-card__head">
            <h3 className="m-card__title">{t.title}</h3>
            {t.source ? <span className="m-help m-mono">{t.source}</span> : null}
          </div>
          <div className="m-card__body">
            {t.rows.length === 0 ? (
              <p className="m-help">{t.empty ?? "Nothing to show."}</p>
            ) : (
              <div className="m-grid-wrap">
                <table className="m-grid m-grid--ui" style={{ width: "100%" }}>
                  <thead>
                    <tr>
                      {t.columns.map((c, i) => (
                        <th key={c} className={cn(t.numeric?.[i] && "is-num")}>
                          {c}
                        </th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {t.rows.map((r, ri) => (
                      <tr key={ri}>
                        {r.map((cell, ci) => (
                          <td key={ci} className={cn("m-mono", t.numeric?.[ci] && "is-num")} style={{ maxWidth: 520 }} title={cell}>
                            {cell}
                          </td>
                        ))}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </section>
      ))}
    </>
  );
}

function Sessions({ database, paused }: { database: string; paused: boolean }) {
  const { connectionId, connection } = useWorkspace();
  const qc = useQueryClient();
  const list = useListSessions(connectionId, database, { query: { refetchInterval: paused ? false : 3000 } });
  const [target, setTarget] = React.useState<{ s: ActiveSession; terminate: boolean } | null>(null);
  const [onlyActive, setOnlyActive] = React.useState(false);
  const rows = (list.data ?? []).filter((s) => !onlyActive || s.state === "active" || s.state.startsWith("idle in"));
  const blocking = rows.filter((s) => s.blocking).length;
  useStatusFacts(list.data ? [plural(list.data.length, "session"), ...(blocking ? [`${blocking} blocking`] : [])] : []);
  return (
    <div style={{ flex: 1, minHeight: 0, overflow: "auto", padding: "16px 24px", display: "flex", flexDirection: "column", gap: 12 }}>
      <div className="m-row">
        <h2 className="m-section-title">
          Sessions <span className="m-badge m-badge--count">{rows.length}</span>
        </h2>
        <span className="m-spacer" />
        <label className="m-row" style={{ cursor: "pointer" }}>
          <input type="checkbox" checked={onlyActive} onChange={() => setOnlyActive(!onlyActive)} style={{ accentColor: "var(--primary)" }} />
          <span className="m-help">Only active or in a transaction</span>
        </label>
        <Button variant="ghost" size="sm" icon aria-label="Refresh" onClick={() => void list.refetch()}>
          <RefreshCw className="m-icon m-icon--sm" aria-hidden="true" />
        </Button>
      </div>
      {list.isError ? <p className="m-help m-help--error">{errorMessage(list.error)}</p> : null}
      <div className="m-grid-wrap">
        <table className="m-grid m-grid--ui" style={{ width: "100%" }}>
          <thead>
            <tr>
              <th className="is-num">PID</th>
              <th>User</th>
              <th>Application</th>
              <th>Client</th>
              <th>State</th>
              <th>Waiting on</th>
              <th className="is-num">For</th>
              <th>Query</th>
              <th aria-label="Actions" />
            </tr>
          </thead>
          <tbody>
            {rows.length === 0 ? (
              <tr>
                <td colSpan={9} className="m-muted">
                  No other sessions on {database}.
                </td>
              </tr>
            ) : null}
            {rows.map((s) => (
              <tr key={s.pid}>
                <td className="m-mono is-num">{s.pid}</td>
                <td className="m-mono">{s.user}</td>
                <td className="m-muted" style={{ maxWidth: 160 }}>
                  {s.application}
                </td>
                <td className="m-mono m-muted">{s.clientAddr}</td>
                <td>
                  <span className="m-status">
                    <span className={cn("m-dot", s.state === "active" ? "m-dot--online" : s.state.startsWith("idle in") ? "m-dot--degraded" : "m-dot--unknown")} />
                    {s.state || "—"}
                    {s.blocking ? <span className="m-badge m-badge--outline" style={{ color: "var(--destructive)" }}>blocking</span> : null}
                  </span>
                </td>
                <td className="m-mono m-muted">{s.waitEvent}</td>
                <td className="m-mono is-num">{duration(nsToMs(s.duration))}</td>
                <td className="m-mono" style={{ maxWidth: 420 }} title={s.query}>
                  {s.query}
                </td>
                <td>
                  <span className="m-row" style={{ flexWrap: "nowrap", gap: 2 }}>
                    <Button variant="ghost" size="xs" icon aria-label={`Cancel query of ${s.pid}`} title="Cancel the running query" onClick={() => setTarget({ s, terminate: false })} disabled={s.state !== "active"}>
                      <Ban className="m-icon m-icon--sm" aria-hidden="true" />
                    </Button>
                    <Button variant="ghost" size="xs" icon aria-label={`Terminate ${s.pid}`} title="Terminate the session" onClick={() => setTarget({ s, terminate: true })}>
                      <Power className="m-icon m-icon--sm" aria-hidden="true" />
                    </Button>
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <Dialog open={!!target} onOpenChange={(o) => !o && setTarget(null)}>
        <DialogContent width={480} aria-describedby={undefined}>
          <DialogHead>
            <DialogTitle>
              {target?.terminate ? "Terminate" : "Cancel query of"} session {target?.s.pid}?
            </DialogTitle>
            <DialogDescription>
              {target?.terminate ? "The backend closes its connection; an open transaction rolls back." : "The running statement stops with an error; the session stays connected."}
              {connection.environment === "production" ? " This is Production." : ""}
            </DialogDescription>
          </DialogHead>
          <DialogBody>
            <code className="m-code" style={{ padding: "8px 12px", whiteSpace: "pre-wrap", maxHeight: 160 }}>
              {target?.s.query || "(no query)"}
            </code>
          </DialogBody>
          <DialogFoot>
            <span className="m-spacer" />
            <Button variant="outline" onClick={() => setTarget(null)}>
              Keep it
            </Button>
            <Button
              variant="destructive"
              onClick={async () => {
                if (!target) return;
                try {
                  await killSession(connectionId, database, target.s.pid, { terminate: target.terminate });
                  toast.success(target.terminate ? `Terminated ${target.s.pid}` : `Cancelled the query of ${target.s.pid}`);
                  setTarget(null);
                  await qc.invalidateQueries({ queryKey: getListSessionsQueryKey(connectionId, database) });
                } catch (e) {
                  toast.error(errorMessage(e));
                }
              }}
            >
              {target?.terminate ? "Terminate" : "Cancel query"}
            </Button>
          </DialogFoot>
        </DialogContent>
      </Dialog>
    </div>
  );
}

function dayTime(iso: string): string {
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString([], { month: "short", day: "numeric", hour: "2-digit", minute: "2-digit" });
}
