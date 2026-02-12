import { createFileRoute } from "@tanstack/react-router";

export const Route = createFileRoute(
  "/connections/$connectionId/databases/$databaseName/tables/$tableName",
)({
  component: DatabaseTable,
});

function DatabaseTable() {
  const { tableName } = Route.useParams();

  return (
    <div className="space-y-2">
      <h2 className="text-xl font-semibold">Table: {tableName}</h2>
      <p className="text-muted-foreground text-sm">
        Table inspector is not available yet.
      </p>
    </div>
  );
}
