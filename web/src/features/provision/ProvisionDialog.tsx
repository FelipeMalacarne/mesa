/**
 * New app database: the role and the database on the connection's CloudNativePG cluster,
 * created now, and their declaration committed to the GitOps repository the cluster syncs from
 * (the CNPG Database, the managed role, and the app's connection Secret, encrypted with sops).
 * The person names the database and the app's namespace, reviews the SQL and the files, and
 * applies; production asks to be unlocked and named, as every write does.
 */
import * as React from "react";
import { useQueryClient } from "@tanstack/react-query";
import { ExternalLink, GitPullRequest, LockOpen, ShieldAlert } from "lucide-react";
import { toast } from "sonner";

import { getGetConnectionQueryKey, getListConnectionsQueryKey, unlockConnection } from "@/lib/api/connections/connections";
import { getListDatabasesQueryKey } from "@/lib/api/explore/explore";
import type { Connection, ProvisioningTarget, ProvisionResult } from "@/lib/api/mesaAPI.schemas";
import { provisionDatabase } from "@/lib/api/provision/provision";
import { getListRolesQueryKey } from "@/lib/api/roles/roles";
import { errorMessage } from "@/lib/api/fetch";
import { Button } from "@/ui/Button";
import { CodeBlock } from "@/ui/Code";
import { EnvTag, Field } from "@/ui/controls";
import { Dialog, DialogBody, DialogContent, DialogDescription, DialogFoot, DialogHead, DialogTitle } from "@/ui/primitives/dialog";

const namePattern = /^[a-z][a-z0-9_]{0,39}$/;
const namespacePattern = /^[a-z0-9]([-a-z0-9]{0,61}[a-z0-9])?$/;

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  connection: Connection;
  target: ProvisioningTarget;
}

export function ProvisionDialog(props: Props) {
  return (
    <Dialog open={props.open} onOpenChange={props.onOpenChange}>
      <DialogContent width={680} aria-describedby={undefined}>
        {props.open ? <ProvisionBody {...props} /> : null}
      </DialogContent>
    </Dialog>
  );
}

function ProvisionBody({ connection, target, onOpenChange }: Props) {
  const qc = useQueryClient();
  const [name, setName] = React.useState("");
  const [namespace, setNamespace] = React.useState("");
  const [nsTouched, setNsTouched] = React.useState(false);
  const [plan, setPlan] = React.useState<ProvisionResult | null>(null);
  const [done, setDone] = React.useState<ProvisionResult | null>(null);
  const [typed, setTyped] = React.useState("");
  const [busy, setBusy] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);

  const ns = nsTouched ? namespace : name.replaceAll("_", "-");
  const validName = namePattern.test(name);
  const validNs = namespacePattern.test(ns);
  const decision = plan?.decision;
  const needsName = decision?.outcome === "needs_confirmation";
  const denied = decision?.outcome === "deny";

  const review = async () => {
    setBusy(true);
    setError(null);
    try {
      setPlan(await provisionDatabase(connection.id, { name, namespace: ns, dryRun: true }));
    } catch (e) {
      setError(errorMessage(e));
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
      setPlan(await provisionDatabase(connection.id, { name, namespace: ns, dryRun: true }));
    } catch (e) {
      toast.error(errorMessage(e));
    } finally {
      setBusy(false);
    }
  };

  const apply = async () => {
    setBusy(true);
    setError(null);
    try {
      const res = await provisionDatabase(connection.id, { name, namespace: ns, confirmation: needsName ? typed.trim() : undefined });
      setDone(res);
      await qc.invalidateQueries({ queryKey: getListRolesQueryKey(connection.id) });
      await qc.invalidateQueries({ queryKey: getListDatabasesQueryKey(connection.id) });
    } catch (e) {
      setError(errorMessage(e));
    } finally {
      setBusy(false);
    }
  };

  const where = (
    <>
      <span className="m-mono">{target.repo}</span>
      {target.pullRequest ? " (as a pull request)" : <> on <span className="m-mono">{target.branch}</span></>}
    </>
  );

  if (done) {
    const link = done.commit?.pullRequestUrl ?? done.commit?.url;
    return (
      <>
        <DialogHead>
          <DialogTitle>Database {name}</DialogTitle>
          <DialogDescription>{done.note}</DialogDescription>
        </DialogHead>
        <DialogBody>
          <Row label="Connection for the app">
            Secret <span className="m-mono">{done.appSecret}</span> in namespace <span className="m-mono">{ns}</span>, with{" "}
            <span className="m-mono">DATABASE_URL</span>, <span className="m-mono">username</span>, <span className="m-mono">password</span>,{" "}
            <span className="m-mono">host</span>, <span className="m-mono">port</span> and <span className="m-mono">dbname</span>.
          </Row>
          {link ? (
            <Row label={done.commit?.pullRequestUrl ? "Pull request" : "Commit"}>
              <a href={link} target="_blank" rel="noreferrer" className="m-row" style={{ display: "inline-flex" }}>
                {done.commit?.pullRequestUrl ? <GitPullRequest className="m-icon m-icon--sm" aria-hidden="true" /> : <ExternalLink className="m-icon m-icon--sm" aria-hidden="true" />}
                {done.commit?.pullRequestUrl ? "Open the pull request" : `${target.repo}@${done.commit?.sha.slice(0, 7)}`}
              </a>
            </Row>
          ) : null}
        </DialogBody>
        <DialogFoot>
          <span className="m-spacer" />
          <Button variant="primary" onClick={() => onOpenChange(false)}>
            Done
          </Button>
        </DialogFoot>
      </>
    );
  }

  return (
    <>
      <DialogHead>
        <div className="m-row">
          <DialogTitle>New app database</DialogTitle>
          <EnvTag env={connection.environment} />
        </div>
        <DialogDescription>
          Creates a login and a database it owns on <span className="m-mono">{target.cluster}</span> now, and declares both in {where}, so
          the cluster keeps them and the app gets its connection as a Secret.
        </DialogDescription>
      </DialogHead>
      <DialogBody>
        <div className="m-row" style={{ alignItems: "flex-start" }}>
          <Field label="Database and login" htmlFor="prov-name" error={name && !validName ? "Lowercase letters, digits and _, starting with a letter" : undefined}>
            <input
              id="prov-name"
              className="m-input m-input--mono"
              value={name}
              onChange={(e) => {
                setName(e.target.value);
                setPlan(null);
              }}
              placeholder="shop"
              autoComplete="off"
              autoFocus
            />
          </Field>
          <Field label="App namespace" htmlFor="prov-ns" help={validNs ? `Gets the Secret ${ns}-db` : undefined} error={ns && !validNs ? "Not a Kubernetes namespace name" : undefined}>
            <input
              id="prov-ns"
              className="m-input m-input--mono"
              value={ns}
              onChange={(e) => {
                setNsTouched(true);
                setNamespace(e.target.value);
                setPlan(null);
              }}
              placeholder="shop"
              autoComplete="off"
            />
          </Field>
        </div>
        {plan ? (
          <>
            <span className="m-label">Runs now (the password is generated and never shown)</span>
            <CodeBlock code={plan.statements.join("\n")} style={{ maxHeight: 160 }} />
            <span className="m-label">Commits to {where}</span>
            <CodeBlock code={plan.files.join("\n")} language="dbml" style={{ maxHeight: 160 }} />
            {target.pullRequest ? <p className="m-help">With a pull request nothing runs now: CNPG creates the login and the database once it is merged and synced.</p> : null}
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
              <div className="m-field">
                <label className="m-label" htmlFor="prov-confirm">
                  Type <span className="m-mono">{decision?.confirmWith}</span> to confirm
                  {decision?.reason ? <span className="m-help" style={{ display: "block", fontWeight: 400 }}>{decision.reason}</span> : null}
                </label>
                <input id="prov-confirm" className="m-input m-input--mono" placeholder={decision?.confirmWith} value={typed} onChange={(e) => setTyped(e.target.value)} autoComplete="off" />
              </div>
            ) : null}
          </>
        ) : null}
        {error ? <p className="m-help m-help--error">{error}</p> : null}
      </DialogBody>
      <DialogFoot>
        <span className="m-spacer" />
        <Button variant="outline" onClick={() => onOpenChange(false)}>
          Cancel
        </Button>
        {plan ? (
          <Button
            variant={needsName && connection.environment === "production" ? "destructive" : "primary"}
            disabled={busy || denied || (needsName && typed.trim() !== decision?.confirmWith)}
            onClick={apply}
          >
            {busy ? "Working…" : target.pullRequest ? "Open the pull request" : "Create and commit"}
          </Button>
        ) : (
          <Button variant="primary" disabled={busy || !validName || !validNs} onClick={review}>
            {busy ? "Planning…" : "Review"}
          </Button>
        )}
      </DialogFoot>
    </>
  );
}

function Row({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <div className="m-field">
      <span className="m-label">{label}</span>
      <span>{children}</span>
    </div>
  );
}
