import { createFileRoute } from "@tanstack/react-router";
import logo from "../logo.svg";
import { Button } from "@/components/ui/button";

export const Route = createFileRoute("/")({
  component: App,
});

function App() {
  return (
    <div className="text-center">
      <Button>testeee</Button>
    </div>
  );
}
