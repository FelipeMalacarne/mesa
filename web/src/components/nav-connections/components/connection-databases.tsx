import { useQuery } from "@tanstack/react-query";

import { ConnectionsService } from "@/api";
import { DatabaseMenuItem } from "./database-menu-item";
import { DisabledSubButton } from "./disabled-sub-button";
import { Spinner } from "../../ui/spinner";
import { useNavConnectionsState } from "../state/context";
import { databaseKey } from "../state/utils";

export type ConnectionDatabasesProps = {
  connectionId: string;
  activeState: {
    connectionId?: string;
    databaseName?: string;
    tableName?: string;
  };
};

export const ConnectionDatabases = ({
  connectionId,
  activeState,
}: ConnectionDatabasesProps) => {
  const { openDatabases } = useNavConnectionsState();
  const {
    data: databases,
    isLoading,
    isError,
    error,
  } = useQuery({
    queryKey: ["connection-databases", connectionId],
    queryFn: () => ConnectionsService.listDatabases({ connectionId }),
  });

  if (isLoading) {
    return (
      <DisabledSubButton>
        <Spinner />
      </DisabledSubButton>
    );
  }

  if (isError) {
    return (
      <DisabledSubButton>
        {error instanceof Error ? error.message : "Unable to load databases"}
      </DisabledSubButton>
    );
  }

  if (!databases || databases.length === 0) {
    return <DisabledSubButton>No databases found</DisabledSubButton>;
  }

  return (
    <>
      {databases.map((database) => {
        const key = databaseKey(connectionId, database.name);
        return (
          <DatabaseMenuItem
            key={key}
            connectionId={connectionId}
            database={database}
            isOpen={openDatabases[key] ?? false}
            activeState={activeState}
          />
        );
      })}
    </>
  );
};
