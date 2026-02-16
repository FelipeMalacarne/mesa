import { create } from "zustand";
import { Connection, type Database, ListTablesResponse } from "@/api";

export type ConnectionStatus = "ok" | "error" | "unknown";
export type FetchStatus = "idle" | "loading" | "ok" | "error";

export type Table = NonNullable<ListTablesResponse["tables"]>[number];

export interface ConnectionState {
  connections: Connection[];
  connectionStatuses: Record<string, ConnectionStatus>;

  databases: Record<string, Database[]>;
  databaseFetchStatuses: Record<string, FetchStatus>;

  tables: Record<string, Table[]>;
  tableFetchStatuses: Record<string, FetchStatus>;

  setConnections: (connections: Connection[]) => void;
  setConnectionStatus: (connectionId: string, status: ConnectionStatus) => void;

  setDatabases: (
    connectionId: string,
    databases: Database[],
    status?: FetchStatus,
  ) => void;
  setDatabaseFetchStatus: (connectionId: string, status: FetchStatus) => void;

  setTables: (
    databaseKey: string,
    tables: Table[],
    status?: FetchStatus,
  ) => void;
  setTableFetchStatus: (databaseKey: string, status: FetchStatus) => void;
}

export const useConnectionStore = create<ConnectionState>((set) => ({
  connections: [],
  connectionStatuses: {},

  databases: {},
  databaseFetchStatuses: {},

  tables: {},
  tableFetchStatuses: {},

  setConnections: (connections) => set({ connections }),
  setConnectionStatus: (connectionId, status) =>
    set((state) => ({
      connectionStatuses: {
        ...state.connectionStatuses,
        [connectionId]: status,
      },
    })),

  setDatabases: (connectionId, databases, status = "ok") =>
    set((state) => ({
      databases: {
        ...state.databases,
        [connectionId]: databases,
      },
      databaseFetchStatuses: {
        ...state.databaseFetchStatuses,
        [connectionId]: status,
      },
    })),

  setDatabaseFetchStatus: (connectionId, status) =>
    set((state) => ({
      databaseFetchStatuses: {
        ...state.databaseFetchStatuses,
        [connectionId]: status,
      },
    })),

  setTables: (databaseKey, tables, status = "ok") =>
    set((state) => ({
      tables: {
        ...state.tables,
        [databaseKey]: tables,
      },
      tableFetchStatuses: {
        ...state.tableFetchStatuses,
        [databaseKey]: status,
      },
    })),

  setTableFetchStatus: (databaseKey, status) =>
    set((state) => ({
      tableFetchStatuses: {
        ...state.tableFetchStatuses,
        [databaseKey]: status,
      },
    })),
}));
