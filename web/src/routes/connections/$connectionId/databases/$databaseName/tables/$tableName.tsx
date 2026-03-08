import { createFileRoute } from "@tanstack/react-router";

import { Breadcrumbs } from "@/components/breadcrumbs";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useListColumns } from "@/api/connections/connections";

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

function ColumnsSkeleton() {
  return (
    <div className="animate-in fade-in-0 duration-300 delay-150 space-y-2">
      {Array.from({ length: 5 }).map((_, i) => (
        <Skeleton key={i} className="h-10 w-full" />
      ))}
    </div>
  );
}

function DatabaseTable() {
  const { tableName, databaseName, connectionId } = Route.useParams();

  const {
    data: columns,
    isLoading,
    isError,
  } = useListColumns(connectionId, databaseName, tableName, {
    query: { staleTime: 60_000 },
  });

  const indexes = [
    {
      name: "orders_pkey",
      type: "btree",
      columns: "id",
      size: "64 MB",
      unique: true,
    },
    {
      name: "orders_customer_id_idx",
      type: "btree",
      columns: "customer_id",
      size: "48 MB",
      unique: false,
    },
    {
      name: "orders_status_created_idx",
      type: "btree",
      columns: "status, created_at",
      size: "32 MB",
      unique: false,
    },
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

      {/* <DataTable columns={columns} data={sampleRows} /> */}

      <Tabs defaultValue="columns">
        <TabsList variant="line">
          <TabsTrigger value="columns">Columns</TabsTrigger>
          <TabsTrigger value="indexes">Indexes</TabsTrigger>
          <TabsTrigger value="sample">Sample data</TabsTrigger>
        </TabsList>
        <TabsContent value="columns" className="mt-4">
          {isLoading ? (
            <ColumnsSkeleton />
          ) : isError ? (
            <p className="text-sm text-destructive">Failed to load columns.</p>
          ) : (
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
                {columns?.map((column) => (
                  <TableRow key={column.name}>
                    <TableCell className="font-medium">{column.name}</TableCell>
                    <TableCell>{column.type}</TableCell>
                    <TableCell>{column.nullable ? "Yes" : "No"}</TableCell>
                    <TableCell>{column.default_value ?? "-"}</TableCell>
                    <TableCell>
                      {column.primary ? (
                        <Badge variant="secondary">PK</Badge>
                      ) : (
                        <span className="text-muted-foreground">-</span>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
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
    </div>
  );
}
