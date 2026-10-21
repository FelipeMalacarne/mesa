/**
 * The status bar: connection health and latency, the read-only lock, facts from the active
 * view, page buttons, and staged changes with Discard / Review & commit.
 */
import * as React from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { ChevronLeft, ChevronRight, Lock, LockOpen } from "lucide-react";
import { toast } from "sonner";

import { getGetConnectionQueryKey, getListConnectionsQueryKey, lockConnection, pingConnection, unlockConnection } from "@/lib/api/connections/connections";
import { errorMessage } from "@/lib/api/fetch";
import { duration } from "@/lib/format";
import { Button } from "@/ui/Button";
import { Kbd, StatusLabel } from "@/ui/controls";
import { useStatus } from "@/app/status.store";

import { useWorkspace } from "../workspace/context";

export function StatusBar() {
  const { connection, connectionId, readOnly } = useWorkspace();
  const qc = useQueryClient();
  const facts = useStatus((s) => s.facts);
  const paging = useStatus((s) => s.paging);
  const pending = useStatus((s) => s.pending);
  const ping = useQuery({ queryKey: ["ping", connectionId], queryFn: () => pingConnection(connectionId), refetchInterval: 30_000, retry: false, staleTime: 10_000 });
  const [, tick] = React.useReducer((x: number) => x + 1, 0);
  React.useEffect(() => {
    const t = setInterval(tick, 30_000);
    return () => clearInterval(t);
  }, []);

  const refresh = async () => {
    await qc.invalidateQueries({ queryKey: getGetConnectionQueryKey(connectionId) });
    await qc.invalidateQueries({ queryKey: getListConnectionsQueryKey() });
  };
  const unlock = async () => {
    try {
      await unlockConnection(connectionId, { ttlSeconds: 15 * 60 });
      await refresh();
      toast.success(`${connection.name} is writable for 15 minutes`);
    } catch (e) {
      toast.error(errorMessage(e));
    }
  };
  const lock = async () => {
    try {
      await lockConnection(connectionId);
      await refresh();
    } catch (e) {
      toast.error(errorMessage(e));
    }
  };
  const minutesLeft = connection.unlockedUntil ? Math.max(0, Math.ceil((Date.parse(connection.unlockedUntil) - Date.now()) / 60_000)) : 0;

  return (
    <footer className="m-statusbar" style={{ height: "var(--size-statusbar)", flex: "none" }}>
      {ping.isPending ? (
        <StatusLabel status="unknown" label="Connecting" />
      ) : ping.isError ? (
        <span title={errorMessage(ping.error)} style={{ color: "var(--foreground)" }}>
          <StatusLabel status="down" label="Offline" />
        </span>
      ) : (
        <span style={{ color: "var(--foreground)" }}>
          <StatusLabel status="online" label="Online" meta={duration(ping.data.latencyMs)} />
        </span>
      )}
      {connection.readOnly ? (
        <>
          <span className="m-sep" />
          {readOnly ? (
            <Button variant="ghost" size="xs" onClick={unlock} title="Allow writes for 15 minutes">
              <Lock className="m-icon m-icon--sm" aria-hidden="true" />
              Read-only · Unlock
            </Button>
          ) : (
            <Button variant="ghost" size="xs" onClick={lock} title="Lock now" style={{ color: "var(--warning)" }}>
              <LockOpen className="m-icon m-icon--sm" aria-hidden="true" />
              Unlocked · {minutesLeft} min
            </Button>
          )}
        </>
      ) : null}
      {facts.map((f, i) => (
        <React.Fragment key={i}>
          <span className="m-sep" />
          <span className="m-mono" style={{ overflow: "hidden", textOverflow: "ellipsis" }}>
            {f}
          </span>
        </React.Fragment>
      ))}
      {paging && paging.pages > 1 ? (
        <>
          <span className="m-sep" />
          <Button variant="ghost" size="xs" icon aria-label="Previous page" disabled={paging.page <= 1} onClick={paging.onPrev}>
            <ChevronLeft className="m-icon m-icon--sm" aria-hidden="true" />
          </Button>
          <span>
            Page {paging.page} of {paging.pages}
          </span>
          <Button variant="ghost" size="xs" icon aria-label="Next page" disabled={paging.page >= paging.pages} onClick={paging.onNext}>
            <ChevronRight className="m-icon m-icon--sm" aria-hidden="true" />
          </Button>
        </>
      ) : null}
      <span className="m-spacer" />
      {pending ? (
        <>
          <span className="m-pending">{pending.summary}</span>
          <Button variant="ghost" size="xs" onClick={pending.onDiscard}>
            Discard
          </Button>
          <Button variant="primary" size="xs" onClick={pending.onReview}>
            Review &amp; commit <Kbd>⌘S</Kbd>
          </Button>
        </>
      ) : null}
    </footer>
  );
}
