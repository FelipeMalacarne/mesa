import {
  ChevronRight,
  Database,
  HardDrive,
  Plus,
  Table as TableIcon,
} from "lucide-react";

import {
  Collapsible,
  CollapsibleContent,
  CollapsibleTrigger,
} from "@/components/ui/collapsible";
import {
  SidebarGroup,
  SidebarGroupLabel,
  SidebarMenu,
  SidebarMenuAction,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSub,
  SidebarMenuSubButton,
  SidebarMenuSubItem,
} from "@/components/ui/sidebar";
import { ResponsiveDialog } from "./responsive-dialog";
import { useCallback, useMemo, useState } from "react";
import type { ReactNode } from "react";
import { ConnectionForm } from "./connection-form";
import { Button } from "./ui/button";
import { useQueries } from "@tanstack/react-query";
import { Link, useNavigate } from "@tanstack/react-router";
import {
  ApiError,
  Connection,
  ConnectionsService,
  ListDatabasesResponse,
  ListTablesResponse,
} from "@/api";

type FetchStatus = "idle" | "loading" | "ok" | "error";
type ConnectionStatus = Connection.status | "unknown";
type FetchResult<T> = {
  status: FetchStatus;
  data?: T;
  errorMessage?: string;
};

type DatabaseEntry = NonNullable<ListDatabasesResponse["databases"]>[number];
type TableEntry = NonNullable<ListTablesResponse["tables"]>[number];
type TablesByDatabaseKey = Map<string, FetchResult<ListTablesResponse>>;
type QueryState<T> = {
  isLoading: boolean;
  isError: boolean;
  error: unknown;
  data?: T;
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

const mapQueryToFetchResult = <T extends { status?: string; error?: string }>(
  query: QueryState<T> | undefined,
  errorStatusValue: string,
): FetchResult<T> => {
  if (!query) {
    return { status: "idle" };
  }

  if (query.isLoading) {
    return { status: "loading" };
  }

  if (query.isError) {
    return { status: "error", errorMessage: getErrorMessage(query.error) };
  }

  if (query.data?.status === errorStatusValue) {
    return {
      status: "error",
      data: query.data,
      errorMessage: query.data.error,
    };
  }

  if (query.data) {
    return { status: "ok", data: query.data };
  }

  return { status: "idle" };
};

const useExpandedConnections = (
  connections: Connection[],
  openConnections: Record<string, boolean>,
) => {
  const connectionsByID = useMemo(
    () => new Map(connections.map((connection) => [connection.id, connection])),
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
        return connection?.status === Connection.status.OK;
      }),
    [connectionsByID, expandedConnectionIds],
  );

  return { connectionsByID, expandedConnectionIds, activeConnectionIds };
};

const useExpandedDatabaseKeys = (
  openDatabases: Record<string, boolean>,
  connectionsByID: Map<string, Connection>,
  openConnections: Record<string, boolean>,
) =>
  useMemo(
    () =>
      Object.entries(openDatabases)
        .filter(([, isOpen]) => isOpen)
        .map(([key]) => {
          const [connectionId] = parseDatabaseKey(key);
          const connection = connectionsByID.get(connectionId);
          return openConnections[connectionId] &&
            connection?.status === Connection.status.OK
            ? key
            : null;
        })
        .filter((key): key is string => key !== null),
    [connectionsByID, openDatabases, openConnections],
  );

const useDatabasesByConnection = (
  activeConnectionIds: string[],
): Map<string, FetchResult<ListDatabasesResponse>> => {
  const databaseQueries = useQueries({
    queries: activeConnectionIds.map((connectionId) => ({
      queryKey: ["connection-databases", connectionId],
      queryFn: () =>
        ConnectionsService.listDatabases({ connectionId: connectionId }),
      staleTime: 60_000,
    })),
  });

  return useMemo(() => {
    const map = new Map<string, FetchResult<ListDatabasesResponse>>();

    activeConnectionIds.forEach((connectionId, index) => {
      map.set(
        connectionId,
        mapQueryToFetchResult(
          databaseQueries[index],
          ListDatabasesResponse.status.ERROR,
        ),
      );
    });

    return map;
  }, [activeConnectionIds, databaseQueries]);
};

const useTablesByDatabase = (
  expandedDatabaseKeys: string[],
): TablesByDatabaseKey => {
  const tableQueries = useQueries({
    queries: expandedDatabaseKeys.map((key) => {
      const [connectionId, databaseName] = parseDatabaseKey(key);
      return {
        queryKey: ["connection-tables", connectionId, databaseName],
        queryFn: () =>
          ConnectionsService.listTables({
            connectionId,
            databaseName,
          }),
        staleTime: 60_000,
      };
    }),
  });

  return useMemo(() => {
    const map = new Map<string, FetchResult<ListTablesResponse>>();

    expandedDatabaseKeys.forEach((key, index) => {
      map.set(
        key,
        mapQueryToFetchResult(
          tableQueries[index],
          ListTablesResponse.status.ERROR,
        ),
      );
    });

    return map;
  }, [expandedDatabaseKeys, tableQueries]);
};

export function NavConnections({
  connections,
  isLoading,
}: {
  connections: Connection[];
  isLoading?: boolean;
}) {
  const navigate = useNavigate();
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
    [setOpenConnections, setOpenDatabases],
  );

  const handleDatabaseToggle = useCallback(
    (key: string, nextOpen: boolean) => {
      setOpenDatabases((prev) => ({
        ...prev,
        [key]: nextOpen,
      }));
    },
    [setOpenDatabases],
  );

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
              navigate={navigate}
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

type ConnectionMenuItemProps = {
  connection: Connection;
  isOpen: boolean;
  onToggle: (nextOpen: boolean) => void;
  navigate: ReturnType<typeof useNavigate>;
  databaseState?: FetchResult<ListDatabasesResponse>;
  openDatabases: Record<string, boolean>;
  onDatabaseToggle: (key: string, nextOpen: boolean) => void;
  tablesByDatabaseKey: TablesByDatabaseKey;
};

const ConnectionMenuItem = ({
  connection,
  isOpen,
  onToggle,
  navigate,
  databaseState,
  openDatabases,
  onDatabaseToggle,
  tablesByDatabaseKey,
}: ConnectionMenuItemProps) => {
  const connectionStatus: ConnectionStatus = connection.status ?? "unknown";
  const statusDot =
    connectionStatus === Connection.status.OK
      ? "bg-emerald-500"
      : connectionStatus === Connection.status.ERROR
        ? "bg-rose-500"
        : "bg-muted-foreground/40";

  return (
    <Collapsible
      asChild
      open={isOpen}
      onOpenChange={onToggle}
      className="group/connection-collapsible"
    >
      <SidebarMenuItem>
        <SidebarMenuButton asChild tooltip={connection.name} isActive={isOpen}>
          <Link
            to="/connections/$connectionId"
            params={{ connectionId: connection.id }}
          >
            <HardDrive />
            <span>{connection.name}</span>
            <span className="ml-auto flex items-center">
              <span className={`h-2 w-2 rounded-full ${statusDot}`} />
            </span>
          </Link>
        </SidebarMenuButton>
        <CollapsibleTrigger asChild>
          <SidebarMenuAction
            type="button"
            aria-label={`Toggle ${connection.name}`}
          >
            <ChevronRight className="transition-transform duration-200 group-data-[state=open]/connection-collapsible:rotate-90" />
          </SidebarMenuAction>
        </CollapsibleTrigger>
        <CollapsibleContent>
          <SidebarMenuSub>
            {connectionStatus === Connection.status.ERROR ? (
              <DisabledSubButton>
                {connection.status_error ??
                  "Unable to connect to this database"}
              </DisabledSubButton>
            ) : null}
            {connectionStatus === "unknown" ? (
              <DisabledSubButton>Status unavailable</DisabledSubButton>
            ) : null}
            {connectionStatus === Connection.status.OK ? (
              <ConnectionDatabases
                connection={connection}
                databaseState={databaseState}
                openDatabases={openDatabases}
                onDatabaseToggle={onDatabaseToggle}
                tablesByDatabaseKey={tablesByDatabaseKey}
                navigate={navigate}
              />
            ) : null}
          </SidebarMenuSub>
        </CollapsibleContent>
      </SidebarMenuItem>
    </Collapsible>
  );
};

type ConnectionDatabasesProps = {
  connection: Connection;
  databaseState?: FetchResult<ListDatabasesResponse>;
  openDatabases: Record<string, boolean>;
  onDatabaseToggle: (key: string, nextOpen: boolean) => void;
  tablesByDatabaseKey: TablesByDatabaseKey;
  navigate: ReturnType<typeof useNavigate>;
};

const ConnectionDatabases = ({
  connection,
  databaseState,
  openDatabases,
  onDatabaseToggle,
  tablesByDatabaseKey,
  navigate,
}: ConnectionDatabasesProps) => {
  if (!databaseState) {
    return null;
  }

  const { status, data, errorMessage } = databaseState;
  const databases = data?.databases ?? [];

  if (status === "loading") {
    return <DisabledSubButton>Loading databases...</DisabledSubButton>;
  }

  if (status === "error") {
    return (
      <DisabledSubButton>
        {errorMessage ?? "Unable to load databases"}
      </DisabledSubButton>
    );
  }

  if (status === "ok" && databases.length === 0) {
    return <DisabledSubButton>No databases found</DisabledSubButton>;
  }

  if (status !== "ok") {
    return null;
  }

  return (
    <>
      {databases.map((database) => {
        const key = databaseKey(connection.id, database.name);
        return (
          <DatabaseMenuItem
            key={database.name}
            connectionId={connection.id}
            database={database}
            isOpen={openDatabases[key] ?? false}
            onToggle={(nextOpen) => onDatabaseToggle(key, nextOpen)}
            navigate={navigate}
            tableState={tablesByDatabaseKey.get(key)}
          />
        );
      })}
    </>
  );
};

type DatabaseMenuItemProps = {
  connectionId: string;
  database: DatabaseEntry;
  isOpen: boolean;
  onToggle: (nextOpen: boolean) => void;
  navigate: ReturnType<typeof useNavigate>;
  tableState?: FetchResult<ListTablesResponse>;
};

const DatabaseMenuItem = ({
  connectionId,
  database,
  isOpen,
  onToggle,
  navigate,
  tableState,
}: DatabaseMenuItemProps) => {
  const tableStatus = tableState?.status ?? "idle";
  const tables = tableState?.data?.tables ?? [];

  const navigateToDatabase = () =>
    navigate({
      to: "/connections/$connectionId/databases/$databaseName",
      params: { connectionId, databaseName: database.name },
    });

  return (
    <Collapsible
      asChild
      open={isOpen}
      onOpenChange={onToggle}
      className="group/database-collapsible"
    >
      <SidebarMenuSubItem>
        <SidebarMenuSubButton
          asChild
          className="pr-8 [&>svg]:text-sidebar-foreground"
        >
          <Link to="/connections/$connectionId/databases/$databaseName" params={{ connectionId, databaseName: database.name }}>
            <Database className="text-sidebar-foreground" />
            <span>{database.name}</span>
          </Link>
        </SidebarMenuSubButton>
        <CollapsibleTrigger asChild>
          <button
            type="button"
            aria-label={`Toggle ${database.name}`}
            className="text-sidebar-foreground ring-sidebar-ring hover:bg-sidebar-accent hover:text-sidebar-accent-foreground absolute top-1 right-1 flex size-5 items-center justify-center rounded-md p-0 outline-hidden transition-transform focus-visible:ring-2 group-data-[collapsible=icon]:hidden"
          >
            <ChevronRight className="transition-transform duration-200 group-data-[state=open]/database-collapsible:rotate-90" />
          </button>
        </CollapsibleTrigger>
        <CollapsibleContent>
          <SidebarMenuSub className="ml-4">
            {tableStatus === "loading" ? (
              <DisabledSubButton size="sm">Loading tables...</DisabledSubButton>
            ) : null}
            {tableStatus === "error" ? (
              <DisabledSubButton size="sm">
                {tableState?.errorMessage ?? "Unable to load tables"}
              </DisabledSubButton>
            ) : null}
            {tableStatus === "ok" && tables.length === 0 ? (
              <DisabledSubButton size="sm">No tables found</DisabledSubButton>
            ) : null}
            {tableStatus === "ok"
              ? tables.map((table) => (
                  <TableMenuItem
                    key={table.name}
                    connectionId={connectionId}
                    databaseName={database.name}
                    table={table}
                    navigate={navigate}
                  />
                ))
              : null}
          </SidebarMenuSub>
        </CollapsibleContent>
      </SidebarMenuSubItem>
    </Collapsible>
  );
};

type TableMenuItemProps = {
  connectionId: string;
  databaseName: string;
  table: TableEntry;
  navigate: ReturnType<typeof useNavigate>;
};

const TableMenuItem = ({
  connectionId,
  databaseName,
  table,
  navigate,
}: TableMenuItemProps) => {
  const navigateToTable = () =>
    navigate({
      to: "/connections/$connectionId/databases/$databaseName/tables/$tableName",
      params: {
        connectionId,
        databaseName,
        tableName: table.name,
      },
    });

  return (
    <SidebarMenuSubItem>
      <SidebarMenuSubButton
        asChild
        size="sm"
        className="[&>svg]:text-sidebar-foreground"
      >
        <button type="button" onClick={navigateToTable}>
          <TableIcon className="text-sidebar-foreground" />
          <span>{table.name}</span>
        </button>
      </SidebarMenuSubButton>
    </SidebarMenuSubItem>
  );
};

const DisabledSubButton = ({
  children,
  size,
}: {
  children: ReactNode;
  size?: "sm" | "md";
}) => (
  <SidebarMenuSubItem>
    <SidebarMenuSubButton size={size} aria-disabled tabIndex={-1}>
      {children}
    </SidebarMenuSubButton>
  </SidebarMenuSubItem>
);
