/**
 * App-wide commands: one list behind the keyboard shortcuts, the command palette, the
 * shortcuts sheet and the desktop menus (which send `mesa:command` events). Views keep their
 * own keys for what only they do (⌘S in a table, ⌘↵ in the SQL editor).
 *
 * Browsers keep ⌘W, ⌘T, ⌘1–9 and ⌃Tab for their own tabs, so every tab command also has an
 * ⌥ shortcut. ⌥ shortcuts are ignored while typing, where ⌥ types characters ([, ], @…).
 */
import type { useNavigate } from "@tanstack/react-router";

import { tabLink, useTabs, type Tab } from "@/features/explorer/tabs.store";

import type { Binding } from "./keys";
import { useUi } from "./ui.store";

export type CommandGroup = "General" | "Tabs";

export interface Command {
  id: string;
  title: string;
  group: CommandGroup;
  keys: Binding[];
  /** Needs an open connection (tabs, the object filter). */
  inConnection?: boolean;
  /** Left out of the palette and the sheet (the sheet shows ⌘1–9 as one row). */
  hidden?: boolean;
}

const digits = [1, 2, 3, 4, 5, 6, 7, 8, 9];

export const commands: Command[] = [
  {
    id: "palette",
    title: "Command palette",
    group: "General",
    keys: [{ code: "KeyK", mod: true }],
  },
  {
    id: "help",
    title: "Keyboard shortcuts",
    group: "General",
    keys: [{ code: "Slash", mod: true }],
  },
  {
    id: "rail",
    title: "Show or hide the sidebar",
    group: "General",
    keys: [{ code: "KeyB", mod: true }],
  },
  {
    id: "filter",
    title: "Filter objects",
    group: "General",
    keys: [{ code: "KeyP", mod: true }],
    inConnection: true,
  },
  {
    id: "tab.new",
    title: "New query tab",
    group: "Tabs",
    inConnection: true,
    keys: [
      { code: "KeyT", mod: true, desktopOnly: true },
      { code: "KeyT", alt: true },
      { code: "KeyN", mod: true, alt: true },
    ],
  },
  {
    id: "tab.close",
    title: "Close tab",
    group: "Tabs",
    inConnection: true,
    keys: [
      { code: "KeyW", mod: true, desktopOnly: true },
      { code: "KeyW", alt: true },
    ],
  },
  {
    id: "tab.reopen",
    title: "Reopen closed tab",
    group: "Tabs",
    inConnection: true,
    keys: [
      { code: "KeyT", mod: true, shift: true, desktopOnly: true },
      { code: "KeyT", alt: true, shift: true },
    ],
  },
  {
    id: "tab.next",
    title: "Next tab",
    group: "Tabs",
    inConnection: true,
    keys: [
      { code: "Tab", ctrl: true, desktopOnly: true },
      { code: "BracketRight", mod: true, shift: true, desktopOnly: true },
      { code: "BracketRight", alt: true },
    ],
  },
  {
    id: "tab.prev",
    title: "Previous tab",
    group: "Tabs",
    inConnection: true,
    keys: [
      { code: "Tab", ctrl: true, shift: true, desktopOnly: true },
      { code: "BracketLeft", mod: true, shift: true, desktopOnly: true },
      { code: "BracketLeft", alt: true },
    ],
  },
  ...digits.map(
    (n): Command => ({
      id: `tab.${n}`,
      title: n === 9 ? "Go to the last tab" : `Go to tab ${n}`,
      group: "Tabs",
      inConnection: true,
      hidden: true,
      keys: [
        { code: `Digit${n}`, mod: true, desktopOnly: true },
        { code: `Digit${n}`, alt: true },
      ],
    }),
  ),
];

export function command(id: string): Command | undefined {
  return commands.find((c) => c.id === id);
}

/** The shortcuts that work where Mesa runs: all of them in the desktop app. */
export function usableKeys(c: Command, desktop: boolean): Binding[] {
  return desktop ? c.keys : c.keys.filter((k) => !k.desktopOnly);
}

export interface CommandContext {
  navigate: ReturnType<typeof useNavigate>;
  connectionId?: string;
}

const lastRun = new Map<string, number>();

/**
 * Runs a command. A desktop menu accelerator and the page can both see one key press, so the
 * same command within 250 ms runs once.
 */
export function runCommand(id: string, ctx: CommandContext): boolean {
  const c = command(id);
  if (!c || (c.inConnection && !ctx.connectionId)) return false;
  const now = performance.now();
  if (now - (lastRun.get(id) ?? -Infinity) < 250) return true;
  lastRun.set(id, now);

  const ui = useUi.getState();
  switch (id) {
    case "palette":
      ui.setPaletteOpen(!ui.paletteOpen);
      return true;
    case "help":
      ui.setPaletteOpen(false);
      ui.setHelpOpen(!ui.helpOpen);
      return true;
    case "rail":
      ui.toggleRail();
      return true;
    case "filter":
      window.dispatchEvent(new Event("mesa:focus-filter"));
      return true;
  }
  return runTabCommand(id, ctx.connectionId!, ctx.navigate);
}

function runTabCommand(id: string, connectionId: string, navigate: CommandContext["navigate"]): boolean {
  const store = useTabs.getState();
  const conn = store.byConnection[connectionId];
  const tabs = conn?.tabs ?? [];
  const index = tabs.findIndex((t) => t.id === conn?.activeId);
  const show = (tab: Tab | null | undefined) => {
    if (tab) {
      store.activate(connectionId, tab.id);
      void navigate(tabLink(connectionId, tab));
    } else void navigate({ to: "/c/$connectionId", params: { connectionId } });
  };

  switch (id) {
    case "tab.new":
      show(store.newQuery(connectionId));
      return true;
    case "tab.close":
      if (index >= 0) show(store.close(connectionId, tabs[index].id));
      return true;
    case "tab.reopen": {
      const tab = store.reopen(connectionId);
      if (tab) show(tab);
      return true;
    }
    case "tab.next":
    case "tab.prev": {
      if (tabs.length < 2) return true;
      const step = id === "tab.next" ? 1 : -1;
      show(tabs[(Math.max(index, 0) + step + tabs.length) % tabs.length]);
      return true;
    }
  }
  const n = Number(id.slice("tab.".length));
  if (n >= 1 && n <= 9) {
    const tab = n === 9 ? tabs[tabs.length - 1] : tabs[n - 1];
    if (tab) show(tab);
    return true;
  }
  return false;
}
