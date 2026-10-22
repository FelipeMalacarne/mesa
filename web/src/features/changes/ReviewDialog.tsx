/**
 * The one review screen every write passes through: the exact SQL (up and down), the risks
 * the engine found, and the policy decision. Read-only connections offer an unlock; risky or
 * production writes ask for the database name. Nothing runs until the person applies.
 */
import * as React from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { Copy, Download, LockOpen, ShieldAlert } from "lucide-react";
import { toast } from "sonner";

import { getListConnectionsQueryKey, getGetConnectionQueryKey, unlockConnection } from "@/lib/api/connections/connections";
import type { ApplyResult, Connection, Decision, Plan, Risk, SchemaChange } from "@/lib/api/mesaAPI.schemas";
import { errorMessage, isApiError } from "@/lib/api/fetch";
import { duration, plural } from "@/lib/format";
import { cn, isCmd } from "@/lib/utils";
import { Button } from "@/ui/Button";
import { CodeBlock } from "@/ui/Code";
import { EnvTag, Kbd } from "@/ui/controls";
import { Dialog, DialogBody, DialogContent, DialogDescription, DialogFoot, DialogHead, DialogTitle } from "@/ui/primitives/dialog";
import { shortcut } from "@/app/keys";

export interface Review {
  plan: Plan;
  decision: Decision;
  upSql: string;
  downSql: string;
  /** Designer reviews list the structural changes above the SQL. */
  changes?: SchemaChange[];
}

export interface ApplyOptions {
  confirmation?: string;
  transaction: boolean;
}

interface ReviewDialogProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  title: string;
  /** What the statements run on, e.g. "shop.public.orders". */
  target: string;
  connection: Connection;
  /** A key that changes whenever the reviewed content changes. */
  reviewKey: unknown;
  load: () => Promise<Review>;
  apply: (opts: ApplyOptions) => Promise<ApplyResult>;
  onApplied?: (result: ApplyResult) => void;
  /** Statements that cannot run in a transaction (CREATE DATABASE) hide the option. */
  transactional?: boolean;
  applyLabel?: string;
}

export function ReviewDialog(props: ReviewDialogProps) {
  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent width={680} aria-describedby={undefined}>
        {props.open ? <ReviewBody {...props} /> : null}
      </DialogContent>
    </Dialog>
  );
}

function ReviewBody({ title, target, connection, reviewKey, load, apply, onApplied, onOpenChange, transactional = true, applyLabel }: ReviewDialogProps) {
  const qc = useQueryClient();
  const review = useQuery({ queryKey: ["review", connection.id, reviewKey], queryFn: load, staleTime: 0, gcTime: 0, retry: false });
  const [direction, setDirection] = React.useState<"up" | "down">("up");
  const [transaction, setTransaction] = React.useState(true);
  const [typed, setTyped] = React.useState("");
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  const data = review.data;
  const decision = data?.decision;
  const count = data?.plan.statements.length ?? 0;
  const needsName = decision?.outcome === "needs_confirmation";
  const denied = decision?.outcome === "deny";
  const canApply = !!data && count > 0 && !denied && (!needsName || typed.trim() === decision?.confirmWith) && !busy;

  const doApply = async () => {
    if (!canApply) return;
    setBusy(true);
    setError(null);
    try {
      const res = await apply({ confirmation: needsName ? typed.trim() : undefined, transaction });
      toast.success(`Applied ${plural(res.plan.statements.length, "statement")}${res.durationMs !== undefined ? ` in ${duration(res.durationMs)}` : ""}`);
      onApplied?.(res);
      onOpenChange(false);
    } catch (e) {
      setError(errorMessage(e));
      if (isApiError(e) && (e.kind === "forbidden" || e.kind === "needs_confirmation")) void review.refetch();
    } finally {
      setBusy(false);
    }
  };

  const unlock = async () => {
    setBusy(true);
    try {
      await unlockConnection(connection.id, { ttlSeconds: 15 * 60 });
      await qc.invalidateQueries({ queryKey: getListConnectionsQueryKey() });
      await qc.invalidateQueries({ queryKey: getGetConnectionQueryKey(connection.id) });
      toast.success(`${connection.name} is writable for 15 minutes`);
      await review.refetch();
    } catch (e) {
      toast.error(errorMessage(e));
    } finally {
      setBusy(false);
    }
  };

  const sql = direction === "up" ? (data?.upSql ?? "") : (data?.downSql ?? "");

  return (
    <div
      onKeyDown={(e) => {
        if (isCmd(e) && e.key === "Enter") {
          e.preventDefault();
          void doApply();
        }
      }}
    >
      <DialogHead>
        <div className="m-row">
          <DialogTitle>{title}</DialogTitle>
          <EnvTag env={connection.environment} />
        </div>
        <DialogDescription>
          {data ? (
            <>
              {plural(count, "statement")} will run on <span className="m-mono">{target}</span>.
            </>
          ) : (
            <>
              Planning changes on <span className="m-mono">{target}</span>…
            </>
          )}
        </DialogDescription>
      </DialogHead>
      <DialogBody>
        {review.isError ? <p className="m-help m-help--error">{errorMessage(review.error)}</p> : null}
        {data?.changes && data.changes.length > 0 ? <ChangeList changes={data.changes} /> : null}
        {data ? (
          <>
            <div className="m-row">
              <div className="m-seg" role="tablist" aria-label="Migration direction">
                <button type="button" role="tab" aria-selected={direction === "up"} className={cn("m-seg__item", direction === "up" && "is-active")} onClick={() => setDirection("up")}>
                  Up
                </button>
                <button type="button" role="tab" aria-selected={direction === "down"} className={cn("m-seg__item", direction === "down" && "is-active")} onClick={() => setDirection("down")}>
                  Down
                </button>
              </div>
              <span className="m-help">{direction === "up" ? "What runs now" : "Saved with the migration to undo it"}</span>
            </div>
            {sql ? (
              <CodeBlock code={sql} style={{ maxHeight: 280 }} />
            ) : (
              <p className="m-help">{direction === "down" ? "Mesa cannot derive a rollback for these statements." : "Nothing to run."}</p>
            )}
            <Risks risks={data.plan.risks ?? []} />
            {transactional ? (
              <label className="m-row" style={{ cursor: "pointer", flexWrap: "nowrap" }}>
                <input type="checkbox" checked={transaction} onChange={() => setTransaction(!transaction)} style={{ width: 16, height: 16, margin: 0, accentColor: "var(--primary)" }} />
                <span>Run in a single transaction</span>
                <span className="m-help">
                  {(data.plan.risks ?? []).some((r) => r.code === "no_transaction")
                    ? "Row changes roll back together; schema changes are committed as each one runs."
                    : "If one statement fails, none are applied."}
                </span>
              </label>
            ) : null}
            {denied ? (
              <div className="m-field" style={{ padding: 12, border: "1px solid var(--border)", borderRadius: "var(--radius-lg)" }}>
                <span className="m-status" style={{ whiteSpace: "normal" }}>
                  <ShieldAlert className="m-icon m-icon--sm" style={{ color: "var(--warning)" }} aria-hidden="true" />
                  {decision?.reason ?? "This connection is read-only."}
                </span>
                <div>
                  <Button variant="secondary" size="sm" onClick={unlock} disabled={busy}>
                    <LockOpen className="m-icon m-icon--sm" aria-hidden="true" />
                    Unlock for 15 minutes
                  </Button>
                </div>
              </div>
            ) : null}
            {needsName ? (
              <div
                className="m-field"
                style={{
                  padding: 12,
                  border: `1px solid ${connection.environment === "production" ? "var(--env-production)" : "var(--warning)"}`,
                  borderRadius: "var(--radius-lg)",
                  background: connection.environment === "production" ? "var(--deleted-surface)" : "var(--pending-surface)",
                }}
              >
                <label className="m-label" htmlFor="review-confirm">
                  Type <span className="m-mono">{decision?.confirmWith}</span> to confirm
                  {decision?.reason ? <span className="m-help" style={{ display: "block", fontWeight: 400 }}>{decision.reason}</span> : null}
                </label>
                <input id="review-confirm" className="m-input m-input--mono" placeholder={decision?.confirmWith} value={typed} onChange={(e) => setTyped(e.target.value)} autoComplete="off" autoFocus />
              </div>
            ) : null}
            {error ? <p className="m-help m-help--error">{error}</p> : null}
          </>
        ) : null}
      </DialogBody>
      <DialogFoot>
        <Button
          variant="ghost"
          disabled={!data}
          onClick={() => {
            void navigator.clipboard.writeText(sql).then(() => toast.success("SQL copied"));
          }}
        >
          <Copy className="m-icon m-icon--sm" aria-hidden="true" />
          Copy SQL
        </Button>
        <Button variant="ghost" disabled={!data} onClick={() => data && saveMigration(target, data)}>
          <Download className="m-icon m-icon--sm" aria-hidden="true" />
          Save as migration
        </Button>
        <span className="m-spacer" />
        <Button variant="outline" onClick={() => onOpenChange(false)}>
          Cancel
        </Button>
        <Button variant={needsName && connection.environment === "production" ? "destructive" : "primary"} disabled={!canApply} onClick={doApply}>
          {busy ? "Applying…" : (applyLabel ?? `Apply ${plural(count, "statement")}`)} <Kbd>{shortcut("Enter")}</Kbd>
        </Button>
      </DialogFoot>
    </div>
  );
}

function ChangeList({ changes }: { changes: SchemaChange[] }) {
  return (
    <ul style={{ listStyle: "none", margin: 0, padding: 0, display: "flex", flexDirection: "column", gap: 6, maxHeight: 160, overflow: "auto" }}>
      {changes.map((c, i) => {
        const sign = c.kind.startsWith("drop") ? "−" : c.kind.startsWith("create") || c.kind.startsWith("add") ? "+" : "~";
        const color = sign === "−" ? "var(--deleted-foreground)" : sign === "+" ? "var(--added-foreground)" : "var(--pending-foreground)";
        return (
          <li key={i} className="m-row" style={{ flexWrap: "nowrap" }}>
            <span className="m-mono" style={{ width: 16, textAlign: "center", color, fontWeight: 600 }}>
              {sign}
            </span>
            <span>{describeChange(c)}</span>
          </li>
        );
      })}
    </ul>
  );
}

/** A person-facing line for a structural change (mirrors schema.Change.Describe on the server). */
export function describeChange(c: SchemaChange): string {
  const t = c.table ? `${c.table.schema}.${c.table.name}` : "";
  switch (c.kind) {
    case "create_schema":
      return `Create schema ${c.schema}`;
    case "drop_schema":
      return `Drop schema ${c.schema}`;
    case "create_enum":
      return `Create type ${c.schema}.${c.enum?.name}`;
    case "drop_enum":
      return `Drop type ${c.schema}.${c.name}`;
    case "add_enum_value":
      return `Add ${c.values?.map((v) => `'${v}'`).join(", ")} to type ${c.schema}.${c.name}`;
    case "create_table":
      return `Create table ${t} (${plural(c.newTable?.columns.length ?? 0, "column")})`;
    case "drop_table":
      return `Drop table ${t}`;
    case "add_column":
      return `Add column ${t}.${c.column?.name} ${c.column?.type}`;
    case "drop_column":
      return `Drop column ${t}.${c.name}`;
    case "alter_column_type":
      return `Change ${t}.${c.column?.name} from ${c.oldColumn?.type} to ${c.column?.type}`;
    case "set_not_null":
      return `Require a value in ${t}.${c.name}`;
    case "drop_not_null":
      return `Allow null in ${t}.${c.name}`;
    case "set_default":
      return `Set default of ${t}.${c.column?.name} to ${c.column?.default}`;
    case "drop_default":
      return `Drop default of ${t}.${c.name}`;
    case "add_primary_key":
      return `Add primary key (${c.key?.columns.join(", ")}) on ${t}`;
    case "drop_primary_key":
      return `Drop primary key of ${t}`;
    case "add_unique":
      return `Add unique (${c.key?.columns.join(", ")}) on ${t}`;
    case "drop_unique":
      return `Drop unique ${c.name} on ${t}`;
    case "add_check":
      return `Add check ${c.check?.expression} on ${t}`;
    case "drop_check":
      return `Drop check ${c.name} on ${t}`;
    case "create_index":
      return `Create index ${c.index?.name} on ${t}`;
    case "drop_index":
      return `Drop index ${c.name}`;
    case "add_foreign_key":
      return `Add foreign key ${t} → ${c.foreignKey?.references.schema}.${c.foreignKey?.references.name}`;
    case "drop_foreign_key":
      return `Drop foreign key ${c.name} on ${t}`;
    case "set_table_comment":
      return `Comment on ${t}`;
    case "set_column_comment":
      return `Comment on ${t}.${c.column?.name}`;
    default:
      return c.kind;
  }
}

function Risks({ risks }: { risks: Risk[] }) {
  const loss = risks.some((r) => r.code === "data_loss");
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 6, padding: "10px 12px", border: "1px solid var(--border)", borderRadius: "var(--radius-lg)" }}>
      {!loss ? (
        <span className="m-status">
          <span className="m-dot m-dot--online" />
          No data loss
        </span>
      ) : null}
      {risks.map((r, i) => (
        <span key={i} className="m-status" style={{ whiteSpace: "normal", alignItems: "flex-start" }}>
          <span className={cn("m-dot", r.level === "danger" ? "m-dot--down" : r.level === "warning" ? "m-dot--degraded" : "m-dot--unknown")} style={{ marginTop: 5 }} />
          <span>
            {r.message}
            {r.object && !r.message.includes(r.object) ? <span className="m-help m-mono"> · {r.object}</span> : null}
          </span>
        </span>
      ))}
    </div>
  );
}

function saveMigration(target: string, review: Review) {
  const stamp = new Date().toISOString().replace(/[-:T]/g, "").slice(0, 14);
  const name = target.replace(/[^a-z0-9]+/gi, "_").toLowerCase();
  const download = (file: string, text: string) => {
    const url = URL.createObjectURL(new Blob([text], { type: "text/plain" }));
    const a = document.createElement("a");
    a.href = url;
    a.download = file;
    a.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  };
  download(`${stamp}_${name}.up.sql`, review.upSql);
  download(`${stamp}_${name}.down.sql`, review.downSql || "-- Mesa could not derive a rollback for this migration; write one by hand.\n");
}
