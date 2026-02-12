import { createFileRoute } from "@tanstack/react-router";
import { useEffect } from "react";

export const Route = createFileRoute("/connections/$connectionId/")({
  component: ConnectionRedirect,
});

function ConnectionRedirect() {
  const { connectionId } = Route.useParams();
  const navigate = Route.useNavigate();

  useEffect(() => {
    navigate({
      to: "/connections/$connectionId/overview",
      params: { connectionId },
      replace: true,
    });
  }, [connectionId, navigate]);

  return null;
}
