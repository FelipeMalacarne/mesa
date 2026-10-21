/**
 * The SQL editor (`SqlEditor` board). Queries run in a read-only transaction; a statement that
 * changes something is planned and reviewed first, then applied through the audited write path.
 */
import * as React from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { BookMarked, Copy, Database as DatabaseIcon, Play, Save, ShieldCheck, Trash2, X } from "lucide-react";
import { toast } from "sonner";

import { applyScript, planScript } from "@/lib/api/changes/changes";
import { getGetSchemaQueryKey, useListDatabases } from "@/lib/api/explore/explore";
import type { QueryResult, QueryRun, SavedQuery } from "@/lib/api/mesaAPI.schemas";
import { deleteSavedQuery, explainQuery, getListHistoryQueryKey, getListSavedQueriesQueryKey, runQuery, saveQuery, useListHistory, useListSavedQueries } from "@/lib/api/query/query";
import { errorMessage, isApiError } from "@/lib/api/fetch";
import { clockTime, duration, nsToMs, plural, relativeTime, thousands } from "@/lib/format";
import { isNull, textOf } from "@/lib/values";
import { cn, isCmd } from "@/lib/utils";
import { useStatusFacts } from "@/app/status.store";
import { Button } from "@/ui/Button";
import { Highlighted } from "@/ui/Code";
import { EmptyState, Field, Kbd, Select } from "@/ui/controls";
import { Dialog, DialogBody, DialogContent, DialogFoot, DialogHead, DialogTitle } from "@/ui/primitives/dialog";
import { ReviewDialog } from "@/features/changes/ReviewDialog";
import { tabLink, useTabs, type QueryTab } from "@/features/explorer/tabs.store";

import { useWorkspace } from "../workspace/context";
import { Editor } from "./Editor";
import { ResultGrid } from "./ResultGrid";
import { formatSql, isWrite, statementAt, statements, toCsv } from "./sql";

type Pane = "results" | "explain" | "messages";

interface Message {
  at: string;
  text: string;
  error?: boolean;
}

const LIMITS = ["100", "500", "1000", "5000"] as const;

export function SqlPage({ tabId }: { tabId?: string }) {
  const { connectionId, connection } = useWorkspace();
  const navigate = useNavigate();
  const tab = useTabs((s) => s.byConnection[connectionId]?.tabs.find((t) => t.id === tabId && t.kind === "query") as QueryTab | undefined);

  // /sql without a known tab opens a fresh one.
  React.useEffect(() => {
    if (tab) {
      useTabs.getState().activate(connectionId, tab.id);
      return;
    }
    const fresh = useTabs.getState().newQuery(connectionId);
    void navigate({ ...tabLink(connectionId, fresh), replace: true });
  }, [tab, connectionId, navigate]);

  if (!tab) return null;
  return <QueryTabView key={tab.id} tab={tab} connectionName={connection.name} />;
}

function QueryTabView({ tab }: { tab: QueryTab; connectionName: string }) {
  const { connectionId, connection } = useWorkspace();
  const qc = useQueryClient();
  const dbs = useListDatabases(connectionId);
  const database = tab.database ?? connection.database;
  const sql = tab.sql ?? "";
  const setSql = (v: string) => useTabs.getState().update(connectionId, tab.id, { sql: v });
  const [limit, setLimit] = React.useState<(typeof LIMITS)[number]>("500");
  const [pane, setPane] = React.useState<Pane>("results");
  const [result, setResult] = React.useState<QueryResult | null>(null);
  const [plan, setPlan] = React.useState<string[] | null>(null);
  const [messages, setMessages] = React.useState<Message[]>([]);
  const [running, setRunning] = React.useState(false);
  const [script, setScript] = React.useState<string | null>(null);
  const [library, setLibrary] = React.useState(false);
  const [saving, setSaving] = React.useState(false);
  const ta = React.useRef<HTMLTextAreaElement>(null);

  useStatusFacts(result ? [`${thousands(result.rows.length)} rows${result.truncated ? " (limited)" : ""}`, duration(nsToMs(result.duration))] : []);

  const log = (text: string, error = false) => setMessages((m) => [{ at: new Date().toISOString(), text, error }, ...m].slice(0, 200));

  /** The selection, else the statement under the cursor. */
  const target = (): string => {
    const el = ta.current;
    if (el && el.selectionEnd > el.selectionStart) return sql.slice(el.selectionStart, el.selectionEnd);
    return el ? statementAt(sql, el.selectionStart) : sql;
  };

  const run = async (text = target()) => {
    const stmts = statements(text);
    if (stmts.length === 0) return;
    if (stmts.some(isWrite)) {
      setScript(stmts.join(";\n") + ";");
      return;
    }
    setRunning(true);
    try {
      const res = await runQuery(connectionId, database, { sql: text, maxRows: Number(limit) });
      setResult(res);
      setPane("results");
      log(`${res.command || "OK"} ${res.columns.length ? thousands(res.rows.length) : thousands(res.rowsAffected)} · ${duration(nsToMs(res.duration))}${res.truncated ? " · limited" : ""}`);
    } catch (e) {
      log(errorMessage(e), true);
      setPane("messages");
      if (isApiError(e) && e.kind === "forbidden") setScript(text);
    } finally {
      setRunning(false);
      void qc.invalidateQueries({ queryKey: getListHistoryQueryKey(connectionId) });
    }
  };

  const explain = async (analyze: boolean) => {
    const text = target();
    if (!text.trim()) return;
    setRunning(true);
    try {
      const res = await explainQuery(connectionId, database, { sql: text, analyze });
      setPlan(res.lines);
      setPane("explain");
      log(`EXPLAIN${analyze ? " ANALYZE (rolled back)" : ""} · ${plural(res.lines.length, "line")}`);
    } catch (e) {
      log(errorMessage(e), true);
      setPane("messages");
    } finally {
      setRunning(false);
    }
  };

  const onKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (isCmd(e) && e.key === "Enter") {
      e.preventDefault();
      if (e.shiftKey) void explain(false);
      else void run();
    } else if (isCmd(e) && e.key.toLowerCase() === "s") {
      e.preventDefault();
      setSaving(true);
    }
  };

  const copyCsv = () => {
    if (!result) return;
    const csv = toCsv(
      result.columns.map((c) => c.name),
      result.rows.map((r) => r.map((v) => (isNull(v) ? "" : textOf(v)))),
    );
    void navigator.clipboard.writeText(csv).then(() => toast.success(`Copied ${plural(result.rows.length, "row")} as CSV`));
  };

  return (
    <>
      <div className="m-toolbar" style={{ height: "var(--size-toolbar)", flex: "none" }}>
        <Button variant="primary" size="sm" onClick={() => void run()} disabled={running}>
          <Play className="m-icon m-icon--sm" aria-hidden="true" />
          Run <Kbd>⌘↵</Kbd>
        </Button>
        <Button variant="outline" size="sm" onClick={() => void explain(false)} disabled={running} title="EXPLAIN (⇧⌘↵)">
          Explain
        </Button>
        <Button variant="ghost" size="sm" onClick={() => void explain(true)} disabled={running} title="Runs the query inside a transaction that is rolled back">
          Analyze
        </Button>
        <Button variant="ghost" size="sm" onClick={() => setSql(formatSql(sql))} disabled={!sql.trim()}>
          Format
        </Button>
        <span className="m-sep" style={{ height: 20, alignSelf: "center" }} />
        <span style={{ display: "inline-flex", alignItems: "center", gap: 6 }}>
          <DatabaseIcon className="m-icon m-icon--sm m-muted" aria-hidden="true" />
          <Select
            size="sm"
            mono
            aria-label="Database"
            value={database}
            options={(dbs.data ?? [{ name: database }]).map((d) => ({ value: d.name, label: d.name }))}
            onChange={(v) => useTabs.getState().update(connectionId, tab.id, { database: v })}
          />
        </span>
        <Select size="sm" aria-label="Row limit" value={limit} options={LIMITS.map((l) => ({ value: l, label: `Limit ${l}` }))} onChange={setLimit} />
        <span className="m-spacer" />
        <span className="m-status m-help" title="Queries run in a read-only transaction. A statement that writes is planned and reviewed first.">
          <ShieldCheck className="m-icon m-icon--sm" style={{ color: "var(--success)" }} aria-hidden="true" />
          Writes are reviewed
        </span>
        <Button variant="ghost" size="sm" onClick={() => setSaving(true)} disabled={!sql.trim()} title="Save (⌘S)">
          <Save className="m-icon m-icon--sm" aria-hidden="true" />
          Save
        </Button>
        <Button variant="ghost" size="sm" icon aria-label="History and saved queries" aria-pressed={library} onClick={() => setLibrary(!library)}>
          <BookMarked className="m-icon m-icon--sm" aria-hidden="true" />
        </Button>
      </div>
      <div style={{ flex: 1, minHeight: 0, display: "flex" }}>
        <div style={{ flex: 1, minWidth: 0, display: "flex", flexDirection: "column" }}>
          <div style={{ flex: "0 0 40%", minHeight: 120, borderBottom: "1px solid var(--border)" }}>
            <Editor value={sql} onChange={setSql} onKeyDown={onKeyDown} textareaRef={ta} ariaLabel="SQL" placeholder="select * from …   ⌘↵ runs the statement under the cursor" />
          </div>
          <div style={{ flex: 1, minHeight: 0, display: "flex", flexDirection: "column" }}>
            <div style={{ display: "flex", alignItems: "center", gap: 8, height: 36, flex: "none", padding: "0 12px", borderBottom: "1px solid var(--border)", background: "var(--sidebar)" }}>
              <div className="m-seg" role="tablist" aria-label="Result view">
                {(["results", "explain", "messages"] as Pane[]).map((p) => (
                  <button key={p} type="button" role="tab" aria-selected={pane === p} className={cn("m-seg__item", pane === p && "is-active")} onClick={() => setPane(p)}>
                    {p === "results" ? "Results" : p === "explain" ? "Explain" : "Messages"}
                    {p === "results" && result ? <span className="m-mono m-muted">{thousands(result.rows.length)}</span> : null}
                    {p === "messages" && messages.some((m) => m.error) ? <span className="m-dot m-dot--down" /> : null}
                  </button>
                ))}
              </div>
              <span className="m-spacer" />
              {pane === "results" && result && result.columns.length > 0 ? (
                <Button variant="ghost" size="xs" onClick={copyCsv}>
                  <Copy className="m-icon m-icon--sm" aria-hidden="true" />
                  Copy as CSV
                </Button>
              ) : null}
            </div>
            {pane === "results" ? (
              result ? (
                result.columns.length > 0 ? (
                  <div className="m-grid-wrap m-grid-wrap--flush" style={{ flex: 1 }}>
                    <ResultGrid result={result} />
                  </div>
                ) : (
                  <p className="m-help" style={{ padding: 16 }}>
                    {result.command} · {plural(result.rowsAffected, "row")}
                  </p>
                )
              ) : (
                <div style={{ padding: 24 }}>
                  <EmptyState title="Run a query" description="⌘↵ runs the statement under the cursor, or the selection. ⇧⌘↵ explains it." />
                </div>
              )
            ) : pane === "explain" ? (
              <div style={{ flex: 1, overflow: "auto", padding: "12px 16px", font: "400 13px/22px var(--font-mono)", whiteSpace: "pre" }}>
                {plan ? plan.map((line, i) => <ExplainLine key={i} line={line} />) : <span className="m-muted">Explain shows the plan without running the query. Analyze runs it and rolls back.</span>}
              </div>
            ) : (
              <div style={{ flex: 1, overflow: "auto", padding: "12px 16px", display: "flex", flexDirection: "column", gap: 6, font: "400 13px/20px var(--font-mono)" }}>
                {messages.length === 0 ? <span className="m-muted">No messages yet.</span> : null}
                {messages.map((m, i) => (
                  <span key={i} style={{ whiteSpace: "pre-wrap" }}>
                    <span className="m-muted">{clockTime(m.at)}</span>
                    {"  "}
                    <span style={{ color: m.error ? "var(--destructive)" : undefined }}>{m.text}</span>
                  </span>
                ))}
              </div>
            )}
          </div>
        </div>
        {library ? <Library onPick={(s) => setSql(s)} onClose={() => setLibrary(false)} /> : null}
      </div>
      <ReviewDialog
        open={script !== null}
        onOpenChange={(o) => !o && setScript(null)}
        title="Review statement"
        target={`${database}`}
        connection={connection}
        reviewKey={["script", database, script]}
        load={async () => {
          const r = await planScript(connectionId, database, { sql: script ?? "" });
          return { plan: r.plan, decision: r.decision, upSql: r.upSql, downSql: r.downSql };
        }}
        apply={({ confirmation, transaction }) => applyScript(connectionId, database, { sql: script ?? "", confirmation, transaction })}
        onApplied={async (res) => {
          log(`Applied ${plural(res.plan.statements.length, "statement")}${res.durationMs !== undefined ? ` · ${duration(res.durationMs)}` : ""} · audited`);
          setPane("messages");
          await qc.invalidateQueries({ queryKey: getGetSchemaQueryKey(connectionId, database) });
        }}
      />
      <SaveDialog open={saving} onOpenChange={setSaving} sql={sql} tab={tab} />
    </>
  );
}

function ExplainLine({ line }: { line: string }) {
  const m = /^(\s*(?:->\s+)?)([A-Z][A-Za-z ]+?)(\s+(?:on|using)\s+.*?)?(\s+\(.*)?$/.exec(line);
  if (!m) return <div style={{ color: /Rows Removed by/.test(line) ? "var(--warning)" : undefined }}>{line}</div>;
  return (
    <div>
      {m[1]}
      <span className="sx-k">{m[2]}</span>
      {m[3]}
      {m[4] ? <span className="m-muted">{m[4]}</span> : null}
    </div>
  );
}

function Library({ onPick, onClose }: { onPick: (sql: string) => void; onClose: () => void }) {
  const { connectionId } = useWorkspace();
  const qc = useQueryClient();
  const history = useListHistory(connectionId, { limit: 50 });
  const saved = useListSavedQueries();
  const [view, setView] = React.useState<"saved" | "history">("history");
  const mine = (saved.data ?? []).filter((q) => !q.connectionId || q.connectionId === connectionId);
  return (
    <aside className="m-inspector" aria-label="Query library" style={{ width: "var(--size-inspector)", flex: "none" }}>
      <div className="m-inspector__head">
        <div className="m-seg" role="tablist" aria-label="Library">
          <button type="button" role="tab" aria-selected={view === "history"} className={cn("m-seg__item", view === "history" && "is-active")} onClick={() => setView("history")}>
            History
          </button>
          <button type="button" role="tab" aria-selected={view === "saved"} className={cn("m-seg__item", view === "saved" && "is-active")} onClick={() => setView("saved")}>
            Saved <span className="m-mono m-muted">{mine.length}</span>
          </button>
        </div>
        <span className="m-spacer" />
        <Button variant="ghost" size="xs" icon aria-label="Close" onClick={onClose}>
          <X className="m-icon m-icon--sm" aria-hidden="true" />
        </Button>
      </div>
      <div className="m-inspector__body" style={{ gap: 4 }}>
        {view === "history"
          ? (history.data ?? []).map((h: QueryRun) => (
              <button key={h.id} type="button" className="m-kv" style={{ textAlign: "left", cursor: "pointer" }} onClick={() => onPick(h.sql)} title="Open in the editor">
                <span className="m-kv__key">
                  <span className={cn("m-dot", h.error ? "m-dot--down" : "m-dot--online")} />
                  <span>
                    {relativeTime(h.ranAt)} · {h.error ? "error" : `${thousands(h.rowCount)} rows · ${duration(h.durationMs)}`}
                  </span>
                </span>
                <span className="m-kv__val" style={{ display: "-webkit-box", WebkitLineClamp: 3, WebkitBoxOrient: "vertical", overflow: "hidden", fontSize: 12, lineHeight: "18px" }}>
                  <Highlighted code={h.sql} />
                </span>
              </button>
            ))
          : mine.map((q: SavedQuery) => (
              <div key={q.id} className="m-kv">
                <span className="m-kv__key" style={{ justifyContent: "space-between" }}>
                  <button type="button" onClick={() => onPick(q.sql)} style={{ fontWeight: 500 }}>
                    {q.name}
                  </button>
                  <Button
                    variant="ghost"
                    size="xs"
                    icon
                    aria-label={`Delete ${q.name}`}
                    onClick={async () => {
                      await deleteSavedQuery(q.id!);
                      await qc.invalidateQueries({ queryKey: getListSavedQueriesQueryKey() });
                    }}
                  >
                    <Trash2 className="m-icon m-icon--sm" aria-hidden="true" />
                  </Button>
                </span>
                <button type="button" className="m-kv__val" style={{ textAlign: "left", fontSize: 12, lineHeight: "18px", display: "-webkit-box", WebkitLineClamp: 3, WebkitBoxOrient: "vertical", overflow: "hidden" }} onClick={() => onPick(q.sql)}>
                  <Highlighted code={q.sql} />
                </button>
              </div>
            ))}
        {view === "history" && (history.data ?? []).length === 0 ? <p className="m-help">Queries you run on this connection appear here.</p> : null}
        {view === "saved" && mine.length === 0 ? <p className="m-help">Save a query with ⌘S to keep it here.</p> : null}
      </div>
    </aside>
  );
}

function SaveDialog({ open, onOpenChange, sql, tab }: { open: boolean; onOpenChange: (o: boolean) => void; sql: string; tab: QueryTab }) {
  const { connectionId } = useWorkspace();
  const qc = useQueryClient();
  const [name, setName] = React.useState(tab.title.startsWith("Query ") ? "" : tab.title);
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent width={440} aria-describedby={undefined}>
        <form
          onSubmit={async (e) => {
            e.preventDefault();
            if (!name.trim()) return;
            try {
              const saved = await saveQuery({ id: tab.savedQueryId, name: name.trim(), sql, connectionId });
              useTabs.getState().update(connectionId, tab.id, { title: saved.name, savedQueryId: saved.id });
              await qc.invalidateQueries({ queryKey: getListSavedQueriesQueryKey() });
              toast.success(`Saved ${saved.name}`);
              onOpenChange(false);
            } catch (err) {
              toast.error(errorMessage(err));
            }
          }}
        >
          <DialogHead>
            <DialogTitle>{tab.savedQueryId ? "Update saved query" : "Save query"}</DialogTitle>
          </DialogHead>
          <DialogBody>
            <Field label="Name" htmlFor="save-name">
              <input id="save-name" className="m-input" value={name} onChange={(e) => setName(e.target.value)} placeholder="Revenue by plan" autoFocus />
            </Field>
          </DialogBody>
          <DialogFoot>
            <span className="m-spacer" />
            <Button variant="ghost" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button variant="primary" type="submit" disabled={!name.trim()}>
              Save
            </Button>
          </DialogFoot>
        </form>
      </DialogContent>
    </Dialog>
  );
}
