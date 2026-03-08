import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

// TODO: replace with real API data
const indexes = [
  { name: "orders_pkey", type: "btree", columns: "id", size: "64 MB", unique: true },
  { name: "orders_customer_id_idx", type: "btree", columns: "customer_id", size: "48 MB", unique: false },
  { name: "orders_status_created_idx", type: "btree", columns: "status, created_at", size: "32 MB", unique: false },
];

export function IndexesTab() {
  return (
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
  );
}
