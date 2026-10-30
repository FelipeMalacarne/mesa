/** Users and roles on the server. Creating and dropping a role are reviewed writes. */
import * as React from "react";
import { useQueryClient } from "@tanstack/react-query";
import { DatabaseZap, Plus, Trash2, Users } from "lucide-react";

import { useGetProvisioningTarget } from "@/lib/api/provision/provision";
import { createRole, dropRole, getListRolesQueryKey, useListRoles } from "@/lib/api/roles/roles";
import type { Role, RoleRequest } from "@/lib/api/mesaAPI.schemas";
import { errorMessage } from "@/lib/api/fetch";
import { plural } from "@/lib/format";
import { useStatusFacts } from "@/app/status.store";
import { Button } from "@/ui/Button";
import { EmptyState, Field, Switch } from "@/ui/controls";
import { Dialog, DialogBody, DialogContent, DialogDescription, DialogFoot, DialogHead, DialogTitle } from "@/ui/primitives/dialog";
import { useCan } from "@/app/auth";
import { ReviewDialog } from "@/features/changes/ReviewDialog";
import { ProvisionDialog } from "@/features/provision/ProvisionDialog";
import { useTabs } from "@/features/explorer/tabs.store";

import { useWorkspace } from "../workspace/context";

export function RolesPage() {
  const { connectionId, connection } = useWorkspace();
  const qc = useQueryClient();
  const roles = useListRoles(connectionId);
  const [form, setForm] = React.useState(false);
  const [pending, setPending] = React.useState<{ kind: "create"; req: RoleRequest } | { kind: "drop"; role: Role } | null>(null);
  const [search, setSearch] = React.useState("");
  const [provision, setProvision] = React.useState(false);
  const admin = useCan("admin");
  // Connections declared with a GitOps repository can provision app databases.
  const target = useGetProvisioningTarget(connectionId, { query: { enabled: admin, retry: false, staleTime: 5 * 60_000 } });

  React.useEffect(() => {
    useTabs.getState().open(connectionId, { id: "roles", kind: "roles", title: "Users & roles" });
  }, [connectionId]);
  useStatusFacts(roles.data ? [plural(roles.data.length, "role"), `${roles.data.filter((r) => r.canLogin).length} can log in`] : []);

  const list = (roles.data ?? []).filter((r) => r.name.toLowerCase().includes(search.toLowerCase()));
  const yes = (b: boolean) => (b ? "Yes" : <span className="m-muted">No</span>);

  return (
    <>
      <div className="m-toolbar" style={{ height: "var(--size-toolbar)", flex: "none" }}>
        <input className="m-input m-input--sm" style={{ width: 240 }} placeholder="Filter roles" aria-label="Filter roles" value={search} onChange={(e) => setSearch(e.target.value)} />
        <span className="m-spacer" />
        {target.data ? (
          <Button variant="secondary" size="sm" onClick={() => setProvision(true)} title={`Login and database, declared in ${target.data.repo}`}>
            <DatabaseZap className="m-icon m-icon--sm" aria-hidden="true" />
            App database
          </Button>
        ) : null}
        <Button variant="primary" size="sm" onClick={() => setForm(true)}>
          <Plus className="m-icon m-icon--sm" aria-hidden="true" />
          Role
        </Button>
      </div>
      <div style={{ flex: 1, minHeight: 0, overflow: "auto", padding: "16px 24px" }}>
        {roles.isError ? <EmptyState icon={<Users className="m-icon" aria-hidden="true" />} title="Cannot list roles" description={errorMessage(roles.error)} /> : null}
        {roles.data ? (
          <div className="m-grid-wrap">
            <table className="m-grid m-grid--ui" style={{ width: "100%" }}>
              <thead>
                <tr>
                  <th>Role</th>
                  <th>Can log in</th>
                  <th>Superuser</th>
                  <th>Create DB</th>
                  <th>Create role</th>
                  <th className="is-num">Connection limit</th>
                  <th>Member of</th>
                  <th>Valid until</th>
                  <th aria-label="Actions" />
                </tr>
              </thead>
              <tbody>
                {list.map((r) => (
                  <tr key={r.name}>
                    <td className="m-mono" style={{ fontWeight: 500 }}>
                      {r.name}
                    </td>
                    <td>{yes(r.canLogin)}</td>
                    <td>{r.superuser ? <span style={{ color: "var(--warning)" }}>Yes</span> : yes(false)}</td>
                    <td>{yes(r.createDb)}</td>
                    <td>{yes(r.createRole)}</td>
                    <td className="m-mono is-num">{r.connLimit < 0 ? "∞" : r.connLimit}</td>
                    <td className="m-mono">{(r.memberOf ?? []).join(", ")}</td>
                    <td className="m-muted">{r.validUntil ? r.validUntil.slice(0, 10) : ""}</td>
                    <td>
                      {!r.name.startsWith("pg_") && r.name !== connection.user ? (
                        <Button variant="ghost" size="xs" icon aria-label={`Drop ${r.name}`} onClick={() => setPending({ kind: "drop", role: r })}>
                          <Trash2 className="m-icon m-icon--sm" aria-hidden="true" />
                        </Button>
                      ) : null}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : null}
      </div>
      <RoleForm open={form} onOpenChange={setForm} roles={roles.data ?? []} onReview={(req) => setPending({ kind: "create", req })} />
      <ReviewDialog
        open={pending !== null}
        onOpenChange={(o) => !o && setPending(null)}
        title={pending?.kind === "drop" ? `Drop role ${pending.role.name}` : `Create role ${pending?.kind === "create" ? pending.req.name : ""}`}
        target={connection.host}
        connection={connection}
        reviewKey={["role", JSON.stringify(pending)]}
        load={async () => {
          const r = pending?.kind === "drop" ? await dropRole(connectionId, pending.role.name, { dryRun: true }) : await createRole(connectionId, { ...(pending?.kind === "create" ? pending.req : { name: "" }), dryRun: true });
          return { plan: r.plan, decision: r.decision, upSql: r.upSql, downSql: r.downSql };
        }}
        apply={({ confirmation }) => (pending?.kind === "drop" ? dropRole(connectionId, pending.role.name, { confirmation }) : createRole(connectionId, { ...(pending?.kind === "create" ? pending.req : { name: "" }), confirmation }))}
        onApplied={async () => {
          await qc.invalidateQueries({ queryKey: getListRolesQueryKey(connectionId) });
        }}
      />
      {target.data ? <ProvisionDialog open={provision} onOpenChange={setProvision} connection={connection} target={target.data} /> : null}
    </>
  );
}

function RoleForm({ open, onOpenChange, roles, onReview }: { open: boolean; onOpenChange: (o: boolean) => void; roles: Role[]; onReview: (r: RoleRequest) => void }) {
  const [r, setR] = React.useState<RoleRequest>({ name: "", canLogin: true });
  const [groups, setGroups] = React.useState<string[]>([]);
  React.useEffect(() => {
    if (open) {
      setR({ name: "", canLogin: true, password: "", connLimit: -1 });
      setGroups([]);
    }
  }, [open]);
  const taken = roles.some((x) => x.name === r.name);
  const valid = /^[A-Za-z_][A-Za-z0-9_$-]*$/.test(r.name) && !taken;
  const groupRoles = roles.filter((x) => !x.canLogin && !x.name.startsWith("pg_"));
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent width={520} aria-describedby={undefined}>
        <form
          onSubmit={(e) => {
            e.preventDefault();
            if (!valid) return;
            onOpenChange(false);
            onReview({ ...r, password: r.password || undefined, memberOf: groups.length ? groups : undefined });
          }}
        >
          <DialogHead>
            <DialogTitle>New role</DialogTitle>
            <DialogDescription>A login (user) or a group. The password is sent once and never shown in the review or the audit log.</DialogDescription>
          </DialogHead>
          <DialogBody>
            <Field label="Name" htmlFor="role-name" error={taken ? "A role has this name" : undefined}>
              <input id="role-name" className="m-input m-input--mono" value={r.name} onChange={(e) => setR({ ...r, name: e.target.value })} autoFocus autoComplete="off" />
            </Field>
            <Field label="Password" htmlFor="role-pass" help={r.canLogin ? "Leave empty for a login without a password (e.g. certificate auth)" : "Groups do not log in"}>
              <input id="role-pass" className="m-input m-input--mono" type="password" value={r.password ?? ""} onChange={(e) => setR({ ...r, password: e.target.value })} disabled={!r.canLogin} autoComplete="new-password" />
            </Field>
            {[
              { k: "canLogin" as const, label: "Can log in", help: "A user; off makes a group" },
              { k: "createDb" as const, label: "Create databases" },
              { k: "createRole" as const, label: "Create roles" },
              { k: "superuser" as const, label: "Superuser", help: "Bypasses every permission check" },
            ].map((o) => (
              <div key={o.k} className="m-row" style={{ justifyContent: "space-between", flexWrap: "nowrap" }}>
                <span>
                  <span className="m-label">{o.label}</span>
                  {o.help ? <span className="m-help" style={{ display: "block" }}>{o.help}</span> : null}
                </span>
                <Switch aria-label={o.label} checked={!!r[o.k]} onChange={(v) => setR({ ...r, [o.k]: v })} />
              </div>
            ))}
            {groupRoles.length ? (
              <div className="m-field">
                <span className="m-label">Member of</span>
                <div className="m-row">
                  {groupRoles.map((g) => (
                    <label key={g.name} className="m-row" style={{ cursor: "pointer", gap: 4 }}>
                      <input type="checkbox" checked={groups.includes(g.name)} onChange={() => setGroups(groups.includes(g.name) ? groups.filter((x) => x !== g.name) : [...groups, g.name])} style={{ accentColor: "var(--primary)" }} />
                      <span className="m-mono">{g.name}</span>
                    </label>
                  ))}
                </div>
              </div>
            ) : null}
          </DialogBody>
          <DialogFoot>
            <span className="m-spacer" />
            <Button variant="ghost" onClick={() => onOpenChange(false)}>
              Cancel
            </Button>
            <Button variant="primary" type="submit" disabled={!valid}>
              Review
            </Button>
          </DialogFoot>
        </form>
      </DialogContent>
    </Dialog>
  );
}
