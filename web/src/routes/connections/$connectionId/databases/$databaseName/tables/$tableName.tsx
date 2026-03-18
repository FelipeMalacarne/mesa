import { createFileRoute } from "@tanstack/react-router";

import { Breadcrumbs } from "@/components/breadcrumbs";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { ColumnsTab } from "./_components/columns-tab";
import { IndexesTab } from "./_components/indexes-tab";
import { RowsTab } from "./_components/rows-tab";

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
      <div>
        <Breadcrumbs />
        <h2 className="text-2xl font-semibold">{tableName}</h2>
      </div>

      <Tabs defaultValue="rows">
        <TabsList variant="line">
          <TabsTrigger value="rows">Rows</TabsTrigger>
          <TabsTrigger value="columns">Columns</TabsTrigger>
          <TabsTrigger value="indexes">Indexes</TabsTrigger>
        </TabsList>
        <TabsContent value="columns" className="mt-4">
          <ColumnsTab
            connectionId={connectionId}
            databaseName={databaseName}
            tableName={tableName}
          />
        </TabsContent>
        <TabsContent value="indexes" className="mt-4">
          <IndexesTab
            connectionId={connectionId}
            databaseName={databaseName}
            tableName={tableName}
          />
        </TabsContent>
        <TabsContent value="rows" className="mt-4">
          <RowsTab
            connectionId={connectionId}
            databaseName={databaseName}
            tableName={tableName}
          />
        </TabsContent>
      </Tabs>
    </div>
  );
}
