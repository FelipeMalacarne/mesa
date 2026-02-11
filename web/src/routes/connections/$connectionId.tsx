import { Outlet, createFileRoute } from "@tanstack/react-router";

export const Route = createFileRoute("/connections/$connectionId")({
  component: ConnectionLayout,
});

function ConnectionLayout() {
  return <Outlet />;
}
