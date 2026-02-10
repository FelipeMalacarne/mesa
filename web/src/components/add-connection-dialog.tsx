import { Plus } from "lucide-react";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "./ui/dialog";
import { Button } from "./ui/button";
import { ConnectionForm } from "./connection-form";

export function AddConnectionDialog() {
  return (
    <Dialog>
      <DialogTrigger>
        <Button variant="ghost" size="icon" className="size-6">
          <Plus />
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Are you absolutely sure?</DialogTitle>
          <DialogDescription>
            This action cannot be undone. This will permanently delete your
            account and remove your data from our servers.
          </DialogDescription>
        </DialogHeader>
        <ConnectionForm />
      </DialogContent>
    </Dialog>
  );
}
