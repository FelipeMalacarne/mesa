import { createFileRoute } from "@tanstack/react-router";
import { Button } from "@/components/ui/button";
import { useQuery } from "@tanstack/react-query";
import { ConnectionsService } from "@/api";
import { Textarea } from "@/components/ui/textarea";

export const Route = createFileRoute("/")({
  component: App,
});

function App() {
	const { data: connections } = useQuery({
    queryKey: ["connections"],
    queryFn: async () => (await ConnectionsService.listConnections()) ?? [],
  });

  console.log("connections", connections);
  // console.log("connection", connection);

  return (
    <div className="text-center">
      <Textarea
        className="h-100"
        value={JSON.stringify(connections, null, 2)}
      />
      <Button>testeee</Button>
    </div>
  );
}
