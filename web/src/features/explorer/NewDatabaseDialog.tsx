/**
 * Create an empty database (`NewDatabase` board). It is a reviewed write like any other; once
 * created, the schema designer opens on it with an empty canvas.
 */
import * as React from "react";
import { useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";

import { createDatabase, getListDatabasesQueryKey } from "@/lib/api/explore/explore";
import type { Connection } from "@/lib/api/mesaAPI.schemas";
import { Button } from "@/ui/Button";
import { Field } from "@/ui/controls";
import { Dialog, DialogBody, DialogContent, DialogDescription, DialogFoot, DialogHead, DialogTitle } from "@/ui/primitives/dialog";
import { ReviewDialog } from "@/features/changes/ReviewDialog";

export function NewDatabaseDialog({ open, onOpenChange, connection }: { open: boolean; onOpenChange: (o: boolean) => void; connection: Connection }) {
  const [name, setName] = React.useState("");
  const [owner, setOwner] = React.useState("");
  const [review, setReview] = React.useState(false);
  const qc = useQueryClient();
  const navigate = useNavigate();
  const valid = /^[A-Za-z_][A-Za-z0-9_$-]{0,62}$/.test(name);
  return (
    <>
      <Dialog open={open} onOpenChange={onOpenChange}>
        <DialogContent width={480} aria-describedby={undefined}>
          <form
            onSubmit={(e) => {
              e.preventDefault();
              if (!valid) return;
              onOpenChange(false);
              setReview(true);
            }}
          >
            <DialogHead>
              <DialogTitle>New database</DialogTitle>
              <DialogDescription>
                An empty database on <span className="m-mono">{connection.host}</span>. The schema designer opens on it next, so you can draw its tables or write them in DBML.
              </DialogDescription>
            </DialogHead>
            <DialogBody>
              <Field label="Name" htmlFor="newdb-name" error={name && !valid ? "Letters, digits and underscores; start with a letter" : undefined}>
                <input id="newdb-name" className="m-input m-input--mono" value={name} onChange={(e) => setName(e.target.value)} placeholder="analytics" autoFocus autoComplete="off" />
              </Field>
              <Field label="Owner" htmlFor="newdb-owner" help={`Defaults to ${connection.user}`}>
                <input id="newdb-owner" className="m-input m-input--mono" value={owner} onChange={(e) => setOwner(e.target.value)} placeholder={connection.user} autoComplete="off" />
              </Field>
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
      <ReviewDialog
        open={review}
        onOpenChange={setReview}
        title="Create database"
        target={connection.host}
        connection={connection}
        reviewKey={["create-db", name, owner]}
        transactional={false}
        applyLabel={`Create ${name}`}
        load={async () => {
          const r = await createDatabase(connection.id, { name, owner: owner || undefined, dryRun: true });
          return { plan: r.plan, decision: r.decision, upSql: r.upSql, downSql: r.downSql };
        }}
        apply={({ confirmation }) => createDatabase(connection.id, { name, owner: owner || undefined, confirmation })}
        onApplied={async () => {
          await qc.invalidateQueries({ queryKey: getListDatabasesQueryKey(connection.id) });
          const created = name;
          setName("");
          setOwner("");
          void navigate({ to: "/c/$connectionId/designer", params: { connectionId: connection.id }, search: { database: created, fresh: true } });
        }}
      />
    </>
  );
}
