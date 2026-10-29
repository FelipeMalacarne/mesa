/**
 * Settings → Account and API tokens, on a server where people sign in. The identity provider
 * owns the name, email and role; Mesa shows them and keeps the person's API tokens, which act
 * with their role on the API and on /mcp.
 */
import * as React from "react";
import { useQueryClient } from "@tanstack/react-query";
import { Copy, KeyRound, LogOut, Trash2 } from "lucide-react";
import { toast } from "sonner";

import { createToken, getListTokensQueryKey, revokeToken, useListTokens } from "@/lib/api/auth/auth";
import type { CreatedToken } from "@/lib/api/mesaAPI.schemas";
import { errorMessage } from "@/lib/api/fetch";
import { Button } from "@/ui/Button";
import { EmptyState, Field } from "@/ui/controls";

import { Row } from "./SettingsRow";
import { useAuth, useSignOut } from "./auth";

const roleHelp = {
  viewer: "Reads, runs read-only queries and watches Monitor.",
  editor: "Also changes databases (always reviewed) and cancels sessions.",
  admin: "Also manages connections and provisions app databases.",
} as const;

export function AccountSettings() {
  const auth = useAuth();
  const signOut = useSignOut();
  const user = auth.data?.user;
  if (!user) return null;
  return (
    <>
      <h2 className="m-section-title">Account</h2>
      <Row label="Name">
        <span>{user.name}</span>
      </Row>
      <Row label="Email">
        <span className="m-mono">{user.email || "—"}</span>
      </Row>
      <Row label="Role" help={`${roleHelp[user.role]} Your role comes from ${auth.data?.provider ?? "the identity provider"}; it changes there.`}>
        <span className="m-badge m-badge--outline">{user.role}</span>
      </Row>
      <Row label="Session" help="Ends after 12 hours; signing in again is one click while you are signed in to the provider.">
        <Button variant="outline" size="sm" onClick={() => void signOut()}>
          <LogOut className="m-icon m-icon--sm" aria-hidden="true" />
          Sign out
        </Button>
      </Row>
    </>
  );
}

export function TokenSettings() {
  const qc = useQueryClient();
  const tokens = useListTokens();
  const [name, setName] = React.useState("");
  const [days, setDays] = React.useState("90");
  const [created, setCreated] = React.useState<CreatedToken | null>(null);
  const [busy, setBusy] = React.useState(false);

  const create = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    try {
      const t = await createToken({ name: name.trim(), expiresInDays: Number(days) || undefined });
      setCreated(t);
      setName("");
      await qc.invalidateQueries({ queryKey: getListTokensQueryKey() });
    } catch (err) {
      toast.error(errorMessage(err));
    } finally {
      setBusy(false);
    }
  };
  const revoke = async (id: string, label: string) => {
    try {
      await revokeToken(id);
      if (created?.token.id === id) setCreated(null);
      await qc.invalidateQueries({ queryKey: getListTokensQueryKey() });
      toast.success(`Revoked ${label}`);
    } catch (err) {
      toast.error(errorMessage(err));
    }
  };

  return (
    <>
      <h2 className="m-section-title">API tokens</h2>
      <p className="m-help" style={{ margin: 0 }}>
        For agents and scripts: send <span className="m-mono">Authorization: Bearer &lt;token&gt;</span> to <span className="m-mono">/mcp</span> or{" "}
        <span className="m-mono">/api</span>. A token acts with your role.
      </p>
      <form onSubmit={create} className="m-row" style={{ alignItems: "flex-end", flexWrap: "wrap" }}>
        <Field label="Name" htmlFor="token-name">
          <input id="token-name" className="m-input" value={name} onChange={(e) => setName(e.target.value)} placeholder="claude" autoComplete="off" />
        </Field>
        <Field label="Expires" htmlFor="token-days">
          <select id="token-days" className="m-input" value={days} onChange={(e) => setDays(e.target.value)}>
            <option value="30">in 30 days</option>
            <option value="90">in 90 days</option>
            <option value="365">in a year</option>
            <option value="0">never</option>
          </select>
        </Field>
        <Button type="submit" variant="primary" size="sm" disabled={busy || !name.trim()}>
          <KeyRound className="m-icon m-icon--sm" aria-hidden="true" />
          Create
        </Button>
      </form>
      {created ? (
        <div className="m-card" style={{ padding: 12, display: "grid", gap: 8 }}>
          <span className="m-label">Copy {created.token.name} now: Mesa will not show it again.</span>
          <div className="m-row" style={{ flexWrap: "nowrap" }}>
            <code className="m-mono" style={{ overflowWrap: "anywhere", flex: 1 }}>
              {created.secret}
            </code>
            <Button
              variant="outline"
              size="sm"
              onClick={() => void navigator.clipboard.writeText(created.secret).then(() => toast.success("Token copied"))}
            >
              <Copy className="m-icon m-icon--sm" aria-hidden="true" />
              Copy
            </Button>
          </div>
        </div>
      ) : null}
      {tokens.data && tokens.data.length === 0 ? <EmptyState title="No tokens yet" /> : null}
      {tokens.data?.map((t) => (
        <Row
          key={t.id}
          label={t.name}
          help={[
            `Created ${new Date(t.createdAt).toLocaleDateString()}`,
            t.lastUsedAt ? `last used ${new Date(t.lastUsedAt).toLocaleString()}` : "never used",
            t.expiresAt ? `expires ${new Date(t.expiresAt).toLocaleDateString()}` : "never expires",
          ].join(" · ")}
        >
          <Button variant="ghost" size="sm" onClick={() => void revoke(t.id, t.name)} aria-label={`Revoke ${t.name}`}>
            <Trash2 className="m-icon m-icon--sm" aria-hidden="true" />
            Revoke
          </Button>
        </Row>
      ))}
    </>
  );
}
