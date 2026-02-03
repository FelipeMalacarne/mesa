import { ChevronDown } from "lucide-react";
import { Button } from "./ui/button";
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "./ui/dropdown-menu";
import { useState } from "react";

interface Connection {
  id: string;
  name: string;
}

export function ConnectionsDropdown() {
  const [connections, setConnections] = useState<Connection[]>([]);
  const [currentConnection, setCurrentConnection] = useState<string | null>();

  const selectedConnection = connections.find(
    (c) => c.id === currentConnection,
  );

  const onConnectionChange = (id: string) => {
    console.log(`Connection changed ! (ID: ${id})`);
  };

  return (
    <>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button
            variant="outline"
            className="gap-2 border-border bg-secondary text-secondary-foreground"
          >
            <span className="h-2 w-2 rounded-full bg-primary" />
            {selectedConnection?.name || "Select Connection"}
            <ChevronDown className="h-4 w-4 text-muted-foreground" />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align="start" className="w-56">
          <DropdownMenuLabel>Active Connections</DropdownMenuLabel>
          <DropdownMenuSeparator />
          {connections.map((conn) => (
            <DropdownMenuItem
              key={conn.id}
              onClick={() => onConnectionChange(conn.id)}
              className="gap-2"
            >
              <span className="h-2 w-2 rounded-full bg-primary" />
              {conn.name}
            </DropdownMenuItem>
          ))}
        </DropdownMenuContent>
      </DropdownMenu>
    </>
  );
}
