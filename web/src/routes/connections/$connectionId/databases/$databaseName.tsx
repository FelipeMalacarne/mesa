import { createFileRoute } from "@tanstack/react-router";

export const Route = createFileRoute(
  "/connections/$connectionId/databases/$databaseName",
)({
  component: SelectedDatabase,
});

function SelectedDatabase() {
  const { databaseName } = Route.useParams();

  return (
    <div className="space-y-2">
      <h2 className="text-xl font-semibold">Database: {databaseName}</h2>
      <p className="text-muted-foreground text-sm">
        Detailed table view coming soon.
      </p>
    </div>
  );
}
