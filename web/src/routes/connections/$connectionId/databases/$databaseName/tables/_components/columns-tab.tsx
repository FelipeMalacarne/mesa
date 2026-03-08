import { useListColumns } from "@/api/connections/connections";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

interface ColumnsTabProps {
  connectionId: string;
  databaseName: string;
  tableName: string;
}

function ColumnsSkeleton() {
  return (
    <div className="animate-in fade-in-0 duration-300 delay-150 space-y-2">
      {Array.from({ length: 5 }).map((_, i) => (
        <Skeleton key={i} className="h-10 w-full" />
      ))}
    </div>
  );
}

export function ColumnsTab({ connectionId, databaseName, tableName }: ColumnsTabProps) {
  const { data: columns, isLoading, isError } = useListColumns(
    connectionId,
    databaseName,
    tableName,
    { query: { staleTime: 60_000 } },
  );

  if (isLoading) return <ColumnsSkeleton />;
  if (isError) return <p className="text-sm text-destructive">Failed to load columns.</p>;

  return (
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
  );
}
