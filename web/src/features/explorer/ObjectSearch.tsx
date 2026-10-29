/**
 * The rail's search (⌘P). While the filter has text the rail lists what matches instead of the
 * tree: tables, views, enums, functions and sequences, then columns, ranked fuzzily with the
 * matched letters marked and an icon for each kind. The caret stays in the filter: ↑/↓, Ctrl+N/P
 * or Ctrl+J/K move, Enter opens (a column opens its table's structure; the other kinds show in
 * the tree), Shift+Enter shows any hit in the tree. A query with a dot also matches the
 * qualifier: `public.ord`, `orders.id`.
 *
 * It searches the connection's database and every database open in the tree; the others are a
 * click away so a server with hundreds of databases does not load them all on a keystroke.
 */
import * as React from "react";
import { useQueries } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { Columns3, Eye, FileCode2, ListOrdered, Table2, Tags } from "lucide-react";

import { getGetSchemaQueryOptions, useListDatabases } from "@/lib/api/explore/explore";
import type { Database } from "@/lib/api/mesaAPI.schemas";
import { compact, plural } from "@/lib/format";
import { fuzzyMatch } from "@/lib/fuzzy";
import { cn } from "@/lib/utils";

import { focusMain } from "../workspace/focus";
import { useWorkspace } from "../workspace/context";
import { useTree, type TreeObjectKind } from "./tree.store";

interface Entry {
  kind: TreeObjectKind;
  database: string;
  schema: string;
  name: string;
  /** The table a column belongs to. */
  table?: string;
  /** Whether a column's owner is a table or a view. */
  owner?: "table" | "view";
  /** What `a.b` queries match before the dot: the schema, or the table for a column. */
  qualifier: string;
  /** The muted path shown after the name. */
  path: string;
  meta?: string;
  title?: string;
}

interface Hit extends Entry {
  key: string;
  score: number;
  indices: number[];
}

const MAX_OBJECTS = 100;
const MAX_COLUMNS = 60;

function entriesOf(db: Database, showDatabase: boolean): Entry[] {
  const out: Entry[] = [];
  const flat = db.schemas.length === 1 && db.schemas[0].name === db.name;
  for (const s of db.schemas) {
    const path = flat ? (showDatabase ? db.name : "") : showDatabase ? `${db.name}.${s.name}` : s.name;
    const base = { database: db.name, schema: s.name, qualifier: flat ? db.name : s.name, path };
    for (const t of s.tables) {
      out.push({ ...base, kind: "table", name: t.name, meta: t.rowEstimate !== undefined && t.rowEstimate >= 0 ? compact(t.rowEstimate) : undefined, title: t.comment || undefined });
      for (const c of t.columns) out.push({ ...base, kind: "column", name: c.name, table: t.name, owner: "table", qualifier: t.name, path: t.name, meta: c.type });
    }
    for (const v of s.views ?? []) {
      out.push({ ...base, kind: "view", name: v.name, meta: v.materialized ? "mat." : undefined });
      for (const c of v.columns ?? []) out.push({ ...base, kind: "column", name: c.name, table: v.name, owner: "view", qualifier: v.name, path: v.name, meta: c.type });
    }
    for (const e of s.enums ?? []) out.push({ ...base, kind: "enum", name: e.name, meta: String(e.values.length), title: e.values.join(", ") });
    const seen = new Set<string>();
    for (const f of s.functions ?? []) {
      // Overloads share a name; the tree shows each, the search one.
      if (seen.has(f.name)) continue;
      seen.add(f.name);
      out.push({ ...base, kind: "function", name: f.name, title: `${f.name}(${f.signature ?? ""})${f.returns ? ` → ${f.returns}` : ""}` });
    }
    for (const q of s.sequences ?? []) out.push({ ...base, kind: "sequence", name: q.name });
  }
  return out;
}

function search(entries: Entry[], query: string): { objects: Hit[]; columns: Hit[] } {
  const dotted = query.includes(".");
  const objects: Hit[] = [];
  const columns: Hit[] = [];
  for (const e of entries) {
    let m;
    if (dotted) {
      const full = `${e.qualifier}.${e.name}`;
      m = fuzzyMatch(query, full);
      if (m) m = { score: m.score, indices: m.indices.filter((i) => i > e.qualifier.length).map((i) => i - e.qualifier.length - 1) };
    } else m = fuzzyMatch(query, e.name);
    if (!m) continue;
    const hit = { ...e, key: `${e.kind}:${e.database}.${e.schema}.${e.table ?? ""}.${e.name}`, score: m.score, indices: m.indices };
    (e.kind === "column" ? columns : objects).push(hit);
  }
  const order = (a: Hit, b: Hit) => b.score - a.score || a.name.localeCompare(b.name) || a.path.localeCompare(b.path);
  return { objects: objects.sort(order).slice(0, MAX_OBJECTS), columns: columns.sort(order).slice(0, MAX_COLUMNS) };
}

function KindIcon({ hit }: { hit: Hit }) {
  const cls = "m-icon m-icon--sm";
  switch (hit.kind) {
    case "table":
      return <Table2 className={cls} aria-label="Table" />;
    case "view":
      return <Eye className={cls} aria-label="View" />;
    case "enum":
      return <Tags className={cls} aria-label="Enum" />;
    case "function":
      return <FileCode2 className={cls} aria-label="Function" />;
    case "sequence":
      return <ListOrdered className={cls} aria-label="Sequence" />;
    case "column":
      return <Columns3 className={cls} aria-label="Column" />;
  }
}

function Marked({ text, indices }: { text: string; indices: number[] }) {
  if (indices.length === 0) return <>{text}</>;
  const marked = new Set(indices);
  const parts: React.ReactNode[] = [];
  let run = "";
  let runMarked = false;
  const flush = (i: number) => {
    if (!run) return;
    parts.push(runMarked ? <mark key={i} className="m-search__hit">{run}</mark> : run);
    run = "";
  };
  for (let i = 0; i < text.length; i++) {
    const m = marked.has(i);
    if (m !== runMarked) {
      flush(i);
      runMarked = m;
    }
    run += text[i];
  }
  flush(text.length);
  return <>{parts}</>;
}

export interface ObjectSearchHandle {
  /** Moves the active result by delta, clamped to the list. */
  move: (delta: number) => void;
  /** Opens the active result; `reveal` shows it in the tree instead. False when there is none. */
  open: (reveal: boolean) => boolean;
}

interface Props {
  query: string;
  /** The keyboard's place in the list; the filter owns it so it can point aria-activedescendant at it. */
  active: number;
  onActive: (index: number) => void;
  /** The search is done: clear the filter (the hit was opened or revealed). */
  onDone: () => void;
  /** The listbox id, for the input's aria-controls. */
  id: string;
}

export const ObjectSearch = React.forwardRef<ObjectSearchHandle, Props>(function ObjectSearch({ query, active, onActive, onDone, id }, ref) {
  const { connectionId, connection } = useWorkspace();
  const navigate = useNavigate();
  const reveal = useTree((s) => s.reveal);
  const openNodes = useTree((s) => s.open[connectionId]);
  const dbs = useListDatabases(connectionId, { query: { staleTime: 60_000 } });
  const [everywhere, setEverywhere] = React.useState(false);
  const names = React.useMemo(() => (dbs.data ?? []).map((d) => d.name), [dbs.data]);
  const searched = names.filter((n) => everywhere || n === connection.database || openNodes?.[`db:${n}`]);
  const skipped = names.length - searched.length;
  const schemas = useQueries({ queries: searched.map((db) => getGetSchemaQueryOptions(connectionId, db, undefined, { query: { staleTime: 30_000 } })) });
  const loading = schemas.some((s) => s.isPending);
  // The index is rebuilt when a schema arrives or changes, not on every keystroke.
  const version = schemas.map((s) => s.dataUpdatedAt).join();
  const entries = React.useMemo(() => {
    const ready = schemas.map((s) => s.data).filter((d): d is Database => !!d);
    return ready.flatMap((d) => entriesOf(d, ready.length > 1));
  }, [version]); // eslint-disable-line react-hooks/exhaustive-deps
  const { objects, columns } = React.useMemo(() => search(entries, query), [entries, query]);
  const hits = React.useMemo(() => [...objects, ...columns], [objects, columns]);

  const listRef = React.useRef<HTMLDivElement>(null);
  React.useEffect(() => {
    listRef.current?.querySelector<HTMLElement>(`[data-index="${active}"]`)?.scrollIntoView({ block: "nearest" });
  }, [active]);

  const openHit = React.useCallback(
    (hit: Hit, inTree: boolean) => {
      onDone();
      // Enums, functions and sequences have no view of their own: the tree is where they live.
      if (inTree || hit.kind === "enum" || hit.kind === "function" || hit.kind === "sequence") {
        // A view's columns are not in the tree; show the view.
        if (hit.owner === "view") reveal(connectionId, "view", hit.database, hit.schema, hit.table!);
        else reveal(connectionId, hit.kind, hit.database, hit.schema, hit.name, hit.table);
        return;
      }
      const table = hit.kind === "column" ? hit.table! : hit.name;
      const kind = hit.kind === "view" || hit.owner === "view" ? "view" : "table";
      void navigate({
        to: "/c/$connectionId/db/$database/t/$schema/$table",
        params: { connectionId, database: hit.database, schema: hit.schema, table },
        search: { view: hit.kind === "column" ? "structure" : "data", kind },
      });
      focusMain();
    },
    [connectionId, navigate, onDone, reveal],
  );

  React.useImperativeHandle(
    ref,
    () => ({
      move: (delta) => onActive(Math.max(0, Math.min(hits.length - 1, active + delta))),
      open: (inTree) => {
        const hit = hits[active];
        if (!hit) return false;
        openHit(hit, inTree);
        return true;
      },
    }),
    [hits, active, onActive, openHit],
  );

  const row = (hit: Hit, index: number) => (
    <div
      key={hit.key}
      id={`${id}-${index}`}
      data-index={index}
      role="option"
      aria-selected={index === active}
      className={cn("m-tree__item", hit.kind === "column" && "m-tree__item--column", index === active && "is-active")}
      title={hit.title ?? `${hit.path ? `${hit.path}.` : ""}${hit.name}`}
      // Keep the caret in the filter while the mouse picks a row.
      onMouseDown={(e) => e.preventDefault()}
      onMouseMove={() => index !== active && onActive(index)}
      onClick={(e) => openHit(hit, e.shiftKey)}
    >
      <KindIcon hit={hit} />
      <span className="m-tree__name m-search__name">
        <Marked text={hit.name} indices={hit.indices} />
      </span>
      {hit.path ? <span className="m-search__path">{hit.path}</span> : null}
      {hit.meta ? <span className="m-tree__meta m-search__meta">{hit.meta}</span> : null}
    </div>
  );

  return (
    <div ref={listRef} id={id} className="m-tree m-search" role="listbox" aria-label="Matching objects">
      {objects.map((h, i) => row(h, i))}
      {columns.length > 0 ? <div className="m-tree__group">Columns</div> : null}
      {columns.map((h, i) => row(h, objects.length + i))}
      {hits.length === 0 && !loading ? <div className="m-search__note">No objects match “{query}”.</div> : null}
      {loading ? <div className="m-search__note">Loading schemas…</div> : null}
      {skipped > 0 ? (
        <div className="m-search__note">
          {searched.length === 0 ? "No database is searched yet." : `${plural(skipped, "more database")} not searched.`}{" "}
          <button type="button" className="m-search__more" onMouseDown={(e) => e.preventDefault()} onClick={() => setEverywhere(true)}>
            Search {skipped === 1 ? "it" : `all ${names.length}`}
          </button>
        </div>
      ) : null}
    </div>
  );
});

export type { Hit as ObjectSearchHit };
