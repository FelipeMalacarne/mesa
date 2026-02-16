import { Link, createFileRoute } from "@tanstack/react-router";
import { useState } from "react";
import {
  useFieldArray,
  useForm,
  type FieldArrayWithId,
  type UseFormReturn,
} from "react-hook-form";
import { toast } from "sonner";
import { zodResolver } from "@hookform/resolvers/zod";
import * as z from "zod";

import { ColumnDataType, type CreateTableIndex } from "@/api";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import { Input } from "@/components/ui/input";
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { Progress } from "@/components/ui/progress";
import { ScrollArea } from "@/components/ui/scroll-area";
import { Checkbox } from "@/components/ui/checkbox";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Switch } from "@/components/ui/switch";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { useCreateTable } from "@/hooks/use-connection";
import { PlusIcon, Trash2Icon } from "lucide-react";

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
          <CreateTableDialog
            connectionId={connectionId}
            databaseName={databaseName}
          />
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

const COLUMN_TYPE_OPTIONS = [
  { value: ColumnDataType.UUID, label: "UUID" },
  { value: ColumnDataType.TEXT, label: "Text" },
  { value: ColumnDataType.VARCHAR, label: "Varchar" },
  { value: ColumnDataType.CHAR, label: "Char" },
  { value: ColumnDataType.INTEGER, label: "Integer" },
  { value: ColumnDataType.BIGINT, label: "Bigint" },
  { value: ColumnDataType.SMALLINT, label: "Smallint" },
  { value: ColumnDataType.NUMERIC, label: "Numeric" },
  { value: ColumnDataType.DECIMAL, label: "Decimal" },
  { value: ColumnDataType.BOOLEAN, label: "Boolean" },
  { value: ColumnDataType.TIMESTAMPTZ, label: "Timestamp TZ" },
  { value: ColumnDataType.TIMESTAMP, label: "Timestamp" },
  { value: ColumnDataType.DATE, label: "Date" },
  { value: ColumnDataType.TIME, label: "Time" },
  { value: ColumnDataType.JSONB, label: "JSONB" },
  { value: ColumnDataType.JSON, label: "JSON" },
  { value: ColumnDataType.BYTEA, label: "Bytea" },
  { value: ColumnDataType.REAL, label: "Real" },
  { value: ColumnDataType.DOUBLE_PRECISION, label: "Double precision" },
  { value: ColumnDataType.SERIAL, label: "Serial" },
  { value: ColumnDataType.BIGSERIAL, label: "Bigserial" },
] as const;

const LENGTH_TYPES = new Set<ColumnDataType>([
  ColumnDataType.VARCHAR,
  ColumnDataType.CHAR,
]);

const PRECISION_TYPES = new Set<ColumnDataType>([
  ColumnDataType.NUMERIC,
  ColumnDataType.DECIMAL,
]);

const INDEX_METHOD_OPTIONS = [
  "btree",
  "hash",
  "gin",
  "gist",
  "brin",
  "spgist",
] as const;

const indexMethodEnum = z.enum(INDEX_METHOD_OPTIONS);

const createId = () => Math.random().toString(36).slice(2);

const columnSchema = z.object({
  clientId: z.string(),
  name: z.string().trim().min(1, "Column name is required"),
  type: z.nativeEnum(ColumnDataType),
  length: z.string().optional(),
  precision: z.string().optional(),
  scale: z.string().optional(),
  nullable: z.boolean(),
  primaryKey: z.boolean(),
  defaultValue: z.string().optional(),
});

const indexSchema = z.object({
  clientId: z.string(),
  name: z.string().trim().min(1, "Index name is required"),
  method: indexMethodEnum,
  unique: z.boolean(),
  columns: z.array(z.string()),
});

const tableFormSchema = z
  .object({
    name: z.string().trim().min(1, "Table name is required"),
    columns: z.array(columnSchema).min(1, "Add at least one column"),
    indexes: z.array(indexSchema),
  })
  .superRefine((values, ctx) => {
    const columnNameMap = new Map<string, string>();

    values.columns.forEach((column, index) => {
      columnNameMap.set(column.clientId, column.name);

      if (LENGTH_TYPES.has(column.type)) {
        if (!column.length?.trim()) {
          ctx.addIssue({
            code: z.ZodIssueCode.custom,
            message: "Length is required for this type",
            path: ["columns", index, "length"],
          });
        } else if (Number(column.length) <= 0 || Number.isNaN(Number(column.length))) {
          ctx.addIssue({
            code: z.ZodIssueCode.custom,
            message: "Length must be a positive number",
            path: ["columns", index, "length"],
          });
        }
      }

      if (PRECISION_TYPES.has(column.type)) {
        if (!column.precision?.trim()) {
          ctx.addIssue({
            code: z.ZodIssueCode.custom,
            message: "Precision is required for this type",
            path: ["columns", index, "precision"],
          });
        } else if (Number(column.precision) <= 0 || Number.isNaN(Number(column.precision))) {
          ctx.addIssue({
            code: z.ZodIssueCode.custom,
            message: "Precision must be a positive number",
            path: ["columns", index, "precision"],
          });
        }

        if (column.scale?.trim()) {
          if (Number(column.scale) < 0 || Number.isNaN(Number(column.scale))) {
            ctx.addIssue({
              code: z.ZodIssueCode.custom,
              message: "Scale must be zero or a positive number",
              path: ["columns", index, "scale"],
            });
          }
        }
      }
    });

    values.indexes.forEach((indexValue, idx) => {
      if (!indexValue.columns || indexValue.columns.length === 0) {
        ctx.addIssue({
          code: z.ZodIssueCode.custom,
          message: "Select at least one column",
          path: ["indexes", idx, "columns"],
        });
      } else {
        indexValue.columns.forEach((columnId, columnIdx) => {
          const columnName = columnNameMap.get(columnId);
          if (!columnName) {
            ctx.addIssue({
              code: z.ZodIssueCode.custom,
              message: "Choose a valid column",
              path: ["indexes", idx, "columns", columnIdx],
            });
          }
        });
      }
    });
  });

type TableFormValues = z.infer<typeof tableFormSchema>;
type ColumnFormValues = z.infer<typeof columnSchema>;
type IndexFormValues = z.infer<typeof indexSchema>;
type BooleanFieldName =
  | `columns.${number}.nullable`
  | `columns.${number}.primaryKey`
  | `indexes.${number}.unique`;

const createColumnField = (): ColumnFormValues => ({
  clientId: createId(),
  name: "",
  type: ColumnDataType.TEXT,
  length: "",
  precision: "",
  scale: "",
  nullable: true,
  primaryKey: false,
  defaultValue: "",
});

const createIndexField = (): IndexFormValues => ({
  clientId: createId(),
  name: "",
  method: INDEX_METHOD_OPTIONS[0],
  unique: false,
  columns: [],
});

function CreateTableDialog({
  connectionId,
  databaseName,
}: {
  connectionId: string;
  databaseName: string;
}) {
  const [open, setOpen] = useState(false);

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button>Create Table</Button>
      </DialogTrigger>
      <DialogContent className="max-w-4xl">
        <DialogHeader>
          <DialogTitle>New table</DialogTitle>
        </DialogHeader>
        <CreateTableForm
          connectionId={connectionId}
          databaseName={databaseName}
          onSuccess={() => setOpen(false)}
          onCancel={() => setOpen(false)}
        />
      </DialogContent>
    </Dialog>
  );
}

function CreateTableForm({
  connectionId,
  databaseName,
  onSuccess,
  onCancel,
}: {
  connectionId: string;
  databaseName: string;
  onSuccess?: () => void;
  onCancel?: () => void;
}) {
  const resolver = zodResolver(tableFormSchema as unknown as never) as never;
  const form = useForm<TableFormValues>({
    resolver,
    defaultValues: {
      name: "",
      columns: [createColumnField()],
      indexes: [],
    },
  });

  const columnsArray = useFieldArray({ control: form.control, name: "columns" });
  const indexesArray = useFieldArray({ control: form.control, name: "indexes" });
  const mutation = useCreateTable(connectionId, databaseName);

  const columns = form.watch("columns");

  const handleAddColumn = () => columnsArray.append(createColumnField());

  const handleRemoveColumn = (index: number) => {
    if (columnsArray.fields.length === 1) {
      toast.error("A table needs at least one column");
      return;
    }

    const removedId = form.getValues(`columns.${index}.clientId` as const);
    columnsArray.remove(index);

    const currentIndexes = form.getValues("indexes");
    currentIndexes.forEach((indexValue, idx) => {
      const nextColumns = (indexValue.columns ?? []).filter((columnId) => columnId !== removedId);
      form.setValue(`indexes.${idx}.columns` as const, nextColumns, { shouldDirty: true });
    });
  };

  const handleAddIndex = () => indexesArray.append(createIndexField());
  const handleRemoveIndex = (index: number) => indexesArray.remove(index);

  const handleSubmit = async (values: TableFormValues) => {
    const columnNameMap = new Map(
      values.columns.map((column) => [column.clientId, column.name.trim()]),
    );

    const payloadColumns = values.columns.map((column) => {
      const requiresLen = LENGTH_TYPES.has(column.type);
      const requiresPrec = PRECISION_TYPES.has(column.type);
      const lengthValue = requiresLen && column.length ? Number(column.length) : undefined;
      const precisionValue = requiresPrec && column.precision ? Number(column.precision) : undefined;
      const scaleValue = requiresPrec && column.scale ? Number(column.scale) : undefined;

      return {
        name: column.name.trim(),
        type: column.type,
        length: lengthValue,
        precision: precisionValue,
        scale: scaleValue,
        nullable: column.nullable,
        primary_key: column.primaryKey,
        default_value: column.defaultValue?.trim() || undefined,
      };
    });

    const payloadIndexes = values.indexes
      .map((index) => {
        const resolvedColumns = index.columns
          .map((clientId) => columnNameMap.get(clientId))
          .filter((value): value is string => Boolean(value?.trim()));

        return {
          name: index.name.trim(),
          method: index.method as CreateTableIndex["method"],
          unique: index.unique,
          columns: resolvedColumns,
        };
      })
      .filter((index) => index.columns.length > 0);

    try {
      await mutation.mutateAsync({
        name: values.name.trim(),
        columns: payloadColumns,
        indexes: payloadIndexes && payloadIndexes.length > 0 ? payloadIndexes : undefined,
      });
      toast.success("Table created");
      form.reset({ name: "", columns: [createColumnField()], indexes: [] });
      onSuccess?.();
    } catch (error) {
      console.error(error);
      toast.error("Failed to create table");
    }
  };

  const columnOptions = columns.map((column, index) => ({
    id: column.clientId,
    label: column.name.trim() || `Column ${index + 1}`,
  }));

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(handleSubmit)} className="space-y-6">
        <ScrollArea className="max-h-[70vh] pr-4">
          <div className="space-y-6 py-2">
            <FormField
              control={form.control}
              name="name"
              render={({ field }) => (
                <FormItem>
                  <FormLabel>Table name</FormLabel>
                  <FormControl>
                    <Input {...field} placeholder="orders" />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <ColumnsFieldArray
              form={form}
              fields={columnsArray.fields}
              onAdd={handleAddColumn}
              onRemove={handleRemoveColumn}
            />

            <IndexesFieldArray
              form={form}
              fields={indexesArray.fields}
              onAdd={handleAddIndex}
              onRemove={handleRemoveIndex}
              columnOptions={columnOptions}
            />
          </div>
        </ScrollArea>
        <DialogFooter>
          <Button
            type="button"
            variant="outline"
            onClick={() => {
              form.reset({ name: "", columns: [createColumnField()], indexes: [] });
              onCancel?.();
            }}
            disabled={mutation.isPending}
          >
            Cancel
          </Button>
          <Button type="submit" disabled={mutation.isPending}>
            {mutation.isPending ? "Creating..." : "Create"}
          </Button>
        </DialogFooter>
      </form>
    </Form>
  );
}

function ColumnsFieldArray({
  form,
  fields,
  onAdd,
  onRemove,
}: {
  form: UseFormReturn<TableFormValues>;
  fields: FieldArrayWithId<TableFormValues, "columns">[];
  onAdd: () => void;
  onRemove: (index: number) => void;
}) {
  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <div>
          <p className="text-sm font-medium">Columns</p>
          <p className="text-muted-foreground text-xs">
            Configure the columns that compose this table.
          </p>
        </div>
        <Button size="sm" variant="outline" onClick={onAdd}>
          <PlusIcon className="size-4" />
          Add column
        </Button>
      </div>
      <div className="space-y-4">
        {fields.map((field, index) => {
          const columnType = form.watch(`columns.${index}.type` as const);
          const requiresLength = LENGTH_TYPES.has(columnType);
          const requiresPrecision = PRECISION_TYPES.has(columnType);

          return (
            <div key={field.id} className="space-y-4 rounded-lg border p-4">
              <div className="flex items-center justify-between">
                <p className="text-sm font-medium">Column {index + 1}</p>
                <Button
                  size="icon"
                  variant="ghost"
                  onClick={() => onRemove(index)}
                  disabled={fields.length === 1}
                >
                  <Trash2Icon className="size-4" />
                  <span className="sr-only">Remove column</span>
                </Button>
              </div>
              <FormField
                control={form.control}
                name={`columns.${index}.clientId` as const}
                render={({ field }) => <input type="hidden" {...field} />}
              />
              <div className="grid gap-4 md:grid-cols-2">
                <FormField
                  control={form.control}
                  name={`columns.${index}.name` as const}
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Name</FormLabel>
                      <FormControl>
                        <Input {...field} placeholder="id" />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name={`columns.${index}.type` as const}
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Type</FormLabel>
                      <Select
                        value={field.value}
                        onValueChange={(value) => {
                          field.onChange(value as ColumnDataType);
                          const nextType = value as ColumnDataType;
                          if (!LENGTH_TYPES.has(nextType)) {
                            form.setValue(`columns.${index}.length` as const, "");
                          }
                          if (!PRECISION_TYPES.has(nextType)) {
                            form.setValue(`columns.${index}.precision` as const, "");
                            form.setValue(`columns.${index}.scale` as const, "");
                          }
                        }}
                      >
                        <FormControl>
                          <SelectTrigger>
                            <SelectValue placeholder="Select a type" />
                          </SelectTrigger>
                        </FormControl>
                        <SelectContent>
                          {COLUMN_TYPE_OPTIONS.map((option) => (
                            <SelectItem key={option.value} value={option.value}>
                              {option.label}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
              {requiresLength && (
                <FormField
                  control={form.control}
                  name={`columns.${index}.length` as const}
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Length</FormLabel>
                      <FormControl>
                        <Input
                          type="number"
                          min={1}
                          {...field}
                          onChange={(event) => field.onChange(event.target.value)}
                          placeholder="255"
                        />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              )}
              {requiresPrecision && (
                <div className="grid gap-4 md:grid-cols-2">
                  <FormField
                    control={form.control}
                    name={`columns.${index}.precision` as const}
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Precision</FormLabel>
                        <FormControl>
                          <Input
                            type="number"
                            min={1}
                            {...field}
                            onChange={(event) => field.onChange(event.target.value)}
                            placeholder="10"
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                  <FormField
                    control={form.control}
                    name={`columns.${index}.scale` as const}
                    render={({ field }) => (
                      <FormItem>
                        <FormLabel>Scale</FormLabel>
                        <FormControl>
                          <Input
                            type="number"
                            min={0}
                            {...field}
                            onChange={(event) => field.onChange(event.target.value)}
                            placeholder="2"
                          />
                        </FormControl>
                        <FormMessage />
                      </FormItem>
                    )}
                  />
                </div>
              )}
              <div className="grid gap-3 md:grid-cols-2">
                <BooleanFieldCard
                  control={form.control}
                  name={`columns.${index}.nullable` as const}
                  title="Nullable"
                  description="Allow NULL values for this column."
                />
                <BooleanFieldCard
                  control={form.control}
                  name={`columns.${index}.primaryKey` as const}
                  title="Primary key"
                  description="Include this column in the primary key."
                />
              </div>
              <FormField
                control={form.control}
                name={`columns.${index}.defaultValue` as const}
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Default value</FormLabel>
                    <FormControl>
                      <Input {...field} placeholder="now()" />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          );
        })}
      </div>
    </div>
  );
}

function IndexesFieldArray({
  form,
  fields,
  onAdd,
  onRemove,
  columnOptions,
}: {
  form: UseFormReturn<TableFormValues>;
  fields: FieldArrayWithId<TableFormValues, "indexes">[];
  onAdd: () => void;
  onRemove: (index: number) => void;
  columnOptions: Array<{ id: string; label: string }>;
}) {
  return (
    <div className="space-y-3">
      <div className="flex items-center justify-between">
        <div>
          <p className="text-sm font-medium">Indexes</p>
          <p className="text-muted-foreground text-xs">
            Optional indexes to accelerate common queries.
          </p>
        </div>
        <Button size="sm" variant="outline" onClick={onAdd}>
          <PlusIcon className="size-4" />
          Add index
        </Button>
      </div>
      {fields.length === 0 ? (
        <p className="text-muted-foreground text-sm">
          No indexes configured. Add one to improve performance.
        </p>
      ) : (
        <div className="space-y-4">
          {fields.map((field, index) => (
            <div key={field.id} className="space-y-4 rounded-lg border p-4">
              <div className="flex items-center justify-between">
                <p className="text-sm font-medium">
                  {form.watch(`indexes.${index}.name` as const)?.trim() || `Index ${index + 1}`}
                </p>
                <Button size="icon" variant="ghost" onClick={() => onRemove(index)}>
                  <Trash2Icon className="size-4" />
                  <span className="sr-only">Remove index</span>
                </Button>
              </div>
              <FormField
                control={form.control}
                name={`indexes.${index}.clientId` as const}
                render={({ field }) => <input type="hidden" {...field} />}
              />
              <div className="grid gap-4 md:grid-cols-2">
                <FormField
                  control={form.control}
                  name={`indexes.${index}.name` as const}
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Name</FormLabel>
                      <FormControl>
                        <Input {...field} placeholder="orders_status_idx" />
                      </FormControl>
                      <FormMessage />
                    </FormItem>
                  )}
                />
                <FormField
                  control={form.control}
                  name={`indexes.${index}.method` as const}
                  render={({ field }) => (
                    <FormItem>
                      <FormLabel>Method</FormLabel>
                      <Select value={field.value} onValueChange={field.onChange}>
                        <FormControl>
                          <SelectTrigger>
                            <SelectValue />
                          </SelectTrigger>
                        </FormControl>
                        <SelectContent>
                          {INDEX_METHOD_OPTIONS.map((method) => (
                            <SelectItem key={method} value={method}>
                              {method.toUpperCase()}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                      <FormMessage />
                    </FormItem>
                  )}
                />
              </div>
              <BooleanFieldCard
                control={form.control}
                name={`indexes.${index}.unique` as const}
                title="Unique"
                description="Enforce unique values for indexed columns."
              />
              <FormField
                control={form.control}
                name={`indexes.${index}.columns` as const}
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>Columns</FormLabel>
                    {columnOptions.length === 0 ? (
                      <p className="text-muted-foreground text-sm">
                        Add columns before configuring indexes.
                      </p>
                    ) : (
                      <div className="grid gap-2 md:grid-cols-2">
                        {columnOptions.map((option) => (
                          <label
                            key={`${field.name}-${option.id}`}
                            className="flex items-center gap-2 text-sm"
                          >
                            <Checkbox
                              checked={field.value?.includes(option.id) ?? false}
                              onCheckedChange={(checked) => {
                                const current = field.value ?? [];
                                if (checked) {
                                  field.onChange(Array.from(new Set([...current, option.id])));
                                } else {
                                  field.onChange(current.filter((value) => value !== option.id));
                                }
                              }}
                              disabled={!option.label.trim()}
                            />
                            <span>{option.label}</span>
                          </label>
                        ))}
                      </div>
                    )}
                    <FormMessage />
                  </FormItem>
                )}
              />
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function BooleanFieldCard({
  control,
  name,
  title,
  description,
}: {
  control: UseFormReturn<TableFormValues>["control"];
  name: BooleanFieldName;
  title: string;
  description: string;
}) {
  return (
    <FormField
      control={control}
      name={name}
      render={({ field }) => (
        <FormItem className="flex items-center justify-between rounded-md border px-3 py-2">
          <div>
            <FormLabel className="text-sm font-medium">{title}</FormLabel>
            <p className="text-muted-foreground text-xs">{description}</p>
          </div>
          <FormControl>
            <Switch checked={field.value} onCheckedChange={field.onChange} />
          </FormControl>
        </FormItem>
      )}
    />
  );
}
