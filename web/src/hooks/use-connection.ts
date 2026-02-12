import {
	ConnectionsService,
	type CreateUserRequest,
	type DBUser,
	type ListDatabasesResponse,
	type OverviewResponse,
	type Session,
} from "@/api";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

type ConnectionID = string;

const queryKey = (id: ConnectionID, resource: string) => ["connection", id, resource];

export function useConnectionDetails(connectionId: ConnectionID) {
  return useQuery({
    queryKey: queryKey(connectionId, "details"),
    enabled: Boolean(connectionId),
    queryFn: () => ConnectionsService.findConnection({ connectionId }),
    staleTime: 1000 * 30,
  });
}

export function useConnectionOverview(connectionId: ConnectionID) {
  return useQuery<OverviewResponse>({
    queryKey: queryKey(connectionId, "overview"),
    enabled: Boolean(connectionId),
    queryFn: () =>
      ConnectionsService.getConnectionOverview({ connectionId }),
    staleTime: 1000 * 30,
  });
}

export function useConnectionDatabases(connectionId: ConnectionID) {
  return useQuery<ListDatabasesResponse>({
    queryKey: queryKey(connectionId, "databases"),
    enabled: Boolean(connectionId),
    queryFn: () => ConnectionsService.listDatabases({ connectionId }),
    staleTime: 1000 * 60 * 5,
  });
}

export function useConnectionUsers(connectionId: ConnectionID) {
  return useQuery<DBUser[]>({
    queryKey: queryKey(connectionId, "users"),
    enabled: Boolean(connectionId),
    queryFn: () => ConnectionsService.listUsers({ connectionId }),
    staleTime: 1000 * 60 * 5,
  });
}

export function useConnectionSessions(connectionId: ConnectionID) {
  return useQuery<Session[]>({
    queryKey: queryKey(connectionId, "sessions"),
    enabled: Boolean(connectionId),
    queryFn: () => ConnectionsService.listSessions({ connectionId }),
    refetchInterval: 5000,
    refetchOnWindowFocus: true,
  });
}

export function useCreateDatabase(connectionId: ConnectionID) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (name: string) =>
      ConnectionsService.createDatabase({
        connectionId,
        requestBody: { name },
      }),
    onSuccess: () => {
      client.invalidateQueries({ queryKey: queryKey(connectionId, "databases") });
    },
  });
}

export function useCreateUser(connectionId: ConnectionID) {
	const client = useQueryClient();
	return useMutation({
		mutationFn: (payload: CreateUserRequest) =>
			ConnectionsService.createUser({
				connectionId,
				requestBody: payload,
			}),
		onSuccess: () => {
			client.invalidateQueries({ queryKey: queryKey(connectionId, "users") });
		},
	});
}

export function useKillSession(connectionId: ConnectionID) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (pid: number) =>
      ConnectionsService.killSession({ connectionId, pid }),
    onSuccess: () => {
      client.invalidateQueries({ queryKey: queryKey(connectionId, "sessions") });
    },
  });
}
