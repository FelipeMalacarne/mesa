import { Plus } from "lucide-react";

import {
  SidebarGroup,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
} from "@/components/ui/sidebar";
import { ResponsiveDialog } from "./responsive-dialog";
import { useCallback, useState } from "react";
import { ConnectionForm } from "./connection-form";
import { Button } from "./ui/button";
import type { Connection } from "@/api";

import {
  parseDatabaseKey,
  useDatabasesByConnection,
  useExpandedConnections,
  useExpandedDatabaseKeys,
  useTablesByDatabase,
} from "./nav-connections/hooks";
import { ConnectionMenuItem } from "./nav-connections/connection-menu-item";

export function NavConnections({
  connections,
  isLoading,
}: {
  connections: Connection[];
  isLoading?: boolean;
}) {
  const [createConnectionOpen, setCreateConnectionOpen] = useState(false);
  const [openConnections, setOpenConnections] = useState<
    Record<string, boolean>
  >({});
  const [openDatabases, setOpenDatabases] = useState<Record<string, boolean>>(
    {},
  );

  const { connectionsByID, activeConnectionIds } = useExpandedConnections(
    connections,
    openConnections,
  );
  const expandedDatabaseKeys = useExpandedDatabaseKeys(
    openDatabases,
    connectionsByID,
    openConnections,
  );
  const databasesByConnection = useDatabasesByConnection(activeConnectionIds);
  const tablesByDatabaseKey = useTablesByDatabase(expandedDatabaseKeys);

  const handleConnectionToggle = useCallback(
    (connectionId: string, nextOpen: boolean) => {
      setOpenConnections((prev) => ({
        ...prev,
        [connectionId]: nextOpen,
      }));

      if (!nextOpen) {
        setOpenDatabases((prev) => {
          const next = { ...prev };
          Object.keys(next).forEach((key) => {
            const [id] = parseDatabaseKey(key);
            if (id === connectionId) {
              delete next[key];
            }
          });
          return next;
        });
      }
    },
    [],
  );

  const handleDatabaseToggle = useCallback((key: string, nextOpen: boolean) => {
    setOpenDatabases((prev) => ({
      ...prev,
      [key]: nextOpen,
    }));
  }, []);

  return (
    <>
      <SidebarGroup>
        <SidebarGroupLabel className="flex items-center justify-between">
          Connections
          <Button
            onClick={() => setCreateConnectionOpen(true)}
            variant="ghost"
            size="icon"
            className="size-6"
          >
            <Plus />
          </Button>
        </SidebarGroupLabel>
        <SidebarMenu>
          {isLoading ? (
            <SidebarMenuItem>
              <SidebarMenuButton disabled>
                Loading connections...
              </SidebarMenuButton>
            </SidebarMenuItem>
          ) : null}
          {connections.map((connection) => (
            <ConnectionMenuItem
              key={connection.id}
              connection={connection}
              isOpen={openConnections[connection.id] ?? false}
              onToggle={(nextOpen) =>
                handleConnectionToggle(connection.id, nextOpen)
              }
              databaseState={databasesByConnection.get(connection.id)}
              openDatabases={openDatabases}
              onDatabaseToggle={handleDatabaseToggle}
              tablesByDatabaseKey={tablesByDatabaseKey}
            />
          ))}
        </SidebarMenu>
      </SidebarGroup>

      <ResponsiveDialog
        title="Add Connection"
        isOpen={createConnectionOpen}
        setIsOpen={setCreateConnectionOpen}
      >
        <ConnectionForm onSuccess={() => setCreateConnectionOpen(false)} />
      </ResponsiveDialog>
    </>
  );
}
