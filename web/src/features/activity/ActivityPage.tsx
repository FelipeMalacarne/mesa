/** Everything Mesa wrote on this connection, newest first, with the SQL and its rollback. */
import * as React from "react";
import { ChevronRight, History } from "lucide-react";

import { useListAudit } from "@/lib/api/changes/changes";
import type { AuditEntry } from "@/lib/api/mesaAPI.schemas";
import { errorMessage } from "@/lib/api/fetch";
import { plural, relativeTime } from "@/lib/format";
import { useListKeys } from "@/lib/listKeys";
import { cn } from "@/lib/utils";
import { useStatusFacts } from "@/app/status.store";
import { CodeBlock } from "@/ui/Code";
import { EmptyState, EnvTag } from "@/ui/controls";
import { useTabs } from "@/features/explorer/tabs.store";

import { useWorkspace } from "../workspace/context";

export function ActivityPage() {
  const { connectionId } = useWorkspace();
  const audit = useListAudit(connectionId, { limit: 200 }, { query: { refetchInterval: 15_000 } });
  const [open, setOpen] = React.useState<string | null>(null);
  React.useEffect(() => {
    useTabs.getState().open(connectionId, { id: "activity", kind: "activity", title: "Activity" });
  }, [connectionId]);
  const failed = (audit.data ?? []).filter((a) => a.status === "failed").length;
  const listKeys = useListKeys<HTMLTableElement>();
  useStatusFacts(audit.data ? [plural(audit.data.length, "write"), ...(failed ? [`${failed} failed`] : [])] : []);
  return (
    <div style={{ flex: 1, minHeight: 0, overflow: "auto", padding: "16px 24px", display: "flex", flexDirection: "column", gap: 12 }}>
      <h2 className="m-section-title">
        Activity <span className="m-badge m-badge--count">{audit.data?.length ?? 0}</span>
        <span className="m-help" style={{ fontWeight: 400 }}>
          every write Mesa ran here, from the web app, the CLI and agents
        </span>
      </h2>
      {audit.isError ? <p className="m-help m-help--error">{errorMessage(audit.error)}</p> : null}
      {audit.data && audit.data.length === 0 ? <EmptyState icon={<History className="m-icon" aria-hidden="true" />} title="Nothing written yet" description="Row edits, schema changes and scripts appear here once applied, with the SQL that ran and how to undo it." /> : null}
      {audit.data && audit.data.length > 0 ? (
        <div className="m-grid-wrap">
          <table className="m-grid m-grid--ui m-list" style={{ width: "100%" }} aria-label="Writes" {...listKeys}>
            <thead>
              <tr>
                <th style={{ width: 28 }} />
                <th>When</th>
                <th>Status</th>
                <th>Change</th>
                <th>Database</th>
                <th>By</th>
                <th>Via</th>
              </tr>
            </thead>
            <tbody>
              {audit.data.map((a) => (
                <Row key={a.id} a={a} open={open === a.id} onToggle={() => setOpen(open === a.id ? null : a.id)} />
              ))}
            </tbody>
          </table>
        </div>
      ) : null}
    </div>
  );
}

function Row({ a, open, onToggle }: { a: AuditEntry; open: boolean; onToggle: () => void }) {
  return (
    <>
      <tr className="is-link" data-row="" tabIndex={-1} onClick={onToggle} aria-expanded={open}>
        <td>
          <ChevronRight className={cn("m-tree__chev", open && "is-open")} aria-hidden="true" />
        </td>
        <td className="m-muted" title={a.startedAt}>
          {relativeTime(a.startedAt)}
        </td>
        <td>
          <span className="m-status">
            <span className={cn("m-dot", a.status === "applied" ? "m-dot--online" : a.status === "failed" ? "m-dot--down" : "m-dot--pending")} />
            {a.status}
          </span>
        </td>
        <td style={{ maxWidth: 480 }}>{a.summary}</td>
        <td className="m-mono">
          {a.database} <EnvTag env={a.environment} />
        </td>
        <td className="m-mono">{a.actor}</td>
        <td className="m-muted">{a.surface}</td>
      </tr>
      {open ? (
        <tr>
          <td />
          <td colSpan={6} style={{ height: "auto", whiteSpace: "normal", padding: "8px 12px 16px", maxWidth: "none" }}>
            {a.error ? <p className="m-help m-help--error" style={{ marginTop: 0 }}>{a.error}</p> : null}
            <div className="m-label" style={{ marginBottom: 4 }}>
              Ran
            </div>
            <CodeBlock code={a.sql} style={{ maxHeight: 260 }} />
            {a.downSql ? (
              <>
                <div className="m-label" style={{ margin: "12px 0 4px" }}>
                  To undo
                </div>
                <CodeBlock code={a.downSql} style={{ maxHeight: 200 }} />
              </>
            ) : null}
          </td>
        </tr>
      ) : null}
    </>
  );
}
