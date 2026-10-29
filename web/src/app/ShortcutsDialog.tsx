/** The keyboard shortcuts sheet (? or ⌘/): the app-wide commands, then each view's own keys. */
import { Dialog, DialogBody, DialogContent, DialogDescription, DialogHead, DialogTitle } from "@/ui/primitives/dialog";
import { Kbd } from "@/ui/controls";
import { isMac } from "@/lib/utils";

import { commands, usableKeys, type CommandGroup } from "./commands";
import { label, shortcut, type Binding } from "./keys";
import { useIsDesktop } from "./shortcuts";
import { useUi } from "./ui.store";

interface Row {
  title: string;
  keys: string[];
}

const k = (code: string, mods: Omit<Binding, "code"> = {}) => label({ code, ...mods });

/** Keys that belong to one view; they are handled there, not in commands.ts. */
const viewKeys: { title: string; rows: Row[] }[] = [
  {
    title: "Object tree",
    rows: [
      { title: "Move", keys: ["↑ ↓", "Home", "End"] },
      { title: "Expand, or step into", keys: ["→"] },
      { title: "Collapse, or step out", keys: ["←"] },
      { title: "Open the table", keys: [k("Enter")] },
      { title: "From the filter: into the tree, open the first match", keys: ["↓", k("Enter")] },
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
    rows: [{ title: "Review & apply the model", keys: [k("KeyS", { mod: true })] }],
  },
  {
    title: "Connections",
    rows: [{ title: "Search connections", keys: ["/"] }],
  },
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

function Section({ title, rows }: { title: string; rows: Row[] }) {
  return (
    <section style={{ breakInside: "avoid", marginBottom: "var(--space-4)" }}>
      <h3 className="m-label" style={{ margin: "0 0 var(--space-2)" }}>
        {title}
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
  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent width={760}>
        <DialogHead>
          <DialogTitle>Keyboard shortcuts</DialogTitle>
          <DialogDescription>
            {desktop
              ? "Every shortcut below works in the desktop app."
              : `In a browser, ${shortcut("KeyW")}, ${shortcut("KeyT")}, ${shortcut("Digit1")}–9 and ${shortcut("Tab", { ctrl: true })} belong to the browser, so tabs use ${isMac ? "⌥" : "Alt"} shortcuts here. The desktop app takes both. ${isMac ? "⌥" : "Alt"} shortcuts are off while you type in a field.`}
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
          <Section title="Tabs" rows={commandRows("Tabs", desktop)} />
          {viewKeys.map((s) => (
            <Section key={s.title} title={s.title} rows={s.rows} />
          ))}
        </DialogBody>
      </DialogContent>
    </Dialog>
  );
}
