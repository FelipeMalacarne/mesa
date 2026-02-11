import * as React from "react";
import { zodResolver } from "@hookform/resolvers/zod";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useForm } from "react-hook-form";
import { toast } from "sonner";
import * as z from "zod";

import { Button } from "@/components/ui/button";
import {
  Form,
  FormControl,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from "@/components/ui/form";
import { Input } from "@/components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { ApiError, ConnectionsService, CreateConnectionRequest } from "@/api";

const formSchema = z.object({
  name: z
    .string()
    .min(3, "Connection name must be at least 3 characters.")
    .max(255, "Connection name must be at most 255 characters."),
  driver: z.nativeEnum(CreateConnectionRequest.driver),
  host: z
    .string()
    .min(1, "Host is required.")
    .max(255, "Host must be at most 255 characters."),
  port: z
    .number()
    .int("Port must be an integer.")
    .min(0, "Port must be greater than or equal to 0.")
    .max(65535, "Port must be less than or equal to 65535."),
  username: z
    .string()
    .min(1, "Username is required.")
    .max(255, "Username must be at most 255 characters."),
  password: z
    .string()
    .min(1, "Password is required.")
    .max(255, "Password must be at most 255 characters."),
});

type ConnectionFormValues = z.infer<typeof formSchema>;

export function ConnectionForm({ onSuccess }: { onSuccess?: () => void }) {
  const [selectContainer, setSelectContainer] =
    React.useState<HTMLElement | null>(null);
  const queryClient = useQueryClient();
  const form = useForm<ConnectionFormValues>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      name: "",
      driver: CreateConnectionRequest.driver.POSTGRES,
      host: "",
      port: 5432,
      username: "",
      password: "",
    },
  });

  const createConnection = useMutation({
    mutationFn: (values: ConnectionFormValues) =>
      ConnectionsService.createConnection({ requestBody: values }),
    onSuccess: (connection) => {
      toast("Connection created", {
        description: `${connection.name} (${connection.driver})`,
      });
      queryClient.invalidateQueries({ queryKey: ["connections"] });
      queryClient.invalidateQueries({ queryKey: ["connections-tree"] });
      form.reset();
      onSuccess?.();
    },
    onError: (error) => {
      const message =
        error instanceof ApiError
          ? (error.body?.message ??
            error.statusText ??
            `Request failed (${error.status})`)
          : "Unable to create connection.";

      toast("Connection failed", {
        description: message,
      });
    },
  });

  const isSubmitting =
    createConnection.isPending || form.formState.isSubmitting;

  const onSubmit = async (values: ConnectionFormValues) => {
    await createConnection.mutateAsync(values);
  };

  return (
    <Form {...form}>
      <form
        id="connection-form"
        onSubmit={form.handleSubmit(onSubmit)}
        className="space-y-6"
        ref={setSelectContainer}
      >
        <FormField
          control={form.control}
          name="name"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Connection Name</FormLabel>
              <FormControl>
                <Input
                  placeholder="Production Connection"
                  autoComplete="off"
                  disabled={isSubmitting}
                  {...field}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name="driver"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Driver</FormLabel>
              <Select
                name={field.name}
                value={field.value}
                onValueChange={field.onChange}
                disabled={isSubmitting}
              >
                <FormControl>
                  <SelectTrigger>
                    <SelectValue placeholder="Select a driver" />
                  </SelectTrigger>
                </FormControl>
                <SelectContent container={selectContainer ?? undefined}>
                  <SelectItem value={CreateConnectionRequest.driver.POSTGRES}>
                    Postgres
                  </SelectItem>
                  <SelectItem value={CreateConnectionRequest.driver.MYSQL}>
                    MySQL
                  </SelectItem>
                </SelectContent>
              </Select>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name="host"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Host</FormLabel>
              <FormControl>
                <Input
                  placeholder="127.0.0.1"
                  autoComplete="off"
                  disabled={isSubmitting}
                  {...field}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name="port"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Port</FormLabel>
              <FormControl>
                <Input
                  type="number"
                  min={0}
                  max={65535}
                  step={1}
                  inputMode="numeric"
                  autoComplete="off"
                  disabled={isSubmitting}
                  {...field}
                  onChange={(event) => {
                    const nextValue = event.target.valueAsNumber;
                    field.onChange(Number.isNaN(nextValue) ? 0 : nextValue);
                  }}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name="username"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Username</FormLabel>
              <FormControl>
                <Input
                  placeholder="db_user"
                  autoComplete="off"
                  disabled={isSubmitting}
                  {...field}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name="password"
          render={({ field }) => (
            <FormItem>
              <FormLabel>Password</FormLabel>
              <FormControl>
                <Input
                  type="password"
                  placeholder="••••••••"
                  autoComplete="new-password"
                  disabled={isSubmitting}
                  {...field}
                />
              </FormControl>
              <FormMessage />
            </FormItem>
          )}
        />

        <Button type="submit" disabled={isSubmitting}>
          {isSubmitting ? "Saving..." : "Submit"}
        </Button>
      </form>
    </Form>
  );
}
