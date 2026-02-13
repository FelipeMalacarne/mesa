import { Link, createFileRoute } from "@tanstack/react-router";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
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
  "/connections/$connectionId/databases/$databaseName/tables/$tableName",
)({
  component: DatabaseTable,
});

function DatabaseTable() {
  const { connectionId, databaseName, tableName } = Route.useParams();
  const summary = {
    rows: "1.2M",
    size: "840 MB",
    indexSize: "210 MB",
    lastAnalyze: "18m ago",
    columns: 14,
  };

  const columns = [
    { name: "id", type: "uuid", nullable: false, defaultValue: "gen_random_uuid()", tags: ["PK"] },
    { name: "customer_id", type: "uuid", nullable: false, defaultValue: null, tags: ["FK"] },
    { name: "status", type: "text", nullable: false, defaultValue: "pending", tags: [] },
    { name: "total_cents", type: "int4", nullable: false, defaultValue: "0", tags: [] },
    { name: "created_at", type: "timestamptz", nullable: false, defaultValue: "now()", tags: [] },
  ];

  const indexes = [
    { name: "orders_pkey", type: "btree", columns: "id", size: "64 MB", unique: true },
    { name: "orders_customer_id_idx", type: "btree", columns: "customer_id", size: "48 MB", unique: false },
    { name: "orders_status_created_idx", type: "btree", columns: "status, created_at", size: "32 MB", unique: false },
  ];

  const sampleRows = [
    {
      id: "ord_1042",
      customer_id: "cus_90a1",
      status: "paid",
      total_cents: 12900,
      created_at: "2024-10-22 14:20:03",
    },
    {
      id: "ord_1043",
      customer_id: "cus_90a1",
      status: "refunded",
      total_cents: 5400,
      created_at: "2024-10-22 14:24:19",
    },
    {
      id: "ord_1044",
      customer_id: "cus_18d2",
      status: "pending",
      total_cents: 2200,
      created_at: "2024-10-22 14:27:55",
    },
  ];

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <div className="flex items-center gap-2 text-sm text-muted-foreground">
            <Link
              to="/connections/$connectionId/databases/$databaseName"
              params={{ connectionId, databaseName }}
              className="hover:text-foreground"
            >
              {databaseName}
            </Link>
            <span>/</span>
            <span className="text-foreground">{tableName}</span>
          </div>
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

      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-5">
        <Card className="xl:col-span-1">
          <CardHeader>
            <CardTitle className="text-sm text-muted-foreground">Rows</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-semibold">{summary.rows}</p>
          </CardContent>
        </Card>
        <Card className="xl:col-span-1">
          <CardHeader>
            <CardTitle className="text-sm text-muted-foreground">Table size</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-semibold">{summary.size}</p>
          </CardContent>
        </Card>
        <Card className="xl:col-span-1">
          <CardHeader>
            <CardTitle className="text-sm text-muted-foreground">Index size</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-semibold">{summary.indexSize}</p>
          </CardContent>
        </Card>
        <Card className="xl:col-span-1">
          <CardHeader>
            <CardTitle className="text-sm text-muted-foreground">Columns</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-semibold">{summary.columns}</p>
          </CardContent>
        </Card>
        <Card className="xl:col-span-1">
          <CardHeader>
            <CardTitle className="text-sm text-muted-foreground">Last analyze</CardTitle>
          </CardHeader>
          <CardContent>
            <p className="text-2xl font-semibold">{summary.lastAnalyze}</p>
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle className="text-lg">Inspector</CardTitle>
        </CardHeader>
        <CardContent>
          <Tabs defaultValue="columns">
            <TabsList variant="line">
              <TabsTrigger value="columns">Columns</TabsTrigger>
              <TabsTrigger value="indexes">Indexes</TabsTrigger>
              <TabsTrigger value="sample">Sample data</TabsTrigger>
            </TabsList>
            <TabsContent value="columns" className="mt-4">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead>Type</TableHead>
                    <TableHead>Nullable</TableHead>
                    <TableHead>Default</TableHead>
                    <TableHead>Tags</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {columns.map((column) => (
                    <TableRow key={column.name}>
                      <TableCell className="font-medium">{column.name}</TableCell>
                      <TableCell>{column.type}</TableCell>
                      <TableCell>{column.nullable ? "Yes" : "No"}</TableCell>
                      <TableCell>{column.defaultValue ?? "-"}</TableCell>
                      <TableCell>
                        <div className="flex flex-wrap gap-2">
                          {column.tags.length === 0 ? (
                            <span className="text-muted-foreground">-</span>
                          ) : (
                            column.tags.map((tag) => (
                              <Badge key={tag} variant="secondary">
                                {tag}
                              </Badge>
                            ))
                          )}
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </TabsContent>
            <TabsContent value="indexes" className="mt-4">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>Name</TableHead>
                    <TableHead>Type</TableHead>
                    <TableHead>Columns</TableHead>
                    <TableHead>Size</TableHead>
                    <TableHead>Unique</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {indexes.map((index) => (
                    <TableRow key={index.name}>
                      <TableCell className="font-medium">{index.name}</TableCell>
                      <TableCell>{index.type}</TableCell>
                      <TableCell>{index.columns}</TableCell>
                      <TableCell>{index.size}</TableCell>
                      <TableCell>{index.unique ? "Yes" : "No"}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </TabsContent>
            <TabsContent value="sample" className="mt-4">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>ID</TableHead>
                    <TableHead>Customer</TableHead>
                    <TableHead>Status</TableHead>
                    <TableHead>Total</TableHead>
                    <TableHead>Created</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {sampleRows.map((row) => (
                    <TableRow key={row.id}>
                      <TableCell className="font-medium">{row.id}</TableCell>
                      <TableCell>{row.customer_id}</TableCell>
                      <TableCell className="capitalize">{row.status}</TableCell>
                      <TableCell>${(row.total_cents / 100).toFixed(2)}</TableCell>
                      <TableCell>{row.created_at}</TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </TabsContent>
          </Tabs>
        </CardContent>
      </Card>
    </div>
  );
}
