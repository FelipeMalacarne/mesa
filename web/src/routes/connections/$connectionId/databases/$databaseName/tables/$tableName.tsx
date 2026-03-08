import { createFileRoute } from "@tanstack/react-router";

import { Breadcrumbs } from "@/components/breadcrumbs";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ColumnsTab } from "./_components/columns-tab";
import { IndexesTab } from "./_components/indexes-tab";
import { SampleTab } from "./_components/sample-tab";

export const Route = createFileRoute(
  "/connections/$connectionId/databases/$databaseName/tables/$tableName",
)({
  component: DatabaseTable,
  beforeLoad: ({ params }) => {
    return {
      breadcrumb: params.tableName,
    };
  },
});

function DatabaseTable() {
  const { tableName, databaseName, connectionId } = Route.useParams();

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <Breadcrumbs />
          <h2 className="text-2xl font-semibold">{tableName}</h2>
          <div className="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
            <Badge variant="secondary">Primary table</Badge>
            <span>•</span>
            <span>Public schema</span>
          </div>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline">Query</Button>
          <Button variant="outline">Insert Row</Button>
          <Button>New Index</Button>
        </div>
      </div>

      <Tabs defaultValue="columns">
        <TabsList variant="line">
          <TabsTrigger value="columns">Columns</TabsTrigger>
          <TabsTrigger value="indexes">Indexes</TabsTrigger>
          <TabsTrigger value="sample">Sample data</TabsTrigger>
        </TabsList>
        <TabsContent value="columns" className="mt-4">
          <ColumnsTab
            connectionId={connectionId}
            databaseName={databaseName}
            tableName={tableName}
          />
        </TabsContent>
        <TabsContent value="indexes" className="mt-4">
          <IndexesTab />
        </TabsContent>
        <TabsContent value="sample" className="mt-4">
          <SampleTab />
        </TabsContent>
      </Tabs>
    </div>
  );
}
