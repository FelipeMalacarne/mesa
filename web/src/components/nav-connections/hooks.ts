import { useMemo } from "react";
import { useQueries } from "@tanstack/react-query";

import {
  ApiError,
  Connection,
  ConnectionsService,
  ListDatabasesResponse,
  ListTablesResponse,
} from "@/api";

import type {
  FetchResult,
  QueryState,
  TablesByDatabaseKey,
} from "./types";

export const databaseKey = (connectionId: string, databaseName: string) =>
  JSON.stringify([connectionId, databaseName]);

export const parseDatabaseKey = (key: string): [string, string] => {
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

export const useExpandedConnections = (
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

  return { connectionsByID, activeConnectionIds };
};

export const useExpandedDatabaseKeys = (
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

export const useDatabasesByConnection = (
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

export const useTablesByDatabase = (
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
