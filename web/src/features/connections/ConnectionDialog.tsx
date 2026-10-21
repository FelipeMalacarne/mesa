/**
 * New / edit connection (`NewConnection` board). Production forces read-only. The password
 * can also be an `env:NAME` reference that the server resolves at connect time.
 */
import * as React from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { Database, Plug } from "lucide-react";
import { toast } from "sonner";

import {
  createConnection,
  getGetConnectionQueryKey,
  getListConnectionsQueryKey,
  testConnection,
  updateConnection,
} from "@/lib/api/connections/connections";
import type { Connection, ConnectionInput, Engine, Environment, TLSMode } from "@/lib/api/mesaAPI.schemas";
import { errorMessage } from "@/lib/api/fetch";
import { defaultPort, duration, ENVIRONMENTS, envLabel } from "@/lib/format";
import { Button } from "@/ui/Button";
import { Choice, Field, Select, StatusLabel, Switch } from "@/ui/controls";
import { Dialog, DialogBody, DialogContent, DialogDescription, DialogFoot, DialogHead, DialogTitle } from "@/ui/primitives/dialog";

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** Editing an existing connection; absent for a new one. */
  connection?: Connection;
}

const TLS_OPTIONS: { value: TLSMode; label: string }[] = [
  { value: "disable", label: "Disable" },
  { value: "prefer", label: "Prefer" },
  { value: "require", label: "Require" },
  { value: "verify-ca", label: "Verify CA" },
  { value: "verify-full", label: "Verify full" },
];

interface FormState {
  engine: Engine;
  name: string;
  environment: Environment;
  host: string;
  port: string;
  database: string;
  user: string;
  secret: string;
  tls: TLSMode;
  readOnly: boolean;
}

function initial(c?: Connection): FormState {
  return {
    engine: c?.engine ?? "postgres",
    name: c?.name ?? "",
    environment: c?.environment ?? "local",
    host: c?.host ?? "localhost",
    port: c ? String(c.port) : "",
    database: c?.database ?? "postgres",
    user: c?.user ?? "postgres",
    secret: "",
    tls: c?.tls ?? "prefer",
    readOnly: c?.readOnly ?? false,
  };
}

export function ConnectionDialog({ open, onOpenChange, connection }: Props) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent width={600} aria-describedby={undefined}>
        {open ? <Form connection={connection} onDone={() => onOpenChange(false)} /> : null}
      </DialogContent>
    </Dialog>
  );
}

function Form({ connection, onDone }: { connection?: Connection; onDone: () => void }) {
  const qc = useQueryClient();
  const navigate = useNavigate();
  const [f, setF] = React.useState<FormState>(() => initial(connection));
  const [errors, setErrors] = React.useState<Partial<Record<keyof FormState, string>>>({});
  const [testing, setTesting] = React.useState(false);
  const [tested, setTested] = React.useState<{ ok: boolean; text: string } | null>(null);
  const [saving, setSaving] = React.useState(false);
  const set = <K extends keyof FormState>(k: K, v: FormState[K]) => {
    setF((prev) => ({ ...prev, [k]: v }));
    setTested(null);
  };
  // Switching engines swaps in the other engine's defaults unless edited.
  const chooseEngine = (engine: Engine) => {
    const defaults: Record<string, { database: string; user: string }> = { postgres: { database: "postgres", user: "postgres" }, mysql: { database: "mysql", user: "root" } };
    setF((prev) => {
      const was = defaults[prev.engine];
      const next = defaults[engine];
      return {
        ...prev,
        engine,
        database: was && prev.database === was.database ? next.database : prev.database,
        user: was && prev.user === was.user ? next.user : prev.user,
      };
    });
    setTested(null);
  };
  const production = f.environment === "production";

  const input = (): ConnectionInput | null => {
    const e: typeof errors = {};
    if (!f.name.trim()) e.name = "Give the connection a name";
    if (!f.host.trim()) e.host = "Enter a host";
    if (!f.user.trim()) e.user = "Enter a user";
    const port = f.port.trim() ? Number(f.port) : defaultPort(f.engine);
    if (!Number.isInteger(port) || port <= 0 || port > 65535) e.port = "1–65535";
    setErrors(e);
    if (Object.keys(e).length) return null;
    const body: ConnectionInput = {
      name: f.name.trim(),
      engine: f.engine,
      environment: f.environment,
      host: f.host.trim(),
      port,
      database: f.database.trim() || undefined,
      user: f.user.trim(),
      tls: f.tls,
      readOnly: production ? true : f.readOnly,
    };
    if (!connection || f.secret) body.secret = f.secret;
    return body;
  };

  const test = async () => {
    const body = input();
    if (!body) return;
    if (connection && !f.secret) {
      setTested({ ok: false, text: "Enter the password to test new settings" });
      return;
    }
    setTesting(true);
    try {
      const res = await testConnection(body);
      setTested({ ok: true, text: `${res.overview.version} · ${duration(res.latencyMs)}` });
    } catch (err) {
      setTested({ ok: false, text: errorMessage(err) });
    } finally {
      setTesting(false);
    }
  };

  const save = async (ev: React.FormEvent) => {
    ev.preventDefault();
    const body = input();
    if (!body) return;
    setSaving(true);
    try {
      const saved = connection ? await updateConnection(connection.id, body) : await createConnection(body);
      await qc.invalidateQueries({ queryKey: getListConnectionsQueryKey() });
      await qc.invalidateQueries({ queryKey: getGetConnectionQueryKey(saved.id) });
      toast.success(connection ? `Saved ${saved.name}` : `Added ${saved.name}`);
      onDone();
      if (!connection) void navigate({ to: "/c/$connectionId", params: { connectionId: saved.id } });
    } catch (err) {
      toast.error(errorMessage(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <form onSubmit={save}>
      <DialogHead>
        <DialogTitle>{connection ? `Edit ${connection.name}` : "New connection"}</DialogTitle>
        <DialogDescription>Credentials are encrypted at rest with AES-256-GCM.</DialogDescription>
      </DialogHead>
      <DialogBody>
        <div className="m-field">
          <span className="m-label" id="engine-l">
            Engine
          </span>
          <div role="radiogroup" aria-labelledby="engine-l" style={{ display: "grid", gridTemplateColumns: "repeat(3, minmax(0, 1fr))", gap: 8 }}>
            <Choice selected={f.engine === "postgres"} onClick={() => chooseEngine("postgres")}>
              <Database className="m-icon" aria-hidden="true" />
              <span style={{ textAlign: "left" }}>
                <span className="m-label" style={{ display: "block" }}>
                  PostgreSQL
                </span>
                <span className="m-choice__sub">12 – 17</span>
              </span>
            </Choice>
            <Choice selected={f.engine === "mysql"} onClick={() => chooseEngine("mysql")}>
              <Database className="m-icon" aria-hidden="true" />
              <span style={{ textAlign: "left" }}>
                <span className="m-label" style={{ display: "block" }}>
                  MySQL
                </span>
                <span className="m-choice__sub">8.0 and later</span>
              </span>
            </Choice>
            <Choice selected={false} disabled>
              <Database className="m-icon" aria-hidden="true" />
              <span style={{ textAlign: "left" }}>
                <span className="m-label" style={{ display: "block" }}>
                  SQLite
                </span>
                <span className="m-choice__sub">Coming soon</span>
              </span>
            </Choice>
          </div>
        </div>
        <Field label="Name" htmlFor="conn-name" error={errors.name}>
          <input id="conn-name" className="m-input" value={f.name} onChange={(e) => set("name", e.target.value)} placeholder="billing" autoFocus aria-invalid={!!errors.name} />
        </Field>
        <div className="m-field">
          <span className="m-label" id="env-l">
            Environment
          </span>
          <div role="radiogroup" aria-labelledby="env-l" style={{ display: "grid", gridTemplateColumns: "repeat(4, minmax(0, 1fr))", gap: 8 }}>
            {ENVIRONMENTS.map((env) => (
              <Choice key={env} selected={f.environment === env} onClick={() => set("environment", env)} style={{ padding: "6px 10px" }}>
                <span className={`m-env m-env--${env}`} style={{ border: 0, background: "transparent", padding: 0 }}>
                  {envLabel(env)}
                </span>
              </Choice>
            ))}
          </div>
        </div>
        <div style={{ display: "grid", gridTemplateColumns: "minmax(0, 1fr) 96px", gap: 12 }}>
          <Field label="Host" htmlFor="conn-host" error={errors.host}>
            <input id="conn-host" className="m-input m-input--mono" value={f.host} onChange={(e) => set("host", e.target.value)} aria-invalid={!!errors.host} />
          </Field>
          <Field label="Port" htmlFor="conn-port" error={errors.port}>
            <input id="conn-port" className="m-input m-input--mono" inputMode="numeric" value={f.port} placeholder={String(defaultPort(f.engine))} onChange={(e) => set("port", e.target.value)} aria-invalid={!!errors.port} />
          </Field>
        </div>
        <div style={{ display: "grid", gridTemplateColumns: "repeat(2, minmax(0, 1fr))", gap: 12 }}>
          <Field label="User" htmlFor="conn-user" error={errors.user}>
            <input id="conn-user" className="m-input m-input--mono" value={f.user} onChange={(e) => set("user", e.target.value)} autoComplete="off" aria-invalid={!!errors.user} />
          </Field>
          <Field label="Password" htmlFor="conn-secret" help={connection ? "Leave empty to keep the stored one" : "Or env:NAME to read it from the server"}>
            <input id="conn-secret" className="m-input m-input--mono" type="password" value={f.secret} onChange={(e) => set("secret", e.target.value)} autoComplete="new-password" />
          </Field>
        </div>
        <div style={{ display: "grid", gridTemplateColumns: "repeat(2, minmax(0, 1fr))", gap: 12 }}>
          <Field label="Database" htmlFor="conn-db" help="Opened first; every database on the server is listed">
            <input id="conn-db" className="m-input m-input--mono" value={f.database} onChange={(e) => set("database", e.target.value)} />
          </Field>
          <Field label="TLS">
            <Select aria-label="TLS" value={f.tls} options={TLS_OPTIONS} onChange={(v) => set("tls", v)} />
          </Field>
        </div>
        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 12 }}>
          <span>
            <span className="m-label">Read-only by default</span>
            <span className="m-help" style={{ display: "block" }}>
              {production ? "Production is always read-only; unlock it for a while when you need to write." : "Writes need an unlock first."}
            </span>
          </span>
          <Switch aria-label="Read-only by default" checked={production || f.readOnly} disabled={production} onChange={(v) => set("readOnly", v)} />
        </div>
      </DialogBody>
      <DialogFoot>
        <Button variant="outline" onClick={test} disabled={testing}>
          <Plug className="m-icon m-icon--sm" aria-hidden="true" />
          {testing ? "Testing…" : "Test connection"}
        </Button>
        {tested ? <StatusLabel status={tested.ok ? "online" : "down"} label={tested.ok ? "Connected" : "Failed"} meta={tested.text} className="min-w-0" /> : null}
        <span className="m-spacer" />
        <Button variant="ghost" onClick={onDone}>
          Cancel
        </Button>
        <Button variant="primary" type="submit" disabled={saving}>
          {saving ? "Saving…" : "Save connection"}
        </Button>
      </DialogFoot>
    </form>
  );
}
