import { Link, Outlet, createFileRoute, useMatchRoute } from "@tanstack/react-router";
import { Database } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { cn } from "@/lib/utils";
import {
  useConnectionDetails,
  useConnectionOverview,
} from "@/hooks/use-connection";

const TABS = [
  { href: "/connections/$connectionId/overview", label: "Overview" },
  { href: "/connections/$connectionId/databases", label: "Databases" },
  { href: "/connections/$connectionId/users", label: "Users" },
  { href: "/connections/$connectionId/monitor", label: "Monitor" },
];

export const Route = createFileRoute("/connections/$connectionId")({
  component: ConnectionLayout,
});

function ConnectionLayout() {
  const matchRoute = useMatchRoute();
  const isDatabaseDetailRoute = Boolean(
    matchRoute({
      to: "/connections/$connectionId/databases/$databaseName",
      fuzzy: true,
    }),
  );
  const { connectionId } = Route.useParams();
  const connectionQuery = useConnectionDetails(connectionId);
  const overviewQuery = useConnectionOverview(connectionId);

  const connection = connectionQuery.data;
  const overview = overviewQuery.data;

  if (isDatabaseDetailRoute) {
    return <Outlet />;
  }

  return (
    <div className="flex h-full flex-col">
      <header className="flex flex-col gap-3 border-b bg-background/95 px-6 py-4 backdrop-blur">
        <div className="flex items-center gap-3">
          <div className="rounded-lg border bg-primary/10 p-2 text-primary">
            <Database className="size-6" />
          </div>
          <div className="flex flex-col">
            {connection ? (
              <div className="flex items-center gap-3">
                <h1 className="text-xl font-semibold">{connection.name}</h1>
                <StatusBadge status={overview?.status} />
              </div>
            ) : (
              <Skeleton className="h-6 w-48" />
            )}
            <p className="text-muted-foreground text-sm">
              {connection ? `${connection.driver.toUpperCase()} • ${connection.host}` : "Loading connection info..."}
            </p>
          </div>
        </div>
        <div className="flex flex-wrap gap-6 text-sm text-muted-foreground">
          <Metric label="Version" value={overview?.version ?? "-"} loading={overviewQuery.isLoading} />
          <Metric label="Uptime" value={overview?.uptime ?? "-"} loading={overviewQuery.isLoading} />
          <Metric label="Sessions" value={overview?.sessions ?? "-"} loading={overviewQuery.isLoading} />
          <Metric
            label="Latency"
            value={overview ? `${overview.latency_ms} ms` : "-"}
            loading={overviewQuery.isLoading}
          />
        </div>
      </header>
      <nav className="border-b px-6">
        <div className="flex gap-4">
          {TABS.map((tab) => (
            <Link
              key={tab.href}
              to={tab.href}
              params={{ connectionId }}
              className={cn(
                "border-b-2 border-transparent px-2 py-3 text-sm font-medium text-muted-foreground transition-colors hover:text-foreground",
              )}
              activeProps={{
                className: cn(
                  "border-b-2 border-primary px-2 py-3 text-sm font-medium text-foreground",
                ),
              }}
            >
              {tab.label}
            </Link>
          ))}
        </div>
      </nav>
      <div className="flex-1 overflow-auto bg-muted/5 p-6">
        <Outlet />
      </div>
    </div>
  );
}

function StatusBadge({ status }: { status?: string }) {
  if (!status) {
    return <Skeleton className="h-5 w-16" />;
  }

  const variant = status === "ONLINE" ? "default" : "destructive";
  const label = status === "ONLINE" ? "Online" : "Unreachable";

  return <Badge variant={variant}>{label}</Badge>;
}

function Metric({
  label,
  value,
  loading,
}: {
  label: string;
  value: string;
  loading: boolean;
}) {
  return (
    <div>
      <p className="text-xs uppercase tracking-wide text-muted-foreground">{label}</p>
      {loading ? <Skeleton className="mt-1 h-4 w-16" /> : <p>{value}</p>}
    </div>
  );
}
