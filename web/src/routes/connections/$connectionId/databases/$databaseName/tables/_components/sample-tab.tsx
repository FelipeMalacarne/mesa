import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

// TODO: replace with real API data
const sampleRows = [
  { id: "ord_1042", customer_id: "cus_90a1", status: "paid", total_cents: 12900, created_at: "2024-10-22 14:20:03" },
  { id: "ord_1043", customer_id: "cus_90a1", status: "refunded", total_cents: 5400, created_at: "2024-10-22 14:24:19" },
  { id: "ord_1044", customer_id: "cus_18d2", status: "pending", total_cents: 2200, created_at: "2024-10-22 14:27:55" },
];

export function SampleTab() {
  return (
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
  );
}
