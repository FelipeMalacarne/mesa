import type {
  Connection,
  ListDatabasesResponse,
  ListTablesResponse,
} from "@/api";

export type FetchStatus = "idle" | "loading" | "ok" | "error";
export type ConnectionStatus = Connection.status | "unknown";
export type FetchResult<T> = {
  status: FetchStatus;
  data?: T;
  errorMessage?: string;
};

export type DatabaseEntry = NonNullable<
  ListDatabasesResponse["databases"]
>[number];
export type TableEntry = NonNullable<ListTablesResponse["tables"]>[number];
export type TablesByDatabaseKey = Map<string, FetchResult<ListTablesResponse>>;

export type QueryState<T> = {
  isLoading: boolean;
  isError: boolean;
  error: unknown;
  data?: T;
};
