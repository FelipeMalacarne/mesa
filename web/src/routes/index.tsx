import { createFileRoute } from "@tanstack/react-router";
import { Button } from "@/components/ui/button";
import { useQuery } from "@tanstack/react-query";
import { ConnectionsService } from "@/api";
import { Textarea } from "@/components/ui/textarea";

export const Route = createFileRoute("/")({
  component: App,
});

function App() {
  const { data: connections, isLoading } = useQuery({
    queryKey: ["connections"],
    queryFn: async () => (await ConnectionsService.listConnections()) ?? [],
  });

  const connectionId = connections?.[0]?.id ?? "";

  const { data: connection } = useQuery({
    queryKey: ["firstConnection"],
    queryFn: async () =>
      (await ConnectionsService.findConnection({
        connectionId: 'b027abcf-cef6-4e34-83ed-e37258eda3c4',
      })) ?? [],
  });

  console.log("connections", connections);
  // console.log("connection", connection);

  return (
    <div className="text-center">
      <Textarea className="h-100" value={JSON.stringify(connections, null, 2)} />
      <Button>testeee</Button>
    </div>
  );
}
