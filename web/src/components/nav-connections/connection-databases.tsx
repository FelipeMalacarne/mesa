import type {
  Connection,
  ListDatabasesResponse,
} from "@/api";

import { databaseKey } from "./hooks";
import type { FetchResult, TablesByDatabaseKey } from "./types";
import { DatabaseMenuItem } from "./database-menu-item";
import { DisabledSubButton } from "./disabled-sub-button";

export type ConnectionDatabasesProps = {
  connection: Connection;
  databaseState?: FetchResult<ListDatabasesResponse>;
  openDatabases: Record<string, boolean>;
  onDatabaseToggle: (key: string, nextOpen: boolean) => void;
  tablesByDatabaseKey: TablesByDatabaseKey;
};

export const ConnectionDatabases = ({
  connection,
  databaseState,
  openDatabases,
  onDatabaseToggle,
  tablesByDatabaseKey,
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
            tableState={tablesByDatabaseKey.get(key)}
          />
        );
      })}
    </>
  );
};
