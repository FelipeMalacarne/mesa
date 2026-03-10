import { useListIndexes } from "@/api/connections/connections";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

interface IndexesTabProps {
  connectionId: string;
  databaseName: string;
  tableName: string;
}

function IndexesSkeleton() {
  return (
    <div className="animate-in fade-in-0 duration-300 delay-150 space-y-2">
      {Array.from({ length: 3 }).map((_, i) => (
        <Skeleton key={i} className="h-10 w-full" />
      ))}
    </div>
  );
}

export function IndexesTab({ connectionId, databaseName, tableName }: IndexesTabProps) {
  const { data: indexes, isLoading, isError } = useListIndexes(
    connectionId,
    databaseName,
    tableName,
    { query: { staleTime: 60_000 } },
  );

  if (isLoading) return <IndexesSkeleton />;
  if (isError) return <p className="text-sm text-destructive">Failed to load indexes.</p>;

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Name</TableHead>
          <TableHead>Type</TableHead>
          <TableHead>Columns</TableHead>
          <TableHead>Unique</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {indexes?.map((index) => (
          <TableRow key={index.name}>
            <TableCell className="font-medium">{index.name}</TableCell>
            <TableCell>{index.method}</TableCell>
            <TableCell>{index.columns.join(", ")}</TableCell>
            <TableCell>{index.unique ? "Yes" : "No"}</TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  );
}
