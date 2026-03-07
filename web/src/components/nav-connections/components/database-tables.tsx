
import { useQuery } from "@tanstack/react-query";

import { listTables, getListTablesQueryKey } from "@/api/connections/connections";
import { TableMenuItem } from "./table-menu-item";
import { DisabledSubButton } from "./disabled-sub-button";
import { Spinner } from "../../ui/spinner";

export type DatabaseTablesProps = {
  connectionId: string;
  databaseName: string;
  activeState: {
    connectionId?: string;
    databaseName?: string;
    tableName?: string;
  };
};

export const DatabaseTables = ({
  connectionId,
  databaseName,
  activeState,
}: DatabaseTablesProps) => {
  const {
    data: tables,
    isLoading,
    isError,
    error,
  } = useQuery({
    queryKey: getListTablesQueryKey(connectionId, databaseName),
    queryFn: ({ signal }) => listTables(connectionId, databaseName, { signal }),
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
      <DisabledSubButton size="sm">
        {error instanceof Error ? error.message : "Unable to load tables"}
      </DisabledSubButton>
    );
  }

  if (!tables || tables.length === 0) {
    return <DisabledSubButton size="sm">No tables found</DisabledSubButton>;
  }

  return (
    <>
      {tables.map((table) => (
        <TableMenuItem
          key={table.name}
          connectionId={connectionId}
          databaseName={databaseName}
          table={table}
          activeState={activeState}
        />
      ))}
    </>
  );
};
