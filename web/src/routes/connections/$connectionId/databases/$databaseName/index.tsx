import { Link, createFileRoute } from "@tanstack/react-router";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";

export const Route = createFileRoute(
  "/connections/$connectionId/databases/$databaseName/",
)({
  component: DatabaseDetail,
});

function DatabaseDetail() {
  const { connectionId, databaseName } = Route.useParams();
  const summary = {
    owner: "postgres",
    encoding: "UTF8",
    collation: "en_US.UTF-8",
    size: "3.2 GB",
    tables: 42,
    schemas: 6,
    connections: 14,
    lastVacuum: "2h ago",
  };

  const storage = [
    { label: "Table data", percent: 62, size: "2.0 GB" },
    { label: "Indexes", percent: 28, size: "920 MB" },
    { label: "TOAST", percent: 10, size: "320 MB" },
  ];

  const tables = [
    { name: "orders", rows: "1.2M", size: "840 MB", type: "base", updated: "5m ago" },
    { name: "customers", rows: "410K", size: "320 MB", type: "base", updated: "12m ago" },
    { name: "subscriptions", rows: "120K", size: "180 MB", type: "base", updated: "30m ago" },
    { name: "event_log", rows: "24M", size: "1.1 GB", type: "partitioned", updated: "2m ago" },
  ];

  const schemas = [
    { name: "public", tables: 28, size: "2.4 GB", owner: "postgres" },
    { name: "billing", tables: 7, size: "420 MB", owner: "billing" },
    { name: "analytics", tables: 6, size: "310 MB", owner: "etl" },
    { name: "audit", tables: 1, size: "90 MB", owner: "security" },
  ];

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Link
              to="/connections/$connectionId/overview"
              params={{ connectionId }}
              className="hover:text-foreground"
            >
              Connection
            </Link>
            <span>/</span>
            <span className="text-foreground">{databaseName}</span>
          </div>
          <h1 className="text-2xl font-semibold">{databaseName}</h1>
          <p className="text-muted-foreground text-sm">
            Owner {summary.owner} • {summary.encoding} • {summary.collation}
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Button variant="outline">Open SQL Editor</Button>
          <Button>New Table</Button>
        </div>
      </div>

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        <Card>
          <CardHeader>
            <CardTitle className="text-sm text-muted-foreground">Size</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-semibold">{summary.size}</p>
            <p className="text-xs text-muted-foreground">Including indexes</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="text-sm text-muted-foreground">Tables</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-semibold">{summary.tables}</p>
            <p className="text-xs text-muted-foreground">{summary.schemas} schemas</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="text-sm text-muted-foreground">Connections</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-semibold">{summary.connections}</p>
            <p className="text-xs text-muted-foreground">Active right now</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="text-sm text-muted-foreground">Maintenance</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-semibold">{summary.lastVacuum}</p>
            <p className="text-xs text-muted-foreground">Last vacuum</p>
          </CardContent>
        </Card>
      </div>

      <div className="grid gap-4 lg:grid-cols-[2fr,1fr]">
        <Card>
          <CardHeader>
            <div className="flex items-center justify-between">
              <CardTitle className="text-lg">Inventory</CardTitle>
              <Badge variant="secondary">Mock data</Badge>
            </div>
          </CardHeader>
          <CardContent>
            <Tabs defaultValue="tables">
              <TabsList variant="line">
                <TabsTrigger value="tables">Tables</TabsTrigger>
                <TabsTrigger value="schemas">Schemas</TabsTrigger>
              </TabsList>
              <TabsContent value="tables" className="mt-4">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Name</TableHead>
                      <TableHead>Rows</TableHead>
                      <TableHead>Size</TableHead>
                      <TableHead>Type</TableHead>
                      <TableHead>Updated</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {tables.map((table) => (
                      <TableRow key={table.name}>
                        <TableCell className="font-medium">
                          <Link
                            to="/connections/$connectionId/databases/$databaseName/tables/$tableName"
                            params={{
                              connectionId,
                              databaseName,
                              tableName: table.name,
                            }}
                            className="hover:text-foreground text-foreground/90"
                          >
                            {table.name}
                          </Link>
                        </TableCell>
                        <TableCell>{table.rows}</TableCell>
                        <TableCell>{table.size}</TableCell>
                        <TableCell className="capitalize">{table.type}</TableCell>
                        <TableCell>{table.updated}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </TabsContent>
              <TabsContent value="schemas" className="mt-4">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>Schema</TableHead>
                      <TableHead>Tables</TableHead>
                      <TableHead>Size</TableHead>
                      <TableHead>Owner</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {schemas.map((schema) => (
                      <TableRow key={schema.name}>
                        <TableCell className="font-medium">{schema.name}</TableCell>
                        <TableCell>{schema.tables}</TableCell>
                        <TableCell>{schema.size}</TableCell>
                        <TableCell>{schema.owner}</TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              </TabsContent>
            </Tabs>
          </CardContent>
        </Card>

        <div className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="text-lg">Storage breakdown</CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              {storage.map((item) => (
                <div key={item.label} className="space-y-2">
                  <div className="flex items-center justify-between text-sm">
                    <span className="text-muted-foreground">{item.label}</span>
                    <span className="font-medium">{item.size}</span>
                  </div>
                  <Progress value={item.percent} />
                </div>
              ))}
              <p className="text-xs text-muted-foreground">
                Compression and bloat estimates are mocked for now.
              </p>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle className="text-lg">Activity snapshot</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3 text-sm">
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">Queries (5m)</span>
                <span className="font-medium">2.4k</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">Slow queries</span>
                <span className="font-medium">18</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">Replication lag</span>
                <span className="font-medium">180 ms</span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">Backups</span>
                <span className="font-medium">Healthy</span>
              </div>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}
