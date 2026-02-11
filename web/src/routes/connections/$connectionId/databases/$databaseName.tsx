import { Outlet, createFileRoute } from "@tanstack/react-router";

export const Route = createFileRoute(
  "/connections/$connectionId/databases/$databaseName",
)({
  component: DatabaseLayout,
});

function DatabaseLayout() {
  return <Outlet />;
}
