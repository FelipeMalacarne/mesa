import { createFileRoute } from "@tanstack/react-router";
import { Button } from "@/components/ui/button";
import { useQuery } from "@tanstack/react-query";
import { ConnectionsService } from "@/api";

export const Route = createFileRoute("/")({
  component: App,
});

function App() {
  const { data: accounts, isLoading } = useQuery({
    queryKey: ["connections"],
    queryFn: async () => (await ConnectionsService.getConnections()) ?? [],
  });
  console.log("accounts", accounts);

  return (
    <div className="text-center">
      <Button>testeee</Button>
    </div>
  );
}
