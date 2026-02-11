import type { CancelablePromise } from "@/api";
import { OpenAPI } from "@/api";
import { request as __request } from "@/api/core/request";

export type Database = {
  name: string;
};

export type Table = {
  name: string;
  type: string;
  row_count: number;
  size: number;
};

export type ListDatabasesResponse = {
  status: "ok" | "error";
  error?: string;
  databases: Database[];
};

export type ListTablesResponse = {
  status: "ok" | "error";
  error?: string;
  tables: Table[];
};

export const listDatabases = (
  connectionId: string,
): CancelablePromise<ListDatabasesResponse> =>
  __request(OpenAPI, {
    method: "GET",
    url: "/connections/{connectionId}/databases",
    path: {
      connectionId,
    },
  });

export const listTables = (
  connectionId: string,
  databaseName: string,
): CancelablePromise<ListTablesResponse> =>
  __request(OpenAPI, {
    method: "GET",
    url: "/connections/{connectionId}/databases/{databaseName}/tables",
    path: {
      connectionId,
      databaseName,
    },
  });
