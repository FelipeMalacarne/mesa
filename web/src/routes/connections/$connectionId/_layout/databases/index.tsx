import { createFileRoute } from "@tanstack/react-router";
import { useEffect, useState } from "react";
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
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "@/components/ui/select";
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
  useConnectionUsers,
  useCreateDatabase,
} from "@/hooks/use-connection";

export const Route = createFileRoute("/connections/$connectionId/_layout/databases/")({
  component: ConnectionDatabases,
});

function ConnectionDatabases() {
  const { connectionId } = Route.useParams();
  const { data, isLoading, isError, error } = useConnectionDatabases(connectionId);

  if (isLoading) {
    return <Skeleton className="h-48 w-full" />;
  }

  const databases = data ?? [];

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

      {isError ? (
        <div className="rounded-md border border-destructive/50 bg-destructive/10 p-4 text-sm text-destructive">
          {error?.message ?? "Failed to load databases"}
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
  const [owner, setOwner] = useState("");
  const mutation = useCreateDatabase(connectionId);
  const { data: users, isLoading: isUsersLoading } = useConnectionUsers(connectionId);

  useEffect(() => {
    if (!owner && users && users.length > 0) {
      setOwner(users[0].name);
    }
  }, [owner, users]);

  const handleCreate = async () => {
    const trimmedName = name.trim();
    const trimmedOwner = owner.trim();
    if (!trimmedName || !trimmedOwner) return;
    try {
      await mutation.mutateAsync({ name: trimmedName, owner: trimmedOwner });
      toast.success("Database created");
      setName("");
      setOwner("");
      setOpen(false);
    } catch (error) {
      console.error(error);
      toast.error("Failed to create database");
    }
  };

  const hasOwnerOptions = Boolean(users?.length);
  const isCreateDisabled =
    mutation.isPending || !name.trim() || !owner.trim();
  const ownerPlaceholder = isUsersLoading
    ? "Loading owners..."
    : hasOwnerOptions
    ? "Select an owner"
    : "No owners available";

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button>Create Database</Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>New database</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="db-name">Name</Label>
            <Input
              id="db-name"
              value={name}
              onChange={(event) => setName(event.target.value)}
              placeholder="analytics"
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="db-owner">Owner</Label>
            <Select
              value={owner || undefined}
              onValueChange={setOwner}
              disabled={!hasOwnerOptions || isUsersLoading}
            >
              <SelectTrigger>
                <SelectValue placeholder={ownerPlaceholder} />
              </SelectTrigger>
              <SelectContent>
                {users?.map((user) => (
                  <SelectItem key={user.name} value={user.name}>
                    {user.name}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            {!isUsersLoading && !hasOwnerOptions && (
              <p className="text-sm text-muted-foreground">
                Create a user first to assign as the owner.
              </p>
            )}
          </div>
        </div>
        <DialogFooter>
          <Button onClick={handleCreate} disabled={isCreateDisabled}>
            {mutation.isPending ? "Creating..." : "Create"}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
