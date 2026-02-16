import { useEffect, useMemo } from "react";
import { useQueries } from "@tanstack/react-query";
import { useShallow } from "zustand/react/shallow";

import {
  Connection,
  ConnectionsService,
  ListDatabasesResponse,
  ListTablesResponse,
} from "@/api";

import type {
  ConnectionTreeNode,
  DatabaseTreeNode,
  FetchResult,
} from "./types";
import { useConnectionStore } from "@/stores/connection-store";
import { useNavConnectionsState } from "./context";
import { databaseKey, parseDatabaseKey } from "./utils";

export const useExpandedConnections = (
  connections: Connection[],
  openConnections: Record<string, boolean>
) => {
  const connectionStatuses = useConnectionStore(
    (state) => state.connectionStatuses
  );

  const connectionsByID = useMemo(
    () => new Map(connections.map((connection) => [connection.id, connection])),
    [connections]
  );

  const expandedConnectionIds = useMemo(
    () =>
      Object.entries(openConnections)
        .filter(([, isOpen]) => isOpen)
        .map(([id]) => id),
    [openConnections]
  );

  const activeConnectionIds = useMemo(
    () =>
      expandedConnectionIds.filter((id) => {
        return connectionsByID.has(id) && connectionStatuses[id] === "ok";
      }),
    [connectionsByID, expandedConnectionIds, connectionStatuses]
  );

  return { connectionsByID, activeConnectionIds };
};

export const useExpandedDatabaseKeys = (
  openDatabases: Record<string, boolean>,
  connectionsByID: Map<string, Connection>,
  openConnections: Record<string, boolean>
) => {
  const connectionStatuses = useConnectionStore(
    (state) => state.connectionStatuses
  );
  return useMemo(
    () =>
      Object.entries(openDatabases)
        .filter(([, isOpen]) => isOpen)
        .map(([key]) => {
          const [connectionId] = parseDatabaseKey(key);
          return (
            openConnections[connectionId] &&
            connectionStatuses[connectionId] === "ok"
              ? key
              : null
          );
        })
        .filter((key): key is string => key !== null),
    [connectionsByID, openDatabases, openConnections, connectionStatuses]
  );
};

export const useSyncDatabases = (activeConnectionIds: string[]) => {
  const setDatabases = useConnectionStore((state) => state.setDatabases);
  const setDatabaseFetchStatus = useConnectionStore(
    (state) => state.setDatabaseFetchStatus
  );

  const databaseQueries = useQueries({
    queries: activeConnectionIds.map((connectionId) => ({
      queryKey: ["connection-databases", connectionId],
      queryFn: () =>
        ConnectionsService.listDatabases({ connectionId: connectionId }),
      staleTime: 60_000,
    })),
  });

  useEffect(() => {
    activeConnectionIds.forEach((connectionId, index) => {
      const query = databaseQueries[index];
      if (!query) return;

      const currentStatus =
        useConnectionStore.getState().databaseFetchStatuses[connectionId];
      // We check reference equality of databases array.
      const currentData = useConnectionStore.getState().databases[connectionId];

      if (query.isLoading) {
        if (currentStatus !== "loading") {
          setDatabaseFetchStatus(connectionId, "loading");
        }
      } else if (query.isError) {
        if (currentStatus !== "error") {
          setDatabaseFetchStatus(connectionId, "error");
        }
      } else if (query.data) {
        if (query.data.status === ListDatabasesResponse.status.ERROR) {
          if (currentStatus !== "error") {
            setDatabaseFetchStatus(connectionId, "error");
          }
        } else {
          // Only update if status changed or data reference changed
          // We also check if the data content is actually different to avoid loops with new references
          const newData = query.data.databases || [];
          if (
            currentStatus !== "ok" ||
            (currentData !== newData &&
              JSON.stringify(currentData) !== JSON.stringify(newData))
          ) {
            setDatabases(connectionId, newData, "ok");
          }
        }
      }
    });
  }, [
    // We intentionally omit databaseQueries to prevent loop if it's unstable.
    // However, we need the effect to run when queries update.
    // The issue is that useQueries returns a new array every render.
    // We can depend on specific properties that change, or use JSON.stringify on the states.
    // But for now, let's just rely on the deep equality check inside the effect to prevent
    // dispatching if nothing changed.
    databaseQueries,
    activeConnectionIds,
    setDatabases,
    setDatabaseFetchStatus,
  ]);
};

export const useSyncTables = (expandedDatabaseKeys: string[]) => {
  const setTables = useConnectionStore((state) => state.setTables);
  const setTableFetchStatus = useConnectionStore(
    (state) => state.setTableFetchStatus
  );

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

  useEffect(() => {
    expandedDatabaseKeys.forEach((key, index) => {
      const query = tableQueries[index];
      if (!query) return;

      const currentStatus =
        useConnectionStore.getState().tableFetchStatuses[key];
      const currentData = useConnectionStore.getState().tables[key];

      if (query.isLoading) {
        if (currentStatus !== "loading") {
          setTableFetchStatus(key, "loading");
        }
      } else if (query.isError) {
        if (currentStatus !== "error") {
          setTableFetchStatus(key, "error");
        }
      } else if (query.data) {
        if (query.data.status === ListTablesResponse.status.ERROR) {
          if (currentStatus !== "error") {
            setTableFetchStatus(key, "error");
          }
        } else {
          const newData = query.data.tables || [];
          if (
            currentStatus !== "ok" ||
            (currentData !== newData &&
              JSON.stringify(currentData) !== JSON.stringify(newData))
          ) {
            setTables(key, newData, "ok");
          }
        }
      }
    });
  }, [expandedDatabaseKeys, tableQueries, setTables, setTableFetchStatus]);
};

export const useConnectionTreeData = (connections: Connection[]) => {
  const { openConnections, openDatabases } = useNavConnectionsState();
  const setConnections = useConnectionStore((state) => state.setConnections);

  // Sync connections to store
  useEffect(() => {
    setConnections(connections);
  }, [connections, setConnections]);

  const {
    connectionStatuses,
    databases,
    databaseFetchStatuses,
    tables,
    tableFetchStatuses,
  } = useConnectionStore(
    useShallow((state) => ({
      connectionStatuses: state.connectionStatuses,
      databases: state.databases,
      databaseFetchStatuses: state.databaseFetchStatuses,
      tables: state.tables,
      tableFetchStatuses: state.tableFetchStatuses,
    }))
  );

  const { connectionsByID, activeConnectionIds } = useExpandedConnections(
    connections,
    openConnections
  );
  const expandedDatabaseKeys = useExpandedDatabaseKeys(
    openDatabases,
    connectionsByID,
    openConnections
  );

  // Sync data
  useSyncDatabases(activeConnectionIds);
  useSyncTables(expandedDatabaseKeys);

  const connectionNodes = useMemo(() => {
    return connections.map<ConnectionTreeNode>((connection) => {
      const connectionStatus = connectionStatuses[connection.id] ?? "unknown";

      const dbStatus = databaseFetchStatuses[connection.id];
      const dbList = databases[connection.id];

      let databaseState: FetchResult<ListDatabasesResponse> = {
        status: "idle",
      };
      if (dbStatus) {
        databaseState = {
          status: dbStatus,
          data: dbList
            ? { status: ListDatabasesResponse.status.OK, databases: dbList }
            : undefined,
        };
      }

      const dbs: DatabaseTreeNode[] =
        connectionStatus === "ok" && dbStatus === "ok"
          ? (dbList ?? []).map((database) => {
              const key = databaseKey(connection.id, database.name);

              const tblStatus = tableFetchStatuses[key];
              const tblList = tables[key];

              let tableState: FetchResult<ListTablesResponse> = {
                status: "idle",
              };
              if (tblStatus) {
                tableState = {
                  status: tblStatus,
                  data: tblList
                    ? {
                        status: ListTablesResponse.status.OK,
                        tables: tblList,
                      }
                    : undefined,
                };
              }

              return {
                key,
                connectionId: connection.id,
                database,
                isOpen: openDatabases[key] ?? false,
                tableState,
              };
            })
          : [];

      return {
        connection,
        isOpen: openConnections[connection.id] ?? false,
        status: connectionStatus,
        databaseState,
        databases: dbs,
      };
    });
  }, [
    connections,
    connectionStatuses,
    databases,
    databaseFetchStatuses,
    tables,
    tableFetchStatuses,
    openConnections,
    openDatabases,
  ]);

  return { connectionNodes };
};
