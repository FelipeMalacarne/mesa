/**
 * The keyboard shortcuts sheet (? or ⌘/): the app-wide commands, then each view's own keys,
 * then the vim keys (on or off in Settings → Keyboard) and, in the desktop app, the zoom.
 */
import { Dialog, DialogBody, DialogContent, DialogDescription, DialogHead, DialogTitle } from "@/ui/primitives/dialog";
import { Kbd } from "@/ui/controls";
import { isMac } from "@/lib/utils";

import { commands, usableKeys, type CommandGroup } from "./commands";
import { label, shortcut, type Binding } from "./keys";
import { useIsDesktop } from "./shortcuts";
import { useUi } from "./ui.store";
import { useVim } from "./vim";
import { useDesktopZoom } from "./zoom";

interface Row {
  title: string;
  keys: string[];
}

const k = (code: string, mods: Omit<Binding, "code"> = {}) => label({ code, ...mods });
const ctrl = (code: string) => k(code, { ctrl: true });

/** Keys that belong to one view; they are handled there, not in commands.ts. */
const viewKeys: { title: string; rows: Row[] }[] = [
  {
    title: "Object search",
    rows: [
      { title: "Next or previous match", keys: ["↑ ↓", ctrl("KeyN"), ctrl("KeyJ"), ...(isMac ? [ctrl("KeyP"), ctrl("KeyK")] : [])] },
      { title: "Open it and move into it", keys: [k("Enter")] },
      { title: "Show it in the tree", keys: [k("Enter", { shift: true })] },
      { title: "Clear, then go back", keys: ["Esc"] },
      { title: "Into the tree (empty search)", keys: ["↓"] },
    ],
  },
  {
    title: "Object tree",
    rows: [
      { title: "Move", keys: ["↑ ↓", "j k"] },
      { title: "Expand, or step into", keys: ["→", "l"] },
      { title: "Collapse, or step out", keys: ["←", "h"] },
      { title: "First or last item", keys: ["Home", "End", "g g", "G"] },
      { title: "Ten items down or up", keys: [ctrl("KeyD"), ctrl("KeyU")] },
      { title: "Open and move into the view", keys: [k("Enter"), "o"] },
      { title: "Open and stay in the tree", keys: ["Space"] },
      { title: "Search", keys: ["/"] },
    ],
  },
  {
    title: "Table data",
    rows: [
      {
        title: "Review & commit staged edits",
        keys: [k("KeyS", { mod: true })],
      },
      { title: "Filter the rows (WHERE)", keys: [k("KeyF", { mod: true })] },
      { title: "Show or hide the row inspector", keys: [k("KeyI", { mod: true })] },
      { title: "Move between cells", keys: ["↑ ↓ ← →", "Tab"] },
      { title: "First or last cell of the row", keys: ["Home", "End"] },
      { title: "First or last row", keys: [k("ArrowUp", { mod: true }), k("ArrowDown", { mod: true })] },
      { title: "Up or down a page", keys: ["PgUp", "PgDn"] },
      { title: "Edit the cell", keys: [k("Enter"), "F2", "type"] },
      {
        title: "Delete or restore the row",
        keys: [k("Backspace", { mod: true })],
      },
      { title: "Copy the cell", keys: [k("KeyC", { mod: true })] },
      { title: "Cancel the edit, leave the grid", keys: ["Esc"] },
    ],
  },
  {
    title: "SQL editor",
    rows: [
      { title: "Run", keys: [k("Enter", { mod: true })] },
      { title: "Explain", keys: [k("Enter", { mod: true, shift: true })] },
      { title: "Save the query", keys: [k("KeyS", { mod: true })] },
      { title: "Results: move, copy the cell", keys: ["↑ ↓ ← →", k("KeyC", { mod: true })] },
    ],
  },
  {
    title: "Review and dialogs",
    rows: [
      { title: "Apply the reviewed change", keys: [k("Enter", { mod: true })] },
      { title: "Close without applying", keys: ["Esc"] },
    ],
  },
  {
    title: "Schema designer",
    rows: [
      { title: "Review & apply the model", keys: [k("KeyS", { mod: true })] },
      { title: "Zoom at the pointer", keys: ["scroll", `${isMac ? "⌘" : "Ctrl+"}scroll`, "pinch"] },
      { title: "Zoom in or out", keys: ["+", "-"] },
      { title: "Zoom to fit, to the selection, to 100%", keys: [k("Digit1", { shift: true }), k("Digit2", { shift: true }), k("Digit0", { shift: true })] },
      { title: "Pan", keys: ["drag", "Shift+scroll"] },
      { title: "Select several, or an area", keys: ["Shift+click", "Shift+drag"] },
      { title: "Find a table", keys: ["/", k("KeyF", { mod: true })] },
      { title: "Focus on the selection and its neighbours", keys: ["f"] },
      { title: "Move the selected tables", keys: ["↑ ↓ ← →", "Shift+↑ ↓ ← →"] },
      { title: "Drop the selected tables or relation", keys: ["Del", "⌫"] },
      { title: "Undo or redo", keys: [k("KeyZ", { mod: true }), k("KeyZ", { mod: true, shift: true })] },
      { title: "Clear the selection, leave focus", keys: ["Esc"] },
    ],
  },
  {
    title: "Connections and activity",
    rows: [
      { title: "Search connections", keys: ["/"] },
      { title: "Into the list, or open the first match", keys: ["↓", k("Enter")] },
      { title: "Move through the list", keys: ["↑ ↓", "j k", "g g", "G"] },
      { title: "Open the row, or expand or collapse it", keys: [k("Enter"), "o", "l", "h"] },
    ],
  },
];

/** The vim keys that views handle; the pane moves come from commands.ts. */
const vimRows: Row[] = [
  { title: "Grids: move between cells", keys: ["h j k l"] },
  { title: "Grids: next or previous column", keys: ["w", "b"] },
  { title: "Grids: first or last column", keys: ["0", "$"] },
  { title: "Grids: first or last row (5G: row 5)", keys: ["g g", "G"] },
  { title: "Grids: half a page down or up", keys: [ctrl("KeyD"), ctrl("KeyU")] },
  { title: "Grids: a count repeats the move", keys: ["5 j"] },
  { title: "Grids: edit the cell (Esc leaves it)", keys: ["i", "a"] },
  { title: "Grids: replace the cell", keys: ["s", "c"] },
  { title: "Grids: delete or restore the row", keys: ["d d"] },
  { title: "Grids: copy the cell, or the row", keys: ["y", "Y"] },
  { title: "Grids: add a row", keys: ["o"] },
  { title: "Grids: go to the filter (Esc comes back)", keys: ["/"] },
  { title: "Next or previous tab", keys: ["g t", "g T"] },
  { title: "Command palette", keys: [":"] },
];

function commandRows(group: CommandGroup, desktop: boolean): Row[] {
  const rows: Row[] = commands
    .filter((c) => c.group === group && !c.hidden)
    .map((c) => ({
      title: c.title,
      keys: [...(c.id === "help" ? ["?"] : []), ...usableKeys(c, desktop).map(label)],
    }));
  if (group === "Tabs") {
    const one = commands.find((c) => c.id === "tab.1")!;
    rows.push({
      title: "Go to tab 1–8, or the last tab",
      keys: usableKeys(one, desktop).map((b) => label(b).replace(/1$/, "1…9")),
    });
  }
  return rows.filter((r) => r.keys.length > 0);
}

function Section({ title, rows, note }: { title: string; rows: Row[]; note?: string }) {
  return (
    <section style={{ breakInside: "avoid", marginBottom: "var(--space-4)", opacity: note === "off" ? 0.6 : undefined }}>
      <h3 className="m-label" style={{ margin: "0 0 var(--space-2)" }}>
        {title}
        {note ? <span className="m-settings__state">{note}</span> : null}
      </h3>
      <dl
        style={{
          margin: 0,
          display: "grid",
          gridTemplateColumns: "1fr auto",
          rowGap: "var(--space-1)",
          columnGap: "var(--space-3)",
        }}
      >
        {rows.map((r) => (
          <div key={r.title} style={{ display: "contents" }}>
            <dt className="m-help" style={{ color: "var(--foreground)", alignSelf: "center" }}>
              {r.title}
            </dt>
            <dd
              style={{
                margin: 0,
                display: "flex",
                gap: "var(--space-1)",
                justifyContent: "flex-end",
                flexWrap: "wrap",
              }}
            >
              {r.keys.map((key) => (
                <Kbd key={key}>{key}</Kbd>
              ))}
            </dd>
          </div>
        ))}
      </dl>
    </section>
  );
}

export function ShortcutsDialog() {
  const open = useUi((s) => s.helpOpen);
  const setOpen = useUi((s) => s.setHelpOpen);
  const desktop = useIsDesktop();
  const vim = useVim((s) => s.vim);
  const { zoom } = useDesktopZoom();
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent width={760}>
        <DialogHead>
          <DialogTitle>Keyboard shortcuts</DialogTitle>
          <DialogDescription>
            {desktop
              ? "Every shortcut below works in the desktop app."
              : `In a browser, ${shortcut("KeyW")}, ${shortcut("KeyT")}, ${shortcut("Digit1")}–9 and ${shortcut("Tab", { ctrl: true })} belong to the browser, so tabs use ${isMac ? "⌥" : "Alt"} shortcuts here. The desktop app takes both. ${isMac ? "⌥" : "Alt"} shortcuts are off while you type in a field.`}{" "}
            Vim keys are {vim ? "on" : "off"}; Settings ({shortcut("Comma")}) → Keyboard turns them {vim ? "off" : "on"}.
          </DialogDescription>
        </DialogHead>
        <DialogBody
          style={{
            columns: "2 300px",
            columnGap: "var(--space-6)",
            maxHeight: "70vh",
            overflowY: "auto",
          }}
        >
          <Section title="General" rows={commandRows("General", desktop)} />
          {zoom?.native ? (
            <Section
              title="Window"
              rows={[
                { title: "Zoom in", keys: [shortcut("Equal")] },
                { title: "Zoom out", keys: [shortcut("Minus")] },
                { title: "Actual size", keys: [shortcut("Digit0")] },
              ]}
            />
          ) : null}
          <Section title="Tabs" rows={commandRows("Tabs", desktop)} />
          {viewKeys.map((s) => (
            <Section key={s.title} title={s.title} rows={s.rows} />
          ))}
          <Section title="Vim keys" note={vim ? "on" : "off"} rows={[...commandRows("Vim keys", desktop), ...vimRows]} />
        </DialogBody>
      </DialogContent>
    </Dialog>
  );
}
