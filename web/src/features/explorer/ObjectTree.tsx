/**
 * The nested object tree (`Rail` board): databases → schemas → Tables / Views / Enums /
 * Functions / Sequences → columns and indexes. Each database's schema loads when it opens.
 * The filter matches table and view names and opens the branches that hold a match.
 */
import * as React from "react";
import { useNavigate, useParams } from "@tanstack/react-router";
import {
  ChevronRight,
  Database as DatabaseIcon,
  Eye,
  FileCode2,
  Folder,
  Hash,
  KeyRound,
  Link2,
  ListOrdered,
  Plus,
  Table2,
  Tags,
} from "lucide-react";

import { useGetSchema, useListDatabases } from "@/lib/api/explore/explore";
import type { Schema, Table } from "@/lib/api/mesaAPI.schemas";
import { errorMessage } from "@/lib/api/fetch";
import { compact } from "@/lib/format";
import { cn } from "@/lib/utils";

import { useWorkspace } from "../workspace/context";
import { useNodeOpen, useTree } from "./tree.store";

interface TreeProps {
  filter: string;
  onNewDatabase: () => void;
}

export function ObjectTree({ filter, onNewDatabase }: TreeProps) {
  const { connectionId, connection } = useWorkspace();
  const dbs = useListDatabases(connectionId, { query: { staleTime: 60_000 } });
  const list = React.useMemo(() => {
    const all = dbs.data ?? [];
    // The connection's own database first, then the rest by name.
    return [...all].sort((a, b) => (a.name === connection.database ? -1 : b.name === connection.database ? 1 : a.name.localeCompare(b.name)));
  }, [dbs.data, connection.database]);

  return (
    <div className="m-tree" role="tree" aria-label={`${connection.name} objects`}>
      {dbs.isPending ? <div className="m-tree__item m-muted">Loading databases…</div> : null}
      {dbs.isError ? <div className="m-tree__item m-help--error" title={errorMessage(dbs.error)}>Cannot list databases</div> : null}
      {list.map((db) => (
        <DatabaseNode key={db.name} database={db.name} size={db.sizeBytes} filter={filter} />
      ))}
      {dbs.isSuccess ? (
        <button type="button" className="m-tree__item m-muted" onClick={onNewDatabase}>
          <span className="m-tree__chev" aria-hidden="true" />
          <Plus className="m-icon m-icon--sm" aria-hidden="true" />
          <span className="m-tree__name">New database</span>
        </button>
      ) : null}
    </div>
  );
}

function Chevron({ open }: { open: boolean }) {
  return <ChevronRight className={cn("m-tree__chev", open && "is-open")} aria-hidden="true" />;
}

function DatabaseNode({ database, size, filter }: { database: string; size?: number; filter: string }) {
  const { connectionId, connection } = useWorkspace();
  const id = `db:${database}`;
  const stored = useNodeOpen(connectionId, id);
  const toggle = useTree((s) => s.toggle);
  const open = stored || (!!filter && database === connection.database);
  return (
    <>
      <button type="button" className="m-tree__item" role="treeitem" aria-expanded={open} onClick={() => toggle(connectionId, id)} title={size ? `${database} · ${compact(size)}B` : database}>
        <Chevron open={open} />
        <DatabaseIcon className="m-icon m-icon--sm" aria-hidden="true" />
        <span className="m-tree__name">{database}</span>
      </button>
      {open ? (
        <div className="m-tree__children" role="group">
          <DatabaseChildren database={database} filter={filter} />
        </div>
      ) : null}
    </>
  );
}

function DatabaseChildren({ database, filter }: { database: string; filter: string }) {
  const { connectionId } = useWorkspace();
  const schema = useGetSchema(connectionId, database, undefined, { query: { staleTime: 30_000 } });
  if (schema.isPending) return <div className="m-tree__item m-muted"><span className="m-tree__chev" />Loading…</div>;
  if (schema.isError)
    return (
      <div className="m-tree__item" style={{ color: "var(--destructive)" }} title={errorMessage(schema.error)}>
        <span className="m-tree__chev" />
        <span className="m-tree__name">{errorMessage(schema.error)}</span>
      </div>
    );
  const schemas = schema.data.schemas.filter((s) => s.tables.length + (s.views?.length ?? 0) + (s.enums?.length ?? 0) + (s.functions?.length ?? 0) > 0 || s.name === "public");
  if (schemas.length === 0)
    return (
      <div className="m-tree__item m-muted" style={{ cursor: "default" }}>
        <span className="m-tree__chev" />
        <span className="m-tree__name">No user tables</span>
      </div>
    );
  return (
    <>
      {schemas.map((s) => (
        <SchemaNode key={s.name} database={database} schema={s} filter={filter} single={schemas.length === 1} />
      ))}
    </>
  );
}

function matches(name: string, filter: string) {
  return !filter || name.toLowerCase().includes(filter.toLowerCase());
}

function SchemaNode({ database, schema, filter, single }: { database: string; schema: Schema; filter: string; single: boolean }) {
  const { connectionId } = useWorkspace();
  const id = `schema:${database}.${schema.name}`;
  const stored = useNodeOpen(connectionId, id);
  const closed = useNodeOpen(connectionId, `closed:${id}`);
  const toggle = useTree((s) => s.toggle);
  const tables = schema.tables.filter((t) => matches(t.name, filter));
  const views = (schema.views ?? []).filter((v) => matches(v.name, filter));
  const hit = !!filter && tables.length + views.length > 0;
  if (filter && !hit) return null;
  // A database with one schema shows it open unless the person closed it.
  const open = hit || (single ? !closed : stored);
  const onToggle = () => (single ? toggle(connectionId, `closed:${id}`, open) : toggle(connectionId, id));
  return (
    <>
      <button type="button" className="m-tree__item" role="treeitem" aria-expanded={open} onClick={onToggle}>
        <Chevron open={open} />
        <Folder className="m-icon m-icon--sm" aria-hidden="true" />
        <span className="m-tree__name">{schema.name}</span>
      </button>
      {open ? (
        <div className="m-tree__children" role="group">
          <Folder_ id={`${id}/tables`} label="Tables" count={tables.length} forceOpen={hit} defaultOpen>
            {tables.length === 0 ? <Leaf label="No tables" muted /> : tables.map((t) => <TableNode key={t.name} database={database} schema={schema.name} table={t} />)}
          </Folder_>
          {views.length > 0 ? (
            <Folder_ id={`${id}/views`} label="Views" count={views.length} forceOpen={hit && views.length > 0}>
              {views.map((v) => (
                <ObjectLeaf key={v.name} icon={<Eye className="m-icon m-icon--sm" aria-hidden="true" />} label={v.name} meta={v.materialized ? "mat." : undefined} database={database} schema={schema.name} table={v.name} kind="view" />
              ))}
            </Folder_>
          ) : null}
          {!filter && (schema.enums?.length ?? 0) > 0 ? (
            <Folder_ id={`${id}/enums`} label="Enums" count={schema.enums!.length}>
              {schema.enums!.map((e) => (
                <Leaf key={e.name} icon={<Tags className="m-icon m-icon--sm" aria-hidden="true" />} label={e.name} meta={String(e.values.length)} title={e.values.join(", ")} />
              ))}
            </Folder_>
          ) : null}
          {!filter && (schema.functions?.length ?? 0) > 0 ? (
            <Folder_ id={`${id}/functions`} label="Functions" count={schema.functions!.length}>
              {schema.functions!.map((f, i) => (
                <Leaf key={`${f.name}${i}`} icon={<FileCode2 className="m-icon m-icon--sm" aria-hidden="true" />} label={`${f.name}(${f.signature ?? ""})`} title={f.returns ? `returns ${f.returns}` : undefined} />
              ))}
            </Folder_>
          ) : null}
          {!filter && (schema.sequences?.length ?? 0) > 0 ? (
            <Folder_ id={`${id}/sequences`} label="Sequences" count={schema.sequences!.length}>
              {schema.sequences!.map((s) => (
                <Leaf key={s.name} icon={<ListOrdered className="m-icon m-icon--sm" aria-hidden="true" />} label={s.name} />
              ))}
            </Folder_>
          ) : null}
        </div>
      ) : null}
    </>
  );
}

function Folder_({ id, label, count, children, forceOpen, defaultOpen }: { id: string; label: string; count: number; children: React.ReactNode; forceOpen?: boolean; defaultOpen?: boolean }) {
  const { connectionId } = useWorkspace();
  const toggle = useTree((s) => s.toggle);
  const closedKey = `closed:${id}`;
  const stored = useNodeOpen(connectionId, id);
  const closed = useNodeOpen(connectionId, closedKey);
  const open = forceOpen || (defaultOpen ? !closed : stored);
  const onClick = () => (defaultOpen ? toggle(connectionId, closedKey, open) : toggle(connectionId, id));
  return (
    <>
      <button type="button" className="m-tree__item m-tree__item--folder" role="treeitem" aria-expanded={open} onClick={onClick}>
        <Chevron open={open} />
        <span className="m-tree__name">{label}</span>
        <span className="m-tree__meta">{count}</span>
      </button>
      {open ? (
        <div className="m-tree__children" role="group">
          {children}
        </div>
      ) : null}
    </>
  );
}

function Leaf({ icon, label, meta, title, muted }: { icon?: React.ReactNode; label: string; meta?: string; title?: string; muted?: boolean }) {
  return (
    <div className={cn("m-tree__item", muted && "m-muted")} role="treeitem" style={{ cursor: "default" }} title={title ?? label}>
      <span className="m-tree__chev" aria-hidden="true" />
      {icon}
      <span className="m-tree__name">{label}</span>
      {meta ? <span className="m-tree__meta">{meta}</span> : null}
    </div>
  );
}

function useIsSelected(database: string, schema: string, table: string) {
  const params = useParams({ strict: false }) as { database?: string; schema?: string; table?: string };
  return params.database === database && params.schema === schema && params.table === table;
}

function ObjectLeaf({ icon, label, meta, database, schema, table, kind }: { icon: React.ReactNode; label: string; meta?: string; database: string; schema: string; table: string; kind: "table" | "view" }) {
  const { connectionId } = useWorkspace();
  const navigate = useNavigate();
  const selected = useIsSelected(database, schema, table);
  return (
    <button
      type="button"
      className={cn("m-tree__item", selected && "is-selected")}
      role="treeitem"
      onClick={() => void navigate({ to: "/c/$connectionId/db/$database/t/$schema/$table", params: { connectionId, database, schema, table }, search: { view: "data", kind } })}
    >
      <span className="m-tree__chev" aria-hidden="true" />
      {icon}
      <span className="m-tree__name">{label}</span>
      {meta ? <span className="m-tree__meta">{meta}</span> : null}
    </button>
  );
}

function TableNode({ database, schema, table }: { database: string; schema: string; table: Table }) {
  const { connectionId } = useWorkspace();
  const navigate = useNavigate();
  const id = `table:${database}.${schema}.${table.name}`;
  const open = useNodeOpen(connectionId, id);
  const toggle = useTree((s) => s.toggle);
  const selected = useIsSelected(database, schema, table.name);
  const fkCols = new Set((table.foreignKeys ?? []).flatMap((f) => f.columns));
  const pkCols = new Set(table.primaryKey?.columns ?? []);
  return (
    <>
      <div className={cn("m-tree__item", selected && "is-selected")} role="treeitem" aria-expanded={open} style={{ paddingLeft: 0 }}>
        <button type="button" aria-label={open ? `Collapse ${table.name}` : `Expand ${table.name}`} onClick={() => toggle(connectionId, id)} style={{ display: "grid", placeItems: "center", width: 22, height: "100%" }}>
          <Chevron open={open} />
        </button>
        <button
          type="button"
          onClick={() => void navigate({ to: "/c/$connectionId/db/$database/t/$schema/$table", params: { connectionId, database, schema, table: table.name }, search: { view: "data", kind: "table" } })}
          style={{ display: "flex", alignItems: "center", gap: 6, flex: 1, minWidth: 0, height: "100%" }}
          title={table.comment || table.name}
        >
          <Table2 className="m-icon m-icon--sm" aria-hidden="true" style={{ color: selected ? "var(--primary)" : "var(--muted-foreground)" }} />
          <span className="m-tree__name">{table.name}</span>
          {table.rowEstimate !== undefined && table.rowEstimate >= 0 ? <span className="m-tree__meta">{compact(table.rowEstimate)}</span> : null}
        </button>
      </div>
      {open ? (
        <div className="m-tree__children" role="group">
          {table.columns.map((c) => (
            <div key={c.name} className="m-tree__item m-tree__item--column" role="treeitem" style={{ cursor: "default" }} title={`${c.name} ${c.type}${c.nullable ? "" : " not null"}${c.default ? ` default ${c.default}` : ""}`}>
              <span className="m-tree__chev" aria-hidden="true" />
              {pkCols.has(c.name) ? (
                <KeyRound className="m-icon" aria-label="Primary key" />
              ) : fkCols.has(c.name) ? (
                <Link2 className="m-icon" aria-label="Foreign key" />
              ) : (
                <span style={{ width: 12, flex: "none" }} aria-hidden="true" />
              )}
              <span className="m-tree__name">{c.name}</span>
              <span className="m-tree__meta">{c.type}</span>
            </div>
          ))}
          {(table.indexes?.length ?? 0) > 0 ? (
            <Folder_ id={`${id}/indexes`} label="Indexes" count={table.indexes!.length}>
              {table.indexes!.map((ix) => (
                <div key={ix.name} className="m-tree__item m-tree__item--column" role="treeitem" style={{ cursor: "default" }} title={ix.expression ?? ix.columns?.join(", ")}>
                  <span className="m-tree__chev" aria-hidden="true" />
                  <Hash className="m-icon" aria-hidden="true" />
                  <span className="m-tree__name">{ix.name}</span>
                  <span className="m-tree__meta">{ix.unique ? "unique" : (ix.method ?? "btree")}</span>
                </div>
              ))}
            </Folder_>
          ) : null}
          {(table.foreignKeys?.length ?? 0) > 0 ? (
            <Folder_ id={`${id}/fks`} label="Foreign keys" count={table.foreignKeys!.length}>
              {table.foreignKeys!.map((fk, i) => (
                <div key={fk.name ?? i} className="m-tree__item m-tree__item--column" role="treeitem" style={{ cursor: "default" }} title={fk.name}>
                  <span className="m-tree__chev" aria-hidden="true" />
                  <Link2 className="m-icon" aria-hidden="true" />
                  <span className="m-tree__name">{fk.columns.join(", ")}</span>
                  <span className="m-tree__meta">→ {fk.references.name}</span>
                </div>
              ))}
            </Folder_>
          ) : null}
        </div>
      ) : null}
    </>
  );
}

