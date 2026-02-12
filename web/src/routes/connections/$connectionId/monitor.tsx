import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Skeleton } from "@/components/ui/skeleton";
import {
  useConnectionSessions,
  useKillSession,
} from "@/hooks/use-connection";

export const Route = createFileRoute("/connections/$connectionId/monitor")({
  component: ConnectionMonitor,
});

function ConnectionMonitor() {
  const { connectionId } = Route.useParams();
  const { data, isLoading } = useConnectionSessions(connectionId);

  if (isLoading) {
    return <Skeleton className="h-64 w-full" />;
  }

  return (
    <div className="space-y-4">
      <div>
        <h2 className="text-xl font-semibold">Active Sessions</h2>
        <p className="text-muted-foreground text-sm">
          Polls every 5 seconds for fresh data.
        </p>
      </div>

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>PID</TableHead>
            <TableHead>User</TableHead>
            <TableHead>Database</TableHead>
            <TableHead>State</TableHead>
            <TableHead>Duration</TableHead>
            <TableHead>Query</TableHead>
            <TableHead></TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {data?.map((session) => (
            <TableRow key={session.pid}>
              <TableCell>{session.pid}</TableCell>
              <TableCell>{session.user}</TableCell>
              <TableCell>{session.database}</TableCell>
              <TableCell>{session.state}</TableCell>
              <TableCell>
                <div className="flex items-center gap-2">
                  {session.duration}
                  {session.is_slow && <Badge variant="destructive">Slow</Badge>}
                </div>
              </TableCell>
              <TableCell className="max-w-xl truncate">
                <code className="text-xs text-muted-foreground">
                  {session.query}
                </code>
              </TableCell>
              <TableCell className="text-right">
                <KillSessionButton connectionId={connectionId} pid={session.pid} />
              </TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

function KillSessionButton({
  connectionId,
  pid,
}: {
  connectionId: string;
  pid: number;
}) {
  const [open, setOpen] = useState(false);
  const mutation = useKillSession(connectionId);

  const handleConfirm = async () => {
    try {
      await mutation.mutateAsync(pid);
      toast.success(`Session ${pid} terminated`);
      setOpen(false);
    } catch (error) {
      console.error(error);
      toast.error("Failed to kill session");
    }
  };

  return (
    <AlertDialog open={open} onOpenChange={setOpen}>
      <AlertDialogTrigger asChild>
        <Button size="sm" variant="destructive">
          Kill
        </Button>
      </AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogHeader>
          <AlertDialogTitle>Terminate PID {pid}?</AlertDialogTitle>
          <AlertDialogDescription>
            This will cancel the running query and rollback any uncommitted
            transactions.
          </AlertDialogDescription>
        </AlertDialogHeader>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction onClick={handleConfirm} disabled={mutation.isPending}>
            {mutation.isPending ? "Terminating..." : "Confirm"}
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
