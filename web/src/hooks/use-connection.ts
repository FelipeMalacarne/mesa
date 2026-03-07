import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  useFindConnection,
  useGetConnectionOverview,
  useListDatabases,
  useListUsers,
  useListSessions,
  createDatabase,
  createUser,
  createTable,
  killSession,
  getListDatabasesQueryKey,
  getListUsersQueryKey,
  getListSessionsQueryKey,
  getListTablesQueryKey,
} from "@/api/connections/connections";
import type {
  CreateDatabaseRequest,
  CreateUserRequest,
  CreateTableRequest,
} from "@/api/mesaAPI.schemas";

type ConnectionID = string;

export function useConnectionDetails(connectionId: ConnectionID) {
  return useFindConnection(connectionId, {
    query: { staleTime: 1000 * 30, enabled: Boolean(connectionId) },
  });
}

export function useConnectionOverview(connectionId: ConnectionID) {
  return useGetConnectionOverview(connectionId, {
    query: { staleTime: 1000 * 30, enabled: Boolean(connectionId) },
  });
}

export function useConnectionDatabases(connectionId: ConnectionID) {
  return useListDatabases(connectionId, {
    query: { staleTime: 1000 * 60 * 5, enabled: Boolean(connectionId) },
  });
}

export function useConnectionUsers(connectionId: ConnectionID) {
  return useListUsers(connectionId, {
    query: { staleTime: 1000 * 60 * 5, enabled: Boolean(connectionId) },
  });
}

export function useConnectionSessions(connectionId: ConnectionID) {
  return useListSessions(connectionId, {
    query: { refetchInterval: 5000, refetchOnWindowFocus: true },
  });
}

export function useCreateDatabase(connectionId: ConnectionID) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (payload: CreateDatabaseRequest) =>
      createDatabase(connectionId, payload),
    onSuccess: () => {
      client.invalidateQueries({
        queryKey: getListDatabasesQueryKey(connectionId),
      });
    },
  });
}

export function useCreateUser(connectionId: ConnectionID) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (payload: CreateUserRequest) =>
      createUser(connectionId, payload),
    onSuccess: () => {
      client.invalidateQueries({
        queryKey: getListUsersQueryKey(connectionId),
      });
    },
  });
}

export function useCreateTable(
  connectionId: ConnectionID,
  databaseName: string
) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (payload: CreateTableRequest) =>
      createTable(connectionId, databaseName, payload),
    onSuccess: () => {
      client.invalidateQueries({
        queryKey: getListTablesQueryKey(connectionId, databaseName),
      });
    },
  });
}

export function useKillSession(connectionId: ConnectionID) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (pid: number) => killSession(connectionId, pid),
    onSuccess: () => {
      client.invalidateQueries({
        queryKey: getListSessionsQueryKey(connectionId),
      });
    },
  });
}

export type { CreateDatabaseRequest, CreateUserRequest, CreateTableRequest };
