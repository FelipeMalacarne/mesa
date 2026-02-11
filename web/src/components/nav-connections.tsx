import { ChevronRight, Database, HardDrive, Plus, Table as TableIcon } from "lucide-react";

import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import {
  SidebarGroup,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSub,
  SidebarMenuSubButton,
  SidebarMenuSubItem,
} from "@/components/ui/sidebar";
import { ResponsiveDialog } from "./responsive-dialog";
import { useMemo, useState } from "react";
import { ConnectionForm } from "./connection-form";
import { Button } from "./ui/button";
import { useQueries } from "@tanstack/react-query";
import { ApiError, type Connection } from "@/api";
import {
  listDatabases,
  listTables,
  type ListDatabasesResponse,
  type ListTablesResponse,
} from "@/lib/inspector-api";

type FetchStatus = "idle" | "loading" | "ok" | "error";
type ConnectionStatus = "ok" | "error" | "unknown";
type ConnectionWithStatus = Connection & {
  status?: "ok" | "error";
  status_error?: string;
};

const databaseKey = (connectionId: string, databaseName: string) =>
  JSON.stringify([connectionId, databaseName]);

const parseDatabaseKey = (key: string): [string, string] => {
  const parsed = JSON.parse(key) as [string, string];
  return [parsed[0], parsed[1]];
};

const getErrorMessage = (error: unknown) => {
  if (error instanceof ApiError) {
    return (
      error.body?.message ??
      error.statusText ??
      `Request failed (${error.status})`
    );
  }

  return "Unable to reach the database.";
};

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

  const handleConnectionToggle = (connectionId: string, nextOpen: boolean) => {
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
  };

  const connectionsByID = useMemo(
    () =>
      new Map(
        (connections as ConnectionWithStatus[]).map((connection) => [
          connection.id,
          connection,
        ]),
      ),
    [connections],
  );

  const expandedConnectionIds = useMemo(
    () =>
      Object.entries(openConnections)
        .filter(([, isOpen]) => isOpen)
        .map(([id]) => id),
    [openConnections],
  );

  const activeConnectionIds = useMemo(
    () =>
      expandedConnectionIds.filter((id) => {
        const connection = connectionsByID.get(id);
        return connection?.status === "ok";
      }),
    [connectionsByID, expandedConnectionIds],
  );

  const databaseQueries = useQueries({
    queries: activeConnectionIds.map((connectionId) => ({
      queryKey: ["connection-databases", connectionId],
      queryFn: () => listDatabases(connectionId),
      staleTime: 60_000,
    })),
  });

  const databasesByConnection = useMemo(() => {
    const map = new Map<
      string,
      {
        status: FetchStatus;
        data?: ListDatabasesResponse;
        errorMessage?: string;
      }
    >();

    activeConnectionIds.forEach((connectionId, index) => {
      const query = databaseQueries[index];
      if (query.isLoading) {
        map.set(connectionId, { status: "loading" });
        return;
      }
      if (query.isError) {
        map.set(connectionId, {
          status: "error",
          errorMessage: getErrorMessage(query.error),
        });
        return;
      }
      if (query.data?.status === "error") {
        map.set(connectionId, {
          status: "error",
          data: query.data,
          errorMessage: query.data.error,
        });
        return;
      }
      if (query.data) {
        map.set(connectionId, { status: "ok", data: query.data });
        return;
      }
      map.set(connectionId, { status: "idle" });
    });

    return map;
  }, [activeConnectionIds, databaseQueries]);

  const expandedDatabaseKeys = useMemo(
    () =>
      Object.entries(openDatabases)
        .filter(([, isOpen]) => isOpen)
        .map(([key]) => {
          const [connectionId] = parseDatabaseKey(key);
          const connection = connectionsByID.get(connectionId);
          return openConnections[connectionId] && connection?.status === "ok"
            ? key
            : null;
        })
        .filter((key): key is string => key !== null),
    [connectionsByID, openDatabases, openConnections],
  );

  const tableQueries = useQueries({
    queries: expandedDatabaseKeys.map((key) => {
      const [connectionId, databaseName] = parseDatabaseKey(key);
      return {
        queryKey: ["connection-tables", connectionId, databaseName],
        queryFn: () => listTables(connectionId, databaseName),
        staleTime: 60_000,
      };
    }),
  });

  const tablesByDatabaseKey = useMemo(() => {
    const map = new Map<
      string,
      {
        status: FetchStatus;
        data?: ListTablesResponse;
        errorMessage?: string;
      }
    >();

    expandedDatabaseKeys.forEach((key, index) => {
      const query = tableQueries[index];
      if (query.isLoading) {
        map.set(key, { status: "loading" });
        return;
      }
      if (query.isError) {
        map.set(key, {
          status: "error",
          errorMessage: getErrorMessage(query.error),
        });
        return;
      }
      if (query.data?.status === "error") {
        map.set(key, {
          status: "error",
          data: query.data,
          errorMessage: query.data.error,
        });
        return;
      }
      if (query.data) {
        map.set(key, { status: "ok", data: query.data });
        return;
      }
      map.set(key, { status: "idle" });
    });

    return map;
  }, [expandedDatabaseKeys, tableQueries]);

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
          {/* <AddConnectionDialog /> */}
        </SidebarGroupLabel>
        <SidebarMenu>
          {isLoading ? (
            <SidebarMenuItem>
              <SidebarMenuButton disabled>Loading connections...</SidebarMenuButton>
            </SidebarMenuItem>
          ) : null}
          {(connections as ConnectionWithStatus[]).map((connection) => {
            const isConnectionOpen = openConnections[connection.id] ?? false;
            const databaseState = databasesByConnection.get(connection.id);
            const connectionStatus: ConnectionStatus =
              connection.status ?? "unknown";
            const databases = databaseState?.data?.databases ?? [];

            const statusDot =
              connectionStatus === "ok"
                ? "bg-emerald-500"
                : connectionStatus === "error"
                  ? "bg-rose-500"
                  : "bg-muted-foreground/40";

            return (
              <Collapsible
                key={connection.id}
                asChild
                open={isConnectionOpen}
                onOpenChange={(nextOpen) =>
                  handleConnectionToggle(connection.id, nextOpen)
                }
                className="group/connection-collapsible"
              >
                <SidebarMenuItem>
                  <CollapsibleTrigger asChild>
                    <SidebarMenuButton
                      tooltip={connection.name}
                      isActive={isConnectionOpen}
                    >
                      <HardDrive />
                      <span>{connection.name}</span>
                      <span className="ml-auto flex items-center gap-2">
                        <span className={`h-2 w-2 rounded-full ${statusDot}`} />
                        <ChevronRight className="transition-transform duration-200 group-data-[state=open]/connection-collapsible:rotate-90" />
                      </span>
                    </SidebarMenuButton>
                  </CollapsibleTrigger>
                  <CollapsibleContent>
                    <SidebarMenuSub>
                      {connectionStatus === "error" ? (
                        <SidebarMenuSubItem>
                          <SidebarMenuSubButton disabled>
                            {connection.status_error ??
                              "Unable to connect to this database"}
                          </SidebarMenuSubButton>
                        </SidebarMenuSubItem>
                      ) : null}
                      {connectionStatus === "unknown" ? (
                        <SidebarMenuSubItem>
                          <SidebarMenuSubButton disabled>
                            Status unavailable
                          </SidebarMenuSubButton>
                        </SidebarMenuSubItem>
                      ) : null}
                      {connectionStatus === "ok" &&
                      databaseState?.status === "loading" ? (
                        <SidebarMenuSubItem>
                          <SidebarMenuSubButton disabled>
                            Loading databases...
                          </SidebarMenuSubButton>
                        </SidebarMenuSubItem>
                      ) : null}
                      {connectionStatus === "ok" &&
                      databaseState?.status === "error" ? (
                        <SidebarMenuSubItem>
                          <SidebarMenuSubButton disabled>
                            {databaseState?.errorMessage ??
                              "Unable to load databases"}
                          </SidebarMenuSubButton>
                        </SidebarMenuSubItem>
                      ) : null}
                      {connectionStatus === "ok" &&
                      databaseState?.status === "ok" &&
                      databases.length === 0 ? (
                        <SidebarMenuSubItem>
                          <SidebarMenuSubButton disabled>
                            No databases found
                          </SidebarMenuSubButton>
                        </SidebarMenuSubItem>
                      ) : null}
                      {connectionStatus === "ok" &&
                      databaseState?.status === "ok"
                        ? databases.map((database) => {
                            const key = databaseKey(connection.id, database.name);
                            const isDatabaseOpen = openDatabases[key] ?? false;
                            const tableState = tablesByDatabaseKey.get(key);
                            const tableStatus = tableState?.status ?? "idle";
                            const tables = tableState?.data?.tables ?? [];

                            return (
                              <Collapsible
                                key={database.name}
                                asChild
                                open={isDatabaseOpen}
                                onOpenChange={(nextOpen) =>
                                  setOpenDatabases((prev) => ({
                                    ...prev,
                                    [key]: nextOpen,
                                  }))
                                }
                                className="group/database-collapsible"
                              >
                                <SidebarMenuSubItem>
                                  <CollapsibleTrigger asChild>
                                    <SidebarMenuSubButton className="[&>svg]:text-sidebar-foreground">
                                      <Database className="text-sidebar-foreground" />
                                      <span>{database.name}</span>
                                      <ChevronRight className="ml-auto transition-transform duration-200 group-data-[state=open]/database-collapsible:rotate-90" />
                                    </SidebarMenuSubButton>
                                  </CollapsibleTrigger>
                                  <CollapsibleContent>
                                    <SidebarMenuSub className="ml-4">
                                      {tableStatus === "loading" ? (
                                        <SidebarMenuSubItem>
                                          <SidebarMenuSubButton size="sm" disabled>
                                            Loading tables...
                                          </SidebarMenuSubButton>
                                        </SidebarMenuSubItem>
                                      ) : null}
                                      {tableStatus === "error" ? (
                                        <SidebarMenuSubItem>
                                          <SidebarMenuSubButton size="sm" disabled>
                                            {tableState?.errorMessage ??
                                              "Unable to load tables"}
                                          </SidebarMenuSubButton>
                                        </SidebarMenuSubItem>
                                      ) : null}
                                      {tableStatus === "ok" && tables.length === 0 ? (
                                        <SidebarMenuSubItem>
                                          <SidebarMenuSubButton size="sm" disabled>
                                            No tables found
                                          </SidebarMenuSubButton>
                                        </SidebarMenuSubItem>
                                      ) : null}
                                      {tableStatus === "ok"
                                        ? tables.map((table) => (
                                            <SidebarMenuSubItem key={table.name}>
                                              <SidebarMenuSubButton
                                                size="sm"
                                                className="[&>svg]:text-sidebar-foreground"
                                              >
                                                <TableIcon className="text-sidebar-foreground" />
                                                <span>{table.name}</span>
                                              </SidebarMenuSubButton>
                                            </SidebarMenuSubItem>
                                          ))
                                        : null}
                                    </SidebarMenuSub>
                                  </CollapsibleContent>
                                </SidebarMenuSubItem>
                              </Collapsible>
                            );
                          })
                        : null}
                    </SidebarMenuSub>
                  </CollapsibleContent>
                </SidebarMenuItem>
              </Collapsible>
            );
          })}
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
