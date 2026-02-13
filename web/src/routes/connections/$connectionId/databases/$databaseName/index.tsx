import { createFileRoute } from "@tanstack/react-router";

export const Route = createFileRoute(
  "/connections/$connectionId/databases/$databaseName/",
)({
  component: DatabaseDetail,
});

function DatabaseDetail() {
  const { connectionId, databaseName } = Route.useParams();

  return (
    <div className="space-y-2">
      <h1 className="text-2xl font-semibold">Database</h1>
      <p className="text-muted-foreground">
        {databaseName} ({connectionId})
      </p>
    </div>
  );
}
