/**
 * The nested object tree (`Rail` board): databases → schemas → Tables / Views / Enums /
 * Functions / Sequences → columns and indexes. Each database's schema loads when it opens.
 * The keyboard (arrows or vim keys, Enter, Space, /) is in treeKeys.ts; Tab enters the tree at
 * the selection, and a node revealed from the search scrolls into view with focus.
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
import { treeNode, useNodeOpen, useTree } from "./tree.store";
import { focusFirstItem, onTreeKeyDown } from "./treeKeys";

interface TreeProps {
  onNewDatabase: () => void;
  /** Called for "/" in the tree: focus the search. */
  onSearch: () => void;
  treeRef?: React.RefObject<HTMLDivElement | null>;
}

const ITEM = '[role="treeitem"]';

export function ObjectTree({ onNewDatabase, onSearch, treeRef }: TreeProps) {
  const { connectionId, connection } = useWorkspace();
  const dbs = useListDatabases(connectionId, { query: { staleTime: 60_000 } });
  const list = React.useMemo(() => {
    const all = dbs.data ?? [];
    // The connection's own database first, then the rest by name.
    return [...all].sort((a, b) => (a.name === connection.database ? -1 : b.name === connection.database ? 1 : a.name.localeCompare(b.name)));
  }, [dbs.data, connection.database]);
  const localRef = React.useRef<HTMLDivElement>(null);
  const ref = treeRef ?? localRef;
  const revealing = useTree((s) => (s.revealing?.connectionId === connectionId ? s.revealing : null));

  // Scroll to and focus a revealed node once its branch has rendered (its schema may still load).
  React.useEffect(() => {
    if (!revealing) return;
    let frames = 0;
    let raf = 0;
    const find = () => {
      const el = ref.current?.querySelector<HTMLElement>(`[data-node="${CSS.escape(revealing.node)}"]`);
      if (el) {
        el.focus({ preventScroll: true });
        el.scrollIntoView({ block: "center" });
        // Done: a tree mounted later (the search cleared again) must not take focus back.
        if (useTree.getState().revealing === revealing) useTree.setState({ revealing: null });
      } else if (++frames < 120) raf = requestAnimationFrame(find);
    };
    raf = requestAnimationFrame(find);
    return () => cancelAnimationFrame(raf);
  }, [revealing, ref]);

  // Tab lands on the selected object, else the first item; the items themselves are not tab stops.
  const onFocus = (e: React.FocusEvent<HTMLDivElement>) => {
    if (e.target !== e.currentTarget) return;
    const selected = e.currentTarget.querySelector<HTMLElement>(`${ITEM}.is-selected`);
    if (selected) selected.focus();
    else focusFirstItem(e.currentTarget);
  };

  return (
    <div
      ref={ref}
      className="m-tree"
      role="tree"
      aria-label={`${connection.name} objects`}
      tabIndex={0}
      onKeyDown={(e) => onTreeKeyDown(e, { onSearch })}
      // A button clicks itself on Space's keyup; onKeyDown already pressed it.
      onKeyUp={(e) => e.key === " " && e.preventDefault()}
      onFocus={onFocus}
    >
      {dbs.isPending ? <div className="m-tree__item m-muted">Loading databases…</div> : null}
      {dbs.isError ? <div className="m-tree__item m-help--error" title={errorMessage(dbs.error)}>Cannot list databases</div> : null}
      {list.map((db) => (
        <DatabaseNode key={db.name} database={db.name} size={db.sizeBytes} />
      ))}
      {dbs.isSuccess ? (
        <button type="button" className="m-tree__item m-muted" role="treeitem" tabIndex={-1} onClick={onNewDatabase}>
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

function DatabaseNode({ database, size }: { database: string; size?: number }) {
  const { connectionId } = useWorkspace();
  const id = `db:${database}`;
  const open = useNodeOpen(connectionId, id);
  const toggle = useTree((s) => s.toggle);
  return (
    <>
      <button type="button" className="m-tree__item" role="treeitem" tabIndex={-1} data-node={id} aria-expanded={open} onClick={() => toggle(connectionId, id)} title={size ? `${database} · ${compact(size)}B` : database}>
        <Chevron open={open} />
        <DatabaseIcon className="m-icon m-icon--sm" aria-hidden="true" />
        <span className="m-tree__name">{database}</span>
      </button>
      {open ? (
        <div className="m-tree__children" role="group">
          <DatabaseChildren database={database} />
        </div>
      ) : null}
    </>
  );
}

function DatabaseChildren({ database }: { database: string }) {
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
  const schemas = schema.data.schemas.filter((s) => s.tables.length + (s.views?.length ?? 0) + (s.enums?.length ?? 0) + (s.functions?.length ?? 0) > 0 || s.name === "public" || s.name === database);
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
        <SchemaNode key={s.name} database={database} schema={s} single={schemas.length === 1} />
      ))}
    </>
  );
}

function SchemaNode({ database, schema, single }: { database: string; schema: Schema; single: boolean }) {
  const { connectionId } = useWorkspace();
  const id = `schema:${database}.${schema.name}`;
  const stored = useNodeOpen(connectionId, id);
  const closed = useNodeOpen(connectionId, `closed:${id}`);
  const toggle = useTree((s) => s.toggle);
  const tables = schema.tables;
  const views = schema.views ?? [];
  // A database with one schema shows it open unless the person closed it.
  const open = single ? !closed : stored;
  const onToggle = () => {
    if (single) toggle(connectionId, `closed:${id}`, open);
    else toggle(connectionId, id);
  };
  // Engines without schemas (MySQL) report one schema named after the
  // database; the tree skips that level.
  const flat = single && schema.name === database;
  const Group = flat ? React.Fragment : TreeGroup;
  const node = (kind: "enum" | "function" | "sequence", name: string) => treeNode(kind, database, schema.name, name);
  return (
    <>
      {flat ? null : (
        <button type="button" className="m-tree__item" role="treeitem" tabIndex={-1} data-node={id} aria-expanded={open} onClick={onToggle}>
          <Chevron open={open} />
          <Folder className="m-icon m-icon--sm" aria-hidden="true" />
          <span className="m-tree__name">{schema.name}</span>
        </button>
      )}
      {open || flat ? (
        <Group>
          <Folder_ id={`${id}/tables`} label="Tables" count={tables.length} defaultOpen>
            {tables.length === 0 ? <Leaf label="No tables" muted /> : tables.map((t) => <TableNode key={t.name} database={database} schema={schema.name} table={t} />)}
          </Folder_>
          {views.length > 0 ? (
            <Folder_ id={`${id}/views`} label="Views" count={views.length}>
              {views.map((v) => (
                <ObjectLeaf key={v.name} icon={<Eye className="m-icon m-icon--sm" aria-hidden="true" />} label={v.name} meta={v.materialized ? "mat." : undefined} database={database} schema={schema.name} table={v.name} kind="view" />
              ))}
            </Folder_>
          ) : null}
          {(schema.enums?.length ?? 0) > 0 ? (
            <Folder_ id={`${id}/enums`} label="Enums" count={schema.enums!.length}>
              {schema.enums!.map((e) => (
                <Leaf key={e.name} node={node("enum", e.name)} icon={<Tags className="m-icon m-icon--sm" aria-hidden="true" />} label={e.name} meta={String(e.values.length)} title={e.values.join(", ")} />
              ))}
            </Folder_>
          ) : null}
          {(schema.functions?.length ?? 0) > 0 ? (
            <Folder_ id={`${id}/functions`} label="Functions" count={schema.functions!.length}>
              {schema.functions!.map((f, i) => (
                <Leaf
                  key={`${f.name}${i}`}
                  node={schema.functions!.findIndex((g) => g.name === f.name) === i ? node("function", f.name) : undefined}
                  icon={<FileCode2 className="m-icon m-icon--sm" aria-hidden="true" />}
                  label={`${f.name}(${f.signature ?? ""})`}
                  title={f.returns ? `returns ${f.returns}` : undefined}
                />
              ))}
            </Folder_>
          ) : null}
          {(schema.sequences?.length ?? 0) > 0 ? (
            <Folder_ id={`${id}/sequences`} label="Sequences" count={schema.sequences!.length}>
              {schema.sequences!.map((s) => (
                <Leaf key={s.name} node={node("sequence", s.name)} icon={<ListOrdered className="m-icon m-icon--sm" aria-hidden="true" />} label={s.name} />
              ))}
            </Folder_>
          ) : null}
        </Group>
      ) : null}
    </>
  );
}

function TreeGroup({ children }: { children: React.ReactNode }) {
  return (
    <div className="m-tree__children" role="group">
      {children}
    </div>
  );
}

function Folder_({ id, label, count, children, defaultOpen }: { id: string; label: string; count: number; children: React.ReactNode; defaultOpen?: boolean }) {
  const { connectionId } = useWorkspace();
  const toggle = useTree((s) => s.toggle);
  const closedKey = `closed:${id}`;
  const stored = useNodeOpen(connectionId, id);
  const closed = useNodeOpen(connectionId, closedKey);
  const open = defaultOpen ? !closed : stored;
  const onClick = () => {
    if (defaultOpen) toggle(connectionId, closedKey, open);
    else toggle(connectionId, id);
  };
  return (
    <>
      <button type="button" className="m-tree__item m-tree__item--folder" role="treeitem" tabIndex={-1} aria-expanded={open} onClick={onClick}>
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

function Leaf({ node, icon, label, meta, title, muted }: { node?: string; icon?: React.ReactNode; label: string; meta?: string; title?: string; muted?: boolean }) {
  return (
    <div className={cn("m-tree__item", muted && "m-muted")} role="treeitem" tabIndex={-1} data-node={node} style={{ cursor: "default" }} title={title ?? label}>
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
      tabIndex={-1}
      data-node={treeNode(kind, database, schema, table)}
      data-open=""
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
      <div className={cn("m-tree__item", selected && "is-selected")} role="treeitem" tabIndex={-1} data-node={id} aria-expanded={open} style={{ paddingLeft: 0 }}>
        <button type="button" tabIndex={-1} data-toggle="" aria-label={open ? `Collapse ${table.name}` : `Expand ${table.name}`} onClick={() => toggle(connectionId, id)} style={{ display: "grid", placeItems: "center", width: 22, height: "100%" }}>
          <Chevron open={open} />
        </button>
        <button
          type="button"
          tabIndex={-1}
          data-open=""
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
            <div
              key={c.name}
              className="m-tree__item m-tree__item--column"
              role="treeitem"
              tabIndex={-1}
              data-node={treeNode("column", database, schema, c.name, table.name)}
              style={{ cursor: "default" }}
              title={`${c.name} ${c.type}${c.nullable ? "" : " not null"}${c.default ? ` default ${c.default}` : ""}`}
            >
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
                <div key={ix.name} className="m-tree__item m-tree__item--column" role="treeitem" tabIndex={-1} style={{ cursor: "default" }} title={ix.expression ?? ix.columns?.join(", ")}>
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
                <div key={fk.name ?? i} className="m-tree__item m-tree__item--column" role="treeitem" tabIndex={-1} style={{ cursor: "default" }} title={fk.name}>
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
