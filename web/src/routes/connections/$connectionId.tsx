import { Outlet, createFileRoute } from "@tanstack/react-router";

export const Route = createFileRoute("/connections/$connectionId")({
  component: ConnectionBase,
});

function ConnectionBase() {
  return <Outlet />;
}
