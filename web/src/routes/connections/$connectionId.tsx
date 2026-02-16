import { Outlet, createFileRoute } from "@tanstack/react-router";

export const Route = createFileRoute("/connections/$connectionId")({
  component: ConnectionBase,
  beforeLoad: () => {
    return {
      breadcrumb: "Connection",
    };
  },
});

function ConnectionBase() {
  return <Outlet />;
}
