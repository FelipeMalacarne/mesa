import { createFileRoute } from "@tanstack/react-router";

export const Route = createFileRoute(
  "/connections/$connectionId/databases/$databaseName/tables/$tableName",
)({
  component: TableDetail,
});

function TableDetail() {
  const { connectionId, databaseName, tableName } = Route.useParams();

  return (
    <div className="p-6">
      <h1 className="text-2xl font-semibold">Table</h1>
      <p className="text-muted-foreground">
        {tableName} ({databaseName} / {connectionId})
      </p>
    </div>
  );
}
