import { createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import { toast } from "sonner";

import { Badge } from "@/components/ui/badge";
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
import { Switch } from "@/components/ui/switch";
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
  useConnectionUsers,
  useCreateUser,
} from "@/hooks/use-connection";

export const Route = createFileRoute("/connections/$connectionId/_layout/users")({
  component: ConnectionUsers,
});

function ConnectionUsers() {
  const { connectionId } = Route.useParams();
  const { data, isLoading } = useConnectionUsers(connectionId);

  if (isLoading) {
    return <Skeleton className="h-48 w-full" />;
  }

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-xl font-semibold">Users & Roles</h2>
          <p className="text-muted-foreground text-sm">
            Review privileges and create dedicated service accounts.
          </p>
        </div>
        <CreateUserDialog connectionId={connectionId} />
      </div>

      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Username</TableHead>
            <TableHead>Superuser</TableHead>
            <TableHead>Login</TableHead>
            <TableHead>Connection Limit</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {data?.map((user) => (
            <TableRow key={user.name}>
              <TableCell className="font-medium">{user.name}</TableCell>
              <TableCell>
                {user.is_superuser ? (
                  <Badge variant="secondary">Superuser</Badge>
                ) : (
                  <span className="text-muted-foreground">Standard</span>
                )}
              </TableCell>
              <TableCell>{user.can_login ? "Allowed" : "Disabled"}</TableCell>
              <TableCell>{user.conn_limit >= 0 ? user.conn_limit : "Unlimited"}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

function CreateUserDialog({ connectionId }: { connectionId: string }) {
  const [open, setOpen] = useState(false);
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [superuser, setSuperuser] = useState(false);
  const mutation = useCreateUser(connectionId);

  const handleCreate = async () => {
    if (!username.trim() || !password.trim()) return;
    try {
      await mutation.mutateAsync({
        username: username.trim(),
        password: password.trim(),
        is_superuser: superuser,
        can_login: true,
      });
      toast.success("User created");
      setUsername("");
      setPassword("");
      setSuperuser(false);
      setOpen(false);
    } catch (error) {
      console.error(error);
      toast.error("Failed to create user");
    }
  };

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button>Add User</Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>New user</DialogTitle>
        </DialogHeader>
        <div className="space-y-4">
          <div className="space-y-2">
            <Label htmlFor="username">Username</Label>
            <Input
              id="username"
              value={username}
              onChange={(event) => setUsername(event.target.value)}
            />
          </div>
          <div className="space-y-2">
            <Label htmlFor="password">Password</Label>
            <Input
              id="password"
              type="password"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
            />
          </div>
          <div className="flex items-center justify-between rounded-md border px-3 py-2">
            <div className="space-y-0.5">
              <Label>Superuser</Label>
              <p className="text-muted-foreground text-xs">
                Grants unrestricted access. Use carefully.
              </p>
            </div>
            <Switch
              checked={superuser}
              onCheckedChange={(checked) => setSuperuser(checked)}
            />
          </div>
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
