import type { ListDatabasesResponse } from "@/api";

import type {
  DatabaseTreeNode,
  FetchResult,
} from "../state/types";
import { DatabaseMenuItem } from "./database-menu-item";
import { DisabledSubButton } from "./disabled-sub-button";
import { Spinner } from "../../ui/spinner";

export type ConnectionDatabasesProps = {
  connectionId: string;
  databaseState?: FetchResult<ListDatabasesResponse>;
  databases: DatabaseTreeNode[];
};

export const ConnectionDatabases = ({
  connectionId,
  databaseState,
  databases,
}: ConnectionDatabasesProps) => {
  if (!databaseState) {
    return null;
  }

  const { status, data, errorMessage } = databaseState;
  const rawLength = data?.databases?.length ?? 0;

  if (status === "loading") {
    return (
      <DisabledSubButton>
        <Spinner />
      </DisabledSubButton>
    );
  }

  if (status === "error") {
    return (
      <DisabledSubButton>
        {errorMessage ?? "Unable to load databases"}
      </DisabledSubButton>
    );
  }

  if (status === "ok" && rawLength === 0) {
    return <DisabledSubButton>No databases found</DisabledSubButton>;
  }

  if (status !== "ok") {
    return null;
  }

  return (
    <>
      {databases.map((database) => (
        <DatabaseMenuItem
          key={database.key}
          connectionId={connectionId}
          node={database}
        />
      ))}
    </>
  );
};
