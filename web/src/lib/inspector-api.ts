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

export const listDatabases = (
  connectionId: string,
): CancelablePromise<Database[]> =>
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
): CancelablePromise<Table[]> =>
  __request(OpenAPI, {
    method: "GET",
    url: "/connections/{connectionId}/databases/{databaseName}/tables",
    path: {
      connectionId,
      databaseName,
    },
  });
