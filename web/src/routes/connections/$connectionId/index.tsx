import { createFileRoute } from "@tanstack/react-router";

export const Route = createFileRoute("/connections/$connectionId/")({
  component: ConnectionDetail,
});

function ConnectionDetail() {
  const { connectionId } = Route.useParams();

  return (
    <div className="p-6">
      <h1 className="text-2xl font-semibold">Connection</h1>
      <p className="text-muted-foreground">{connectionId}</p>
    </div>
  );
}
