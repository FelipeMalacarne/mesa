/**
 * The schema designer (`Diagram`, `DiagramNew`, `DiagramApply` boards). An existing database
 * opens drawn from its live schema; a new one opens empty. The DBML and the diagram stay in
 * sync: typing parses into the model, UI edits re-print the DBML. The diagram is drawn against
 * the live schema, so it shows the migration; Review plans the diff against the live database
 * and nothing runs until it is applied. Edits, moves and layouts undo with ⌘Z.
 */
import * as React from "react";
import { keepPreviousData, useQuery, useQueryClient } from "@tanstack/react-query";
import { useNavigate } from "@tanstack/react-router";
import { ChevronDown, Database as DatabaseIcon, Download, FileUp, Focus, LayoutGrid, Link2, Network, Plus, Redo2, RefreshCw, Rows3, Table2, Undo2, Workflow } from "lucide-react";
import { toast } from "sonner";

import { applyChanges } from "@/lib/api/changes/changes";
import { diffDesignerModel, getGetDesignerModelQueryKey, parseDbml, printDbml, saveDesignerLayout, useGetDesignerModel } from "@/lib/api/designer/designer";
import { getGetSchemaQueryKey, useListDatabases } from "@/lib/api/explore/explore";
import type { Database, Problem, SchemaChange } from "@/lib/api/mesaAPI.schemas";
import { errorMessage } from "@/lib/api/fetch";
import { plural } from "@/lib/format";
import { cn, isCmd } from "@/lib/utils";
import { usePendingStatus, useStatusFacts } from "@/app/status.store";
import { label, shortcut, typing } from "@/app/keys";
import { Button } from "@/ui/Button";
import { EmptyState, Kbd, Select } from "@/ui/controls";
import { Popover, PopoverContent, PopoverTrigger } from "@/ui/primitives/popover";
import { describeChange, ReviewDialog } from "@/features/changes/ReviewDialog";
import { useTabs } from "@/features/explorer/tabs.store";
import { Editor } from "@/features/sql/Editor";

import { useWorkspace } from "../workspace/context";
import { AddRelationDialog, ImportDialog } from "./dialogs";
import { Erd, type ErdHandle, type RelationsMode, type WheelMode } from "./Erd";
import { diagramOf, edgeId, neighbours } from "./graph";
import { History } from "./history";
import { useDebounced, usePref } from "./hooks";
import { RelationInspector, TableInspector } from "./Inspector";
import { TableDialog } from "./TableDialog";
import { freeSpot, layout, placeMissing, type LayoutKind } from "./layout";
import { addForeignKey, addTable, emptyModel, findTable, fkId, NODE_WIDTH, relate, removeForeignKey, removeTable, renameTable, tableKey, tablesOf, type Pos } from "./model";

type View = "split" | "diagram" | "code";

interface Snapshot {
  model: Database;
  positions: Record<string, Pos>;
}

interface FocusState {
  keys: Set<string>;
  /** Where the focused tables sit while focused; the saved layout is left alone. */
  positions: Record<string, Pos>;
  around: string[];
}

/** The model plus the live tables it drops, so layouts leave room for their ghosts. */
function withGhosts(model: Database, live: Database | undefined): Database {
  let out = model;
  for (const t of tablesOf(live)) if (!findTable(model, t.key)) out = addTable(out, t.schema, t.table);
  return out;
}

/** The line of `Table schema.name {` (or without the schema) in DBML, 1-based. */
function tableLine(dbml: string, schema: string, name: string): number | null {
  const q = (s: string) => `"?${s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}"?`;
  const re = new RegExp(`^\\s*Table\\s+(?:${q(schema)}\\.)?${q(name)}(?:\\s|\\{|$)`, "i");
  const i = dbml.split("\n").findIndex((l) => re.test(l));
  return i < 0 ? null : i + 1;
}

const LAYOUTS: { kind: LayoutKind; title: string; help: string; icon: React.ReactNode }[] = [
  { kind: "hierarchy", title: "Hierarchy", help: "Referenced tables left, the tables that use them to the right", icon: <Workflow className="m-icon m-icon--sm" aria-hidden="true" /> },
  { kind: "clusters", title: "Clusters", help: "Related tables pulled together; busy hubs in the middle", icon: <Network className="m-icon m-icon--sm" aria-hidden="true" /> },
  { kind: "grid", title: "Grid", help: "Alphabetical columns, relations ignored", icon: <Rows3 className="m-icon m-icon--sm" aria-hidden="true" /> },
];

export function DesignerPage({ database: requested, fresh }: { database?: string; fresh: boolean }) {
  const { connection } = useWorkspace();
  const database = requested ?? connection.database;
  return <Designer key={database} database={database} fresh={fresh} />;
}

function Designer({ database, fresh }: { database: string; fresh: boolean }) {
  const { connectionId, connection } = useWorkspace();
  const navigate = useNavigate();
  const qc = useQueryClient();
  const dbs = useListDatabases(connectionId);
  const live = useGetDesignerModel(connectionId, database, { query: { retry: false, staleTime: Infinity } });

  const [view, setView] = React.useState<View>("split");
  const [split, setSplit] = React.useState(() => {
    try {
      return Math.min(70, Math.max(20, Number(localStorage.getItem("mesa.designer.split")) || 36));
    } catch {
      return 36;
    }
  });
  const [dbml, setDbml] = React.useState<string | null>(null);
  const [model, setModel] = React.useState<Database | null>(null);
  const [problems, setProblems] = React.useState<Problem[]>([]);
  const [positions, setPositions] = React.useState<Record<string, Pos>>({});
  const [selected, setSelected] = React.useState<Set<string>>(() => new Set());
  const [selectedEdge, setSelectedEdge] = React.useState<string | null>(null);
  const [focus, setFocus] = React.useState<FocusState | null>(null);
  const [dialog, setDialog] = React.useState<"table" | "relation" | "import" | null>(null);
  const [addAt, setAddAt] = React.useState<Pos | null>(null);
  const [review, setReview] = React.useState(false);
  const [relations, setRelations] = usePref<RelationsMode>("mesa.designer.relations", "all", ["all", "selected"]);
  const [wheel, setWheel] = usePref<WheelMode>("mesa.designer.wheel", "zoom", ["zoom", "pan"]);
  const parseSeq = React.useRef(0);
  const printSeq = React.useRef(0);
  const erd = React.useRef<ErdHandle>(null);
  const textarea = React.useRef<HTMLTextAreaElement>(null);
  const history = React.useRef(new History<Snapshot>());
  // The DBML each model came from or was printed as, so undo restores text and model together.
  const texts = React.useRef(new WeakMap<Database, string>());
  const lastPush = React.useRef<{ key?: string; at: number }>({ at: 0 });
  const [, setHistoryVersion] = React.useState(0);

  React.useEffect(() => {
    useTabs.getState().open(connectionId, { id: `designer:${database}`, kind: "designer", title: `Designer · ${database}`, database });
  }, [connectionId, database]);

  // Start from the live model whenever it (re)loads; local positions win over saved ones.
  const loadedAt = live.dataUpdatedAt;
  React.useEffect(() => {
    const data = live.data;
    if (!data) return;
    const empty = tablesOf(data.schema).length === 0;
    const text = empty ? `// ${database} · no tables yet\n// Write tables here, or add them on the diagram.\n` : data.dbml;
    texts.current.set(data.schema, text);
    setModel(data.schema);
    setDbml(text);
    setProblems(data.problems);
    setPositions((p) => placeMissing(data.schema, { ...data.layout.positions, ...p }));
    history.current.clear();
    setHistoryVersion((v) => v + 1);
  }, [loadedAt, database]); // eslint-disable-line react-hooks/exhaustive-deps

  const liveSchema = live.data?.schema;
  const current = model ?? liveSchema ?? emptyModel(database);

  // What would change, planned by the server once typing pauses (also the status bar count).
  const settled = useDebounced(current, 400);
  const settledJson = React.useMemo(() => JSON.stringify(settled), [settled]);
  const diff = useQuery({
    queryKey: ["designer-diff", connectionId, database, settledJson],
    queryFn: () => diffDesignerModel(connectionId, database, { schema: settled }),
    enabled: !!live.data && !!model,
    retry: false,
    staleTime: 0,
    placeholderData: keepPreviousData,
  });
  const changes = diff.data?.changes ?? [];
  const changeCount = changes.length;
  // The server could not plan the edit (an engine refusing a change): say so, never "matches".
  const planError = diff.isError && !diff.isFetching ? errorMessage(diff.error) : null;
  const errorsCount = problems.filter((p) => p.level === "error").length;
  const diagram = React.useMemo(() => diagramOf(current, liveSchema, diff.data?.changes), [current, liveSchema, diff.data?.changes]);

  // Refs for handlers that outlive a render (keys, debounced saves).
  const state = React.useRef({ current, positions, focus, selected, selectedEdge, diagram });
  state.current = { current, positions, focus, selected, selectedEdge, diagram };

  const snapshot = (): Snapshot => ({ model: state.current.current, positions: state.current.positions });
  /** Records the state before a change; changes with the same key close together are one step. */
  const record = (coalesce?: string) => {
    const now = Date.now();
    const same = coalesce && lastPush.current.key === coalesce && now - lastPush.current.at < 1500;
    lastPush.current = { key: coalesce, at: now };
    if (same) return;
    history.current.push(snapshot());
    setHistoryVersion((v) => v + 1);
  };

  useStatusFacts([`${plural(tablesOf(current).length, "table")}`, ...(errorsCount ? [plural(errorsCount, "DBML error")] : [])]);
  usePendingStatus(changeCount > 0 && errorsCount === 0 ? { summary: plural(changeCount, "change"), onDiscard: () => reset(), onReview: () => setReview(true) } : null);

  const saveLayout = React.useRef<number | null>(null);
  const persistLayout = (pos: Record<string, Pos>) => {
    if (saveLayout.current) window.clearTimeout(saveLayout.current);
    saveLayout.current = window.setTimeout(() => {
      // Only tables that exist or are drawn; positions of tables long gone are not kept.
      const keep = new Set([...tablesOf(state.current.current), ...tablesOf(liveSchema)].map((t) => t.key));
      const out = Object.fromEntries(Object.entries(pos).filter(([k]) => keep.has(k)));
      void saveDesignerLayout(connectionId, database, { positions: out }).catch(() => undefined);
    }, 600);
  };

  const reset = () => {
    if (!live.data) return;
    record();
    setModel(live.data.schema);
    setDbml(live.data.dbml);
    setProblems(live.data.problems);
    setSelectedEdge(null);
  };

  /**
   * Parses DBML after a short pause; the diagram follows the last text that parsed. Restoring
   * a snapshot only checks it: the model came with the snapshot.
   */
  const parse = (text: string, mode: "edit" | "check") => {
    const seq = ++parseSeq.current;
    window.setTimeout(async () => {
      if (seq !== parseSeq.current) return;
      try {
        const r = await parseDbml({ dbml: text });
        if (seq !== parseSeq.current) return;
        setProblems(r.problems);
        if (r.schema && mode === "edit") {
          const next = { ...r.schema, name: database };
          texts.current.set(next, text);
          const before = state.current.current;
          record("dbml");
          setModel(next);
          // One new table gone and one new: a rename, so the card stays where it was. (A table in
          // the database stays as a ghost where it is: renaming it drops and creates.)
          const gone = tablesOf(before).filter((t) => !findTable(next, t.key) && !(liveSchema && findTable(liveSchema, t.key)));
          const added = tablesOf(next).filter((t) => !findTable(before, t.key));
          const renamed = gone.length === 1 && added.length === 1 ? { from: gone[0]!.key, to: added[0]!.key } : null;
          setPositions((p) => placeMissing(withGhosts(next, liveSchema), renamed && p[renamed.from] ? { ...p, [renamed.to]: p[renamed.from]! } : p, erd.current?.center()));
          if (renamed && state.current.selected.has(renamed.from)) setSelected(new Set([renamed.to]));
        }
      } catch (e) {
        setProblems([{ level: "error", message: errorMessage(e) }]);
      }
    }, 350);
  };

  const onType = (text: string) => {
    setDbml(text);
    parse(text, "edit");
  };

  /** UI edits: change the model, then print it so the DBML matches. */
  const applyModel = async (next: Database, coalesce?: string) => {
    record(coalesce);
    setModel(next);
    setPositions((p) => placeMissing(withGhosts(next, liveSchema), p, addAt ?? erd.current?.center()));
    parseSeq.current++;
    const seq = ++printSeq.current;
    try {
      const r = await printDbml({ schema: next });
      texts.current.set(next, r.dbml);
      if (seq !== printSeq.current) return;
      setDbml(r.dbml);
      setProblems([]);
    } catch (e) {
      toast.error(errorMessage(e));
    }
  };

  const restore = async (s: Snapshot) => {
    parseSeq.current++;
    const seq = ++printSeq.current;
    setModel(s.model);
    setPositions(s.positions);
    persistLayout(s.positions);
    setHistoryVersion((v) => v + 1);
    const text = texts.current.get(s.model);
    if (text !== undefined) {
      setDbml(text);
      parse(text, "check");
      return;
    }
    try {
      const r = await printDbml({ schema: s.model });
      texts.current.set(s.model, r.dbml);
      if (seq !== printSeq.current) return;
      setDbml(r.dbml);
      setProblems([]);
    } catch (e) {
      toast.error(errorMessage(e));
    }
  };
  const undo = () => {
    const s = history.current.undo(snapshot());
    if (s) void restore(s);
  };
  const redo = () => {
    const s = history.current.redo(snapshot());
    if (s) void restore(s);
  };

  // Moving cards: in focus mode the focus positions, otherwise the saved layout.
  const onMove = (moves: Record<string, Pos>) => {
    if (state.current.focus) setFocus((f) => (f ? { ...f, positions: { ...f.positions, ...moves } } : f));
    else setPositions((p) => ({ ...p, ...moves }));
  };
  const onMoveEnd = () => {
    if (!state.current.focus) persistLayout(state.current.positions);
  };

  const arrange = (kind: LayoutKind) => {
    const drawn = withGhosts(current, liveSchema);
    if (focus) {
      setFocus({ ...focus, positions: layout(drawn, kind, focus.keys) });
      requestAnimationFrame(() => erd.current?.fit(focus.keys));
      return;
    }
    record();
    const pos = layout(drawn, kind);
    setPositions(pos);
    persistLayout(pos);
    requestAnimationFrame(() => erd.current?.fit());
  };

  const enterFocus = (around: Iterable<string>) => {
    const start = [...around];
    if (start.length === 0) return;
    const keys = neighbours(diagram, start);
    setFocus({ keys, positions: layout(withGhosts(current, liveSchema), "hierarchy", keys), around: start });
    requestAnimationFrame(() => erd.current?.fit(keys));
  };
  const exitFocus = () => {
    setFocus(null);
    requestAnimationFrame(() => (state.current.selected.size ? erd.current?.fit(state.current.selected) : erd.current?.fit()));
  };

  const select = (keys: Set<string>) => {
    setSelected(keys);
    setSelectedEdge(null);
  };
  const selectEdge = (id: string) => {
    setSelected(new Set());
    setSelectedEdge(id);
  };
  // A table to bring into view once it has a place (a new one gets it a render later).
  const pendingReveal = React.useRef<string | null>(null);
  const drawnPositions = focus ? focus.positions : positions;
  React.useEffect(() => {
    const key = pendingReveal.current;
    if (!key || !drawnPositions[key]) return;
    pendingReveal.current = null;
    requestAnimationFrame(() => erd.current?.reveal(key));
  }, [drawnPositions]);
  const showTable = (key: string) => {
    if (focus && !focus.keys.has(key)) setFocus(null);
    select(new Set([key]));
    pendingReveal.current = key;
    requestAnimationFrame(() => erd.current?.reveal(key));
  };

  // A table selected on the diagram scrolls the DBML to it (once its DBML is there).
  const one = selected.size === 1 ? [...selected][0]! : null;
  const scrolledTo = React.useRef<string | null>(null);
  React.useEffect(() => {
    const ta = textarea.current;
    const t = one ? diagram.byKey.get(one) : undefined;
    if (!one) scrolledTo.current = null;
    if (!ta || !t || !dbml || scrolledTo.current === one) return;
    const line = tableLine(dbml, t.schema, t.table.name);
    if (!line) return;
    scrolledTo.current = one;
    ta.scrollTop = Math.max(0, (line - 2) * 22);
  }, [one, dbml]); // eslint-disable-line react-hooks/exhaustive-deps

  const deleteSelection = () => {
    const s = state.current;
    if (s.selectedEdge) {
      const e = s.diagram.edges.find((x) => x.id === s.selectedEdge);
      if (e && e.state !== "dropped") void applyModel(removeForeignKey(s.current, e.from, fkId(e.fk)));
      setSelectedEdge(null);
      return;
    }
    const keys = [...s.selected].filter((k) => findTable(s.current, k));
    if (keys.length === 0) return;
    let m = s.current;
    for (const k of keys) m = removeTable(m, k);
    void applyModel(m);
    setSelected(new Set());
  };

  const nudge = (dx: number, dy: number) => {
    const s = state.current;
    const keys = [...s.selected].filter((k) => (s.focus ? s.focus.positions[k] : s.positions[k]));
    if (keys.length === 0) return;
    const base = s.focus ? s.focus.positions : s.positions;
    if (!s.focus) record("nudge");
    onMove(Object.fromEntries(keys.map((k) => [k, { x: Math.max(0, base[k]!.x + dx), y: Math.max(0, base[k]!.y + dy) }])));
    if (!s.focus) persistLayout({ ...base, ...Object.fromEntries(keys.map((k) => [k, { x: Math.max(0, base[k]!.x + dx), y: Math.max(0, base[k]!.y + dy) }])) });
  };

  const onKey = React.useRef<(e: KeyboardEvent) => void>(() => undefined);
  onKey.current = (e: KeyboardEvent) => {
    if (isCmd(e) && e.code === "KeyS" && !e.shiftKey && !e.altKey) {
      e.preventDefault();
      if (changeCount > 0 && errorsCount === 0) setReview(true);
      return;
    }
    if (e.defaultPrevented || typing(e.target) || document.querySelector(".m-overlay")) return;
    const mod = isCmd(e);
    if (mod && e.code === "KeyZ") {
      e.preventDefault();
      if (e.shiftKey) redo();
      else undo();
      return;
    }
    if (mod && e.code === "KeyY") {
      e.preventDefault();
      redo();
      return;
    }
    if (view === "code") return;
    const plain = !mod && !e.altKey;
    const run = (f: () => void) => {
      e.preventDefault();
      f();
    };
    if ((mod && e.code === "KeyF") || (plain && e.key === "/")) return run(() => erd.current?.find());
    if (mod && e.code === "KeyA") return run(() => select(new Set(diagram.tables.filter((t) => !focus || focus.keys.has(t.key)).map((t) => t.key))));
    if (!plain) return;
    if (e.code === "Equal" || e.code === "NumpadAdd") return run(() => erd.current?.zoom(1));
    if (e.code === "Minus" || e.code === "NumpadSubtract") return run(() => erd.current?.zoom(-1));
    if (e.shiftKey && e.code === "Digit0") return run(() => erd.current?.zoomTo(1));
    if (e.shiftKey && e.code === "Digit1") return run(() => erd.current?.fit(focus ? focus.keys : undefined));
    if (e.shiftKey && e.code === "Digit2") return run(() => selected.size && erd.current?.fit(selected));
    if (e.code === "KeyF" && !e.shiftKey) return run(() => (focus ? exitFocus() : enterFocus(selected)));
    if (e.code === "Delete" || e.code === "Backspace") return run(deleteSelection);
    if (e.code === "Escape") {
      if (focus) return run(exitFocus);
      if (selected.size || selectedEdge) return run(() => select(new Set()));
      return;
    }
    const step = e.shiftKey ? 40 : 8;
    if (e.code === "ArrowLeft") return run(() => nudge(-step, 0));
    if (e.code === "ArrowRight") return run(() => nudge(step, 0));
    if (e.code === "ArrowUp") return run(() => nudge(0, -step));
    if (e.code === "ArrowDown") return run(() => nudge(0, step));
  };
  React.useEffect(() => {
    const handle = (e: KeyboardEvent) => onKey.current(e);
    window.addEventListener("keydown", handle);
    return () => window.removeEventListener("keydown", handle);
  }, []);

  const exportDbml = () => {
    const url = URL.createObjectURL(new Blob([dbml ?? ""], { type: "text/plain" }));
    const a = document.createElement("a");
    a.href = url;
    a.download = `${database}.dbml`;
    a.click();
    setTimeout(() => URL.revokeObjectURL(url), 1000);
  };

  if (live.isError) {
    return (
      <div style={{ padding: 24 }}>
        <EmptyState icon={<DatabaseIcon className="m-icon" aria-hidden="true" />} title={`Cannot open ${database}`} description={errorMessage(live.error)} />
      </div>
    );
  }

  const markLines = new Set(problems.filter((p) => p.level === "error" && p.line).map((p) => p.line!));
  const selTable = one ? diagram.byKey.get(one) : undefined;
  const selEdge = selectedEdge ? diagram.edges.find((e) => e.id === selectedEdge) : undefined;
  const tableCount = tablesOf(current).length;
  const emptyState = (
    <div className="m-empty" style={{ width: 420, background: "var(--card)" }}>
      <div className="m-empty__media">
        <LayoutGrid className="m-icon" aria-hidden="true" />
      </div>
      <p className="m-empty__title">{database} has no tables yet</p>
      <p className="m-empty__desc">Add a table here or write DBML on the left. Both stay in sync, and nothing runs until you review the SQL.</p>
      <div className="m-empty__actions">
        <Button variant="primary" size="sm" onClick={() => setDialog("table")}>
          <Plus className="m-icon m-icon--sm" aria-hidden="true" />
          Add table
        </Button>
        <Button variant="outline" size="sm" onClick={() => setDialog("import")}>
          <FileUp className="m-icon m-icon--sm" aria-hidden="true" />
          Import DBML
        </Button>
      </div>
    </div>
  );
  const focusNames = focus ? focus.around.map((k) => diagram.byKey.get(k)?.table.name ?? k) : [];
  const banner = focus ? (
    <>
      <Focus className="m-icon m-icon--sm" aria-hidden="true" />
      <span>
        <span className="m-mono">{focusNames.slice(0, 3).join(", ")}</span>
        {focusNames.length > 3 ? ` and ${focusNames.length - 3} more` : ""} with {plural(focus.keys.size - focus.around.length, "related table")}
      </span>
      <Button variant="ghost" size="xs" onClick={exitFocus}>
        Show all <Kbd>Esc</Kbd>
      </Button>
    </>
  ) : null;

  return (
    <>
      <div className="m-toolbar m-erd-toolbar" style={{ height: "var(--size-toolbar)", flex: "none" }}>
        <div className="m-seg" role="tablist" aria-label="Designer view">
          {(["split", "diagram", "code"] as View[]).map((v) => (
            <button key={v} type="button" role="tab" aria-selected={view === v} className={cn("m-seg__item", view === v && "is-active")} onClick={() => setView(v)}>
              {v === "split" ? "Split" : v === "diagram" ? "Diagram" : "Code"}
            </button>
          ))}
        </div>
        <span className="m-sep" style={{ height: 20, alignSelf: "center" }} />
        <span style={{ display: "inline-flex", alignItems: "center", gap: 6 }}>
          <DatabaseIcon className="m-icon m-icon--sm m-muted" aria-hidden="true" />
          <Select size="sm" mono aria-label="Database" value={database} options={(dbs.data ?? [{ name: database }]).map((d) => ({ value: d.name, label: d.name }))} onChange={(v) => void navigate({ to: "/c/$connectionId/designer", params: { connectionId }, search: { database: v } })} />
        </span>
        <span className="m-sep" style={{ height: 20, alignSelf: "center" }} />
        <Button variant="ghost" size="sm" onClick={() => setDialog("table")} title="New table (or double-click the canvas)">
          <Table2 className="m-icon m-icon--sm" aria-hidden="true" />
          Table
        </Button>
        <Button variant="ghost" size="sm" onClick={() => setDialog("relation")} disabled={tableCount === 0} title="New relation (or drag a column's dot onto another table)">
          <Link2 className="m-icon m-icon--sm" aria-hidden="true" />
          Relation
        </Button>
        {view !== "code" ? (
          <>
            <ArrangeMenu focused={focus?.keys.size ?? 0} disabled={tableCount === 0} onArrange={arrange} />
            <span className="m-sep" style={{ height: 20, alignSelf: "center" }} />
            <div className="m-seg" role="radiogroup" aria-label="Relations shown">
              <button type="button" role="radio" aria-checked={relations === "all"} className={cn("m-seg__item", relations === "all" && "is-active")} onClick={() => setRelations("all")} title="Draw every relation">
                All relations
              </button>
              <button type="button" role="radio" aria-checked={relations === "selected"} className={cn("m-seg__item", relations === "selected" && "is-active")} onClick={() => setRelations("selected")} title="Draw only the relations of the selected or hovered tables">
                Selected
              </button>
            </div>
            <Button variant={focus ? "secondary" : "ghost"} size="sm" aria-pressed={!!focus} disabled={!focus && selected.size === 0} onClick={() => (focus ? exitFocus() : enterFocus(selected))} title={`Show only the selected tables and their neighbours (${label({ code: "KeyF" })})`}>
              <Focus className="m-icon m-icon--sm" aria-hidden="true" />
              Focus
            </Button>
          </>
        ) : null}
        <span className="m-spacer" />
        <Button variant="ghost" size="sm" icon aria-label="Undo" title={`Undo (${shortcut("KeyZ")})`} disabled={!history.current.canUndo} onClick={undo}>
          <Undo2 className="m-icon m-icon--sm" aria-hidden="true" />
        </Button>
        <Button variant="ghost" size="sm" icon aria-label="Redo" title={`Redo (${label({ code: "KeyZ", mod: true, shift: true })})`} disabled={!history.current.canRedo} onClick={redo}>
          <Redo2 className="m-icon m-icon--sm" aria-hidden="true" />
        </Button>
        <Button
          variant="ghost"
          size="sm"
          icon
          aria-label="Reload the live schema"
          title="Reload the live schema"
          onClick={async () => {
            if (changeCount > 0 && !window.confirm("Reload the live schema and drop the edits that are not applied?")) return;
            await qc.invalidateQueries({ queryKey: getGetDesignerModelQueryKey(connectionId, database) });
          }}
        >
          <RefreshCw className="m-icon m-icon--sm" aria-hidden="true" />
        </Button>
        <Button variant="ghost" size="sm" icon aria-label="Import DBML" title="Import DBML" onClick={() => setDialog("import")}>
          <FileUp className="m-icon m-icon--sm" aria-hidden="true" />
        </Button>
        <Button variant="ghost" size="sm" icon aria-label="Export DBML" title="Export DBML" onClick={exportDbml} disabled={!dbml}>
          <Download className="m-icon m-icon--sm" aria-hidden="true" />
        </Button>
        <ChangesMenu changes={changes} error={planError} pending={diff.isFetching} onShow={(c) => c.table && showTable(tableKey(c.table.schema, c.table.name))} onDiscard={reset} />
        <Button variant="primary" size="sm" onClick={() => setReview(true)} disabled={changeCount === 0 || errorsCount > 0} title={errorsCount > 0 ? "Fix the DBML errors first" : planError ?? `Review & apply (${shortcut("KeyS")})`}>
          Review &amp; apply <Kbd>{shortcut("KeyS")}</Kbd>
        </Button>
      </div>
      <div style={{ flex: 1, minHeight: 0, display: "flex" }}>
        {view !== "diagram" ? (
          <section aria-label="DBML source" style={{ flex: view === "code" ? 1 : `0 0 ${split}%`, minWidth: 0, display: "flex", flexDirection: "column", background: "var(--background)" }}>
            <div style={{ display: "flex", alignItems: "center", gap: 8, height: 36, flex: "none", padding: "0 12px", borderBottom: "1px solid var(--border-subtle)" }}>
              <span className="m-mono" style={{ fontSize: 12 }}>
                {database}
              </span>
              <span className="m-badge m-badge--mono">DBML</span>
              <span className="m-spacer" />
              {errorsCount > 0 ? (
                <span className="m-status m-help" style={{ color: "var(--destructive)", minWidth: 0, overflow: "hidden", textOverflow: "ellipsis" }}>
                  <span className="m-dot m-dot--down" />
                  {problems.find((p) => p.level === "error")?.message}
                </span>
              ) : planError ? (
                <span className="m-status m-help" style={{ color: "var(--destructive)", minWidth: 0, overflow: "hidden", textOverflow: "ellipsis" }} title={planError}>
                  <span className="m-dot m-dot--down" />
                  Cannot plan: {planError}
                </span>
              ) : changeCount > 0 ? (
                <span className="m-status m-help">
                  <span className="m-dot m-dot--pending" />
                  {plural(changeCount, "change")} not applied
                </span>
              ) : (
                <span className="m-status m-help">
                  <span className="m-dot m-dot--online" />
                  {fresh && tableCount === 0 ? "New database" : "Matches the live schema"}
                </span>
              )}
            </div>
            <div style={{ flex: 1, minHeight: 0 }}>{dbml !== null ? <Editor value={dbml} onChange={onType} language="dbml" ariaLabel="DBML" markLines={markLines} textareaRef={textarea} /> : <p className="m-help" style={{ padding: 12 }}>Loading the live schema…</p>}</div>
            {problems.filter((p) => p.level === "warning").length > 0 ? (
              <details className="m-erd-warnings">
                <summary className="m-help">{plural(problems.filter((p) => p.level === "warning").length, "warning")}</summary>
                {problems
                  .filter((p) => p.level === "warning")
                  .map((p, i) => (
                    <div key={i} className="m-help">
                      {p.line ? `line ${p.line}: ` : ""}
                      {p.message}
                    </div>
                  ))}
              </details>
            ) : null}
          </section>
        ) : null}
        {view === "split" ? <Divider onResize={setSplit} /> : null}
        {view !== "code" ? (
          <Erd
            ref={erd}
            diagram={diagram}
            positions={focus ? focus.positions : positions}
            visible={focus?.keys ?? null}
            selected={selected}
            selectedEdge={selectedEdge}
            relations={relations}
            wheel={wheel}
            onWheelMode={setWheel}
            onSelect={select}
            onSelectEdge={selectEdge}
            onMoveStart={() => !state.current.focus && record()}
            onMove={onMove}
            onMoveEnd={onMoveEnd}
            onConnect={(a, b) => {
              const r = relate(current, a, b);
              if (r.error || !r.fk || !r.from) return void toast.error(r.error ?? "Cannot relate these");
              void applyModel(r.model);
              selectEdge(edgeId(r.from, r.fk));
              const to = r.fk.references.name;
              const said = `${findTable(current, r.from)?.table.name}.${r.fk.columns[0]} now refers to ${to}.${r.fk.refColumns[0]}${r.addedColumn ? ` (added column ${r.addedColumn})` : ""}`;
              if (r.warning) toast.warning(said, { description: r.warning });
              else toast.success(said);
            }}
            onAddAt={(p) => {
              setAddAt(p);
              setDialog("table");
            }}
            banner={banner}
            empty={live.data ? emptyState : undefined}
          />
        ) : null}
        {view !== "code" && selTable ? (
          <TableInspector
            key={selTable.key}
            t={selTable}
            model={current}
            live={liveSchema}
            onChange={(m, c) => void applyModel(m, c)}
            onRename={(name) => {
              // The card keeps its place and stays selected under its new name.
              const to = tableKey(selTable.schema, name);
              const from = selTable.key;
              setPositions((p) => (p[from] ? { ...p, [to]: p[from]! } : p));
              if (focus) setFocus({ ...focus, keys: new Set([...focus.keys, to]), positions: { ...focus.positions, [to]: focus.positions[from]! } });
              void applyModel(renameTable(current, from, name));
              select(new Set([to]));
            }}
            onSelectEdge={selectEdge}
            onSelectTable={showTable}
            onRelation={() => setDialog("relation")}
            onClose={() => select(new Set())}
          />
        ) : view !== "code" && selEdge ? (
          <RelationInspector key={selEdge.id} edge={selEdge} model={current} live={liveSchema} onChange={(m, c) => void applyModel(m, c)} onSelectTable={showTable} onClose={() => setSelectedEdge(null)} />
        ) : null}
      </div>
      <TableDialog
        open={dialog === "table"}
        onOpenChange={(o) => {
          setDialog(o ? "table" : null);
          if (!o) setTimeout(() => setAddAt(null), 0);
        }}
        model={current}
        live={liveSchema}
        onAdd={(schema, table) => {
          const key = tableKey(schema, table.name);
          if (focus) {
            // Focused: the new table joins the focused ones, near the middle of the view.
            const taken = [...focus.keys].filter((k) => focus.positions[k]).map((k) => ({ ...focus.positions[k]!, w: NODE_WIDTH, h: diagram.byKey.get(k)?.h ?? 100 }));
            const at = addAt ?? erd.current?.center() ?? { x: 0, y: 0 };
            const p = freeSpot(taken, NODE_WIDTH, 100, { x: at.x - NODE_WIDTH / 2, y: at.y - 50 });
            setFocus({ ...focus, keys: new Set([...focus.keys, key]), positions: { ...focus.positions, [key]: p } });
          }
          void applyModel(addTable(current, schema, table));
          select(new Set([key]));
          pendingReveal.current = key;
        }}
      />
      <AddRelationDialog
        open={dialog === "relation"}
        onOpenChange={(o) => setDialog(o ? "relation" : null)}
        model={current}
        from={one}
        onAdd={(key, fk) => {
          void applyModel(addForeignKey(current, key, fk));
          selectEdge(edgeId(key, fk));
        }}
      />
      <ImportDialog open={dialog === "import"} onOpenChange={(o) => setDialog(o ? "import" : null)} onImport={(text) => onType(text)} />
      <ReviewDialog
        open={review}
        onOpenChange={setReview}
        title="Apply schema changes"
        target={database}
        connection={connection}
        reviewKey={["designer", database, JSON.stringify(current)]}
        load={async () => {
          const d = await diffDesignerModel(connectionId, database, { schema: current });
          return { plan: d.plan, decision: d.decision, upSql: d.upSql, downSql: d.downSql, changes: d.changes };
        }}
        apply={async ({ confirmation, transaction }) => {
          const d = await diffDesignerModel(connectionId, database, { schema: current });
          return applyChanges(connectionId, database, { changeSet: { database, schema: d.changes }, confirmation, transaction });
        }}
        onApplied={async () => {
          // Save the layout first: the reload below brings it back with the applied schema.
          if (saveLayout.current) window.clearTimeout(saveLayout.current);
          await saveDesignerLayout(connectionId, database, { positions }).catch(() => undefined);
          await qc.invalidateQueries({ queryKey: getGetDesignerModelQueryKey(connectionId, database) });
          await qc.invalidateQueries({ queryKey: getGetSchemaQueryKey(connectionId, database) });
        }}
      />
    </>
  );
}

/** The bar between the DBML and the diagram; drag to resize. */
function Divider({ onResize }: { onResize: (pct: number) => void }) {
  return (
    <div
      className="m-erd-divider"
      role="separator"
      aria-orientation="vertical"
      aria-label="Resize the DBML pane"
      onPointerDown={(e) => {
        const parent = e.currentTarget.parentElement!;
        e.currentTarget.setPointerCapture(e.pointerId);
        const move = (ev: PointerEvent) => {
          const r = parent.getBoundingClientRect();
          const pct = Math.min(70, Math.max(20, ((ev.clientX - r.left) / r.width) * 100));
          onResize(pct);
          try {
            localStorage.setItem("mesa.designer.split", String(Math.round(pct)));
          } catch {
            // Blocked storage: the width lasts for this page.
          }
        };
        const up = () => {
          window.removeEventListener("pointermove", move);
          window.removeEventListener("pointerup", up);
        };
        window.addEventListener("pointermove", move);
        window.addEventListener("pointerup", up);
      }}
    />
  );
}

function ArrangeMenu({ focused, disabled, onArrange }: { focused: number; disabled: boolean; onArrange: (k: LayoutKind) => void }) {
  const [open, setOpen] = React.useState(false);
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button variant="ghost" size="sm" disabled={disabled} title="Lay the tables out again">
          <LayoutGrid className="m-icon m-icon--sm" aria-hidden="true" />
          Arrange
          <ChevronDown className="m-icon m-icon--sm" aria-hidden="true" />
        </Button>
      </PopoverTrigger>
      <PopoverContent className="m-erd-menu" style={{ width: 300 }}>
        {focused ? <div className="m-combo__group">Arrange the {focused} focused tables</div> : null}
        {LAYOUTS.map((l) => (
          <button
            key={l.kind}
            type="button"
            className="m-menu__item m-erd-menu__rich"
            onClick={() => {
              onArrange(l.kind);
              setOpen(false);
            }}
          >
            {l.icon}
            <span>
              <span style={{ display: "block" }}>{l.title}</span>
              <span className="m-help" style={{ display: "block", whiteSpace: "normal" }}>
                {l.help}
              </span>
            </span>
          </button>
        ))}
      </PopoverContent>
    </Popover>
  );
}

function ChangesMenu({ changes, error, pending, onShow, onDiscard }: { changes: SchemaChange[]; error: string | null; pending: boolean; onShow: (c: SchemaChange) => void; onDiscard: () => void }) {
  const [open, setOpen] = React.useState(false);
  if (changes.length === 0 && !error) return null;
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button variant="ghost" size="sm" className={cn("m-erd-changes", error && "is-error")} title={error ? "The changes cannot be planned" : "What applying would change"}>
          <span className={cn("m-dot", error ? "m-dot--down" : "m-dot--pending")} />
          {error ? "Cannot plan" : plural(changes.length, "change")}
          {pending ? "…" : ""}
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="m-erd-menu" style={{ width: 380 }}>
        {error ? (
          <p className="m-help m-help--error" style={{ margin: 0, padding: "6px 8px", whiteSpace: "normal" }}>
            {error}
          </p>
        ) : null}
        {changes.length ? <div className="m-combo__group">Applying would</div> : null}
        <div style={{ maxHeight: 320, overflow: "auto" }}>
          {changes.map((c, i) => {
            const sign = c.kind.startsWith("drop") ? "−" : c.kind.startsWith("create") || c.kind.startsWith("add") ? "+" : "~";
            return (
              <button
                key={i}
                type="button"
                className="m-menu__item"
                style={{ whiteSpace: "normal", height: "auto", minHeight: 28, alignItems: "flex-start", paddingTop: 4, paddingBottom: 4 }}
                onClick={() => {
                  onShow(c);
                  setOpen(false);
                }}
              >
                <span className={cn("m-mono m-erd-sign", sign === "−" ? "is-dropped" : sign === "+" ? "is-new" : "is-changed")}>{sign}</span>
                <span>{describeChange(c)}</span>
              </button>
            );
          })}
        </div>
        <div className="m-menu__sep" />
        <button
          type="button"
          className="m-menu__item is-danger"
          onClick={() => {
            onDiscard();
            setOpen(false);
          }}
        >
          Discard all edits
        </button>
      </PopoverContent>
    </Popover>
  );
}
