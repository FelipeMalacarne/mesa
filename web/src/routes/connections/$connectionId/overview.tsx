import { createFileRoute } from "@tanstack/react-router";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { useConnectionOverview } from "@/hooks/use-connection";

export const Route = createFileRoute("/connections/$connectionId/overview")({
  component: ConnectionOverview,
});

function ConnectionOverview() {
  const { connectionId } = Route.useParams();
  const { data, isLoading } = useConnectionOverview(connectionId);

  const cards = [
    { label: "Status", value: data?.status ?? "-" },
    { label: "Version", value: data?.version ?? "-" },
    { label: "Uptime", value: data?.uptime ?? "-" },
    { label: "Sessions", value: data?.sessions ?? "-" },
    { label: "Latency", value: data ? `${data.latency_ms} ms` : "-" },
  ];

  return (
    <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
      {cards.map((card) => (
        <Card key={card.label}>
          <CardHeader>
            <CardTitle className="text-sm text-muted-foreground">
              {card.label}
            </CardTitle>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <Skeleton className="h-6 w-24" />
            ) : (
              <p className="text-2xl font-semibold">{card.value}</p>
            )}
          </CardContent>
        </Card>
      ))}
    </div>
  );
}
