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

export type DatabaseTreeNode = {
  key: string;
  connectionId: string;
  database: DatabaseEntry;
  isOpen: boolean;
  tableState?: FetchResult<ListTablesResponse>;
};

export type ConnectionTreeNode = {
  connection: Connection;
  isOpen: boolean;
  status: ConnectionStatus;
  databaseState?: FetchResult<ListDatabasesResponse>;
  databases: DatabaseTreeNode[];
};
