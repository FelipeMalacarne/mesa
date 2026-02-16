import { Outlet, createFileRoute } from "@tanstack/react-router";

export const Route = createFileRoute(
  "/connections/$connectionId/databases/$databaseName",
)({
  component: DatabaseLayout,
  beforeLoad: ({ params }) => {
    return {
      breadcrumb: params.databaseName,
    };
  },
});

function DatabaseLayout() {
  return (
    <div className="flex h-full flex-col">
      <div className="flex-1 overflow-auto p-6">
        <Outlet />
      </div>
    </div>
  );
}
