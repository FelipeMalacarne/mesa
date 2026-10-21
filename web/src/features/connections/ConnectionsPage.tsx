/**
 * The home screen: every connection with its environment, engine, host, live status and when
 * it was last opened (`Connections` board). Status comes from a ping per row.
 */
import * as React from "react";
import { useQueries, useQueryClient } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import { Database, Info, Lock, MoreHorizontal, Pencil, Plus, Search, Trash2 } from "lucide-react";
import { toast } from "sonner";

import { deleteConnection, getListConnectionsQueryKey, pingConnection, useListConnections } from "@/lib/api/connections/connections";
import type { Connection, Environment } from "@/lib/api/mesaAPI.schemas";
import { errorMessage } from "@/lib/api/fetch";
import { duration, ENVIRONMENTS, envLabel, relativeTime } from "@/lib/format";
import { Button } from "@/ui/Button";
import { EmptyState, EnvTag, Kbd, StatusLabel } from "@/ui/controls";
import { Popover, PopoverContent, PopoverTrigger } from "@/ui/primitives/popover";
import { Dialog, DialogBody, DialogContent, DialogDescription, DialogFoot, DialogHead, DialogTitle } from "@/ui/primitives/dialog";
import { ThemeToggle } from "@/app/ThemeToggle";

import { ConnectionDialog } from "./ConnectionDialog";

type EnvFilter = "all" | Environment;

export function ConnectionsPage() {
  const list = useListConnections();
  const navigate = useNavigate();
  const [search, setSearch] = React.useState("");
  const [env, setEnv] = React.useState<EnvFilter>("all");
  const [dialog, setDialog] = React.useState<{ open: boolean; connection?: Connection }>({ open: false });
  const [removing, setRemoving] = React.useState<Connection | null>(null);
  const searchRef = React.useRef<HTMLInputElement>(null);

  React.useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "/" && document.activeElement?.tagName !== "INPUT" && document.activeElement?.tagName !== "TEXTAREA") {
        e.preventDefault();
        searchRef.current?.focus();
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  const connections = list.data ?? [];
  const pings = useQueries({
    queries: connections.map((c) => ({
      queryKey: ["ping", c.id],
      queryFn: () => pingConnection(c.id),
      staleTime: 30_000,
      retry: false,
    })),
  });
  const statusOf = (i: number) => pings[i];

  const shown = connections
    .map((c, i) => ({ c, i }))
    .filter(({ c }) => (env === "all" || c.environment === env) && `${c.name} ${c.host} ${c.database}`.toLowerCase().includes(search.toLowerCase()));

  return (
    <div className="m-app" style={{ minHeight: "100%", overflow: "auto" }}>
      <div className="m-page">
        <header className="m-page__head">
          <div className="m-page__title">
            <h1 className="text-page-title">Connections</h1>
            <span className="m-badge m-badge--count">{connections.length}</span>
          </div>
          <div className="m-input-group" style={{ width: 280, maxWidth: "100%" }}>
            <Search className="m-icon m-icon--sm" aria-hidden="true" />
            <input ref={searchRef} className="m-input" placeholder="Search connections" aria-label="Search connections" value={search} onChange={(e) => setSearch(e.target.value)} />
            <Kbd>/</Kbd>
          </div>
          <div className="m-seg" role="tablist" aria-label="Environment">
            {(["all", ...ENVIRONMENTS] as EnvFilter[]).map((e) => (
              <button key={e} type="button" role="tab" aria-selected={env === e} className={`m-seg__item ${env === e ? "is-active" : ""}`} onClick={() => setEnv(e)}>
                {e === "all" ? "All" : envLabel(e)}
              </button>
            ))}
          </div>
          <Button variant="primary" onClick={() => setDialog({ open: true })}>
            <Plus className="m-icon m-icon--sm" aria-hidden="true" />
            New connection
          </Button>
          <ThemeToggle />
        </header>

        {list.isError ? <p className="m-help m-help--error">{errorMessage(list.error)}</p> : null}
        {list.isSuccess && connections.length === 0 ? (
          <EmptyState
            icon={<Database className="m-icon" aria-hidden="true" />}
            title="No connections yet"
            description="Add a PostgreSQL server to browse its databases, edit rows with a review step, watch its metrics and design its schema."
            actions={
              <Button variant="primary" onClick={() => setDialog({ open: true })}>
                <Plus className="m-icon m-icon--sm" aria-hidden="true" />
                New connection
              </Button>
            }
          />
        ) : null}

        {connections.length > 0 ? (
          <div className="m-grid-wrap" style={{ overflowX: "auto" }}>
            <table className="m-grid m-grid--ui" style={{ width: "100%" }}>
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Environment</th>
                  <th>Engine</th>
                  <th>Host</th>
                  <th>Status</th>
                  <th>Last opened</th>
                  <th aria-label="Actions" style={{ width: 48 }} />
                </tr>
              </thead>
              <tbody>
                {shown.map(({ c, i }) => {
                  const p = statusOf(i);
                  return (
                    <tr key={c.id} className="is-link" onClick={() => void navigate({ to: "/c/$connectionId", params: { connectionId: c.id } })}>
                      <td>
                        <Link to="/c/$connectionId" params={{ connectionId: c.id }} style={{ display: "inline-flex", alignItems: "center", gap: 8, fontWeight: 500 }} onClick={(e) => e.stopPropagation()}>
                          <Database className="m-icon m-icon--sm m-muted" aria-hidden="true" />
                          {c.name}
                          {c.readOnly ? <Lock className="m-icon m-icon--sm m-muted" aria-label="Read-only" /> : null}
                        </Link>
                      </td>
                      <td>
                        <EnvTag env={c.environment} />
                      </td>
                      <td>
                        <span className="m-badge m-badge--mono">{c.engine === "postgres" ? "pg" : c.engine}</span>
                      </td>
                      <td className="m-mono">
                        {c.host}:{c.port}/{c.database}
                      </td>
                      <td>
                        {!p || p.isPending ? (
                          <StatusLabel status="unknown" label="Checking" />
                        ) : p.isError ? (
                          <span title={errorMessage(p.error)}>
                            <StatusLabel status="down" label="Unreachable" />
                          </span>
                        ) : (
                          <StatusLabel status="online" label="Online" meta={`${p.data.overview.version} · ${duration(p.data.latencyMs)}`} />
                        )}
                      </td>
                      <td className="m-muted">{relativeTime(c.lastOpenedAt)}</td>
                      <td onClick={(e) => e.stopPropagation()}>
                        <RowMenu onEdit={() => setDialog({ open: true, connection: c })} onDelete={() => setRemoving(c)} />
                      </td>
                    </tr>
                  );
                })}
                {shown.length === 0 ? (
                  <tr>
                    <td colSpan={7} className="m-muted">
                      No connection matches.
                    </td>
                  </tr>
                ) : null}
              </tbody>
            </table>
          </div>
        ) : null}
        <p className="m-help" style={{ margin: 0, display: "flex", alignItems: "center", gap: 6 }}>
          <Info className="m-icon m-icon--sm" aria-hidden="true" />
          Production connections open read-only. Unlock per session from the status bar.
        </p>
      </div>
      <ConnectionDialog open={dialog.open} connection={dialog.connection} onOpenChange={(open) => setDialog((d) => ({ ...d, open }))} />
      <RemoveDialog connection={removing} onClose={() => setRemoving(null)} />
    </div>
  );
}

function RowMenu({ onEdit, onDelete }: { onEdit: () => void; onDelete: () => void }) {
  const [open, setOpen] = React.useState(false);
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button variant="ghost" size="sm" icon aria-label="Connection actions">
          <MoreHorizontal className="m-icon m-icon--sm" aria-hidden="true" />
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end">
        <button
          type="button"
          className="m-menu__item"
          onClick={() => {
            setOpen(false);
            onEdit();
          }}
        >
          <Pencil className="m-icon m-icon--sm" aria-hidden="true" />
          Edit
        </button>
        <div className="m-menu__sep" />
        <button
          type="button"
          className="m-menu__item is-danger"
          onClick={() => {
            setOpen(false);
            onDelete();
          }}
        >
          <Trash2 className="m-icon m-icon--sm" aria-hidden="true" />
          Remove
        </button>
      </PopoverContent>
    </Popover>
  );
}

function RemoveDialog({ connection, onClose }: { connection: Connection | null; onClose: () => void }) {
  const qc = useQueryClient();
  const [busy, setBusy] = React.useState(false);
  return (
    <Dialog open={!!connection} onOpenChange={(o) => !o && onClose()}>
      <DialogContent width={440} aria-describedby={undefined}>
        <DialogHead>
          <DialogTitle>Remove {connection?.name}?</DialogTitle>
          <DialogDescription>Mesa forgets the settings and the stored password. The database itself is not touched.</DialogDescription>
        </DialogHead>
        <DialogBody>
          <span />
        </DialogBody>
        <DialogFoot>
          <span className="m-spacer" />
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button
            variant="destructive"
            disabled={busy}
            onClick={async () => {
              if (!connection) return;
              setBusy(true);
              try {
                await deleteConnection(connection.id);
                await qc.invalidateQueries({ queryKey: getListConnectionsQueryKey() });
                toast.success(`Removed ${connection.name}`);
                onClose();
              } catch (e) {
                toast.error(errorMessage(e));
              } finally {
                setBusy(false);
              }
            }}
          >
            Remove
          </Button>
        </DialogFoot>
      </DialogContent>
    </Dialog>
  );
}
