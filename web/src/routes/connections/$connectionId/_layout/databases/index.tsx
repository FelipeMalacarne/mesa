import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import {
  Table,
  TableBody,
  TableCaption,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Skeleton } from "@/components/ui/skeleton";
import {
  useConnectionDatabases,
  useCreateDatabase,
} from "@/hooks/use-connection";

export const Route = createFileRoute("/connections/$connectionId/_layout/databases/")({
  component: ConnectionDatabases,
});

function ConnectionDatabases() {
  const { connectionId } = Route.useParams();
  const { data, isLoading } = useConnectionDatabases(connectionId);

  if (isLoading) {
    return <Skeleton className="h-48 w-full" />;
  }

  const databases = data?.databases ?? [];

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-semibold">Databases</h2>
          <p className="text-muted-foreground text-sm">
            Track existing databases and create new tenants.
          </p>
        </div>
        <CreateDatabaseDialog connectionId={connectionId} />
      </div>

      {data?.status === "error" ? (
        <div className="rounded-md border border-destructive/50 bg-destructive/10 p-4 text-sm text-destructive">
          {data.error ?? "Failed to load databases"}
        </div>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Name</TableHead>
              <TableHead>Owner</TableHead>
              <TableHead>Encoding</TableHead>
              <TableHead>Size</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {databases.map((database) => (
              <TableRow key={database.name}>
                <TableCell className="font-medium">{database.name}</TableCell>
                <TableCell>{database.owner}</TableCell>
                <TableCell>{database.encoding}</TableCell>
                <TableCell>{database.size_formatted}</TableCell>
              </TableRow>
            ))}
          </TableBody>
          {databases.length === 0 && (
            <TableCaption>No databases found for this connection.</TableCaption>
          )}
        </Table>
      )}
    </div>
  );
}

function CreateDatabaseDialog({ connectionId }: { connectionId: string }) {
  const [open, setOpen] = useState(false);
  const [name, setName] = useState("");
  const mutation = useCreateDatabase(connectionId);

  const handleCreate = async () => {
    if (!name.trim()) return;
    try {
      await mutation.mutateAsync(name.trim());
      toast.success("Database created");
      setName("");
      setOpen(false);
    } catch (error) {
      console.error(error);
      toast.error("Failed to create database");
    }
  };

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button>Create Database</Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>New database</DialogTitle>
        </DialogHeader>
        <div className="space-y-3">
          <Label htmlFor="db-name">Name</Label>
          <Input
            id="db-name"
            value={name}
            onChange={(event) => setName(event.target.value)}
            placeholder="analytics"
          />
        </div>
        <DialogFooter>
          <Button onClick={handleCreate} disabled={mutation.isPending}>
            {mutation.isPending ? "Creating..." : "Create"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
