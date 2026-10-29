/**
 * App commands: what a shortcut, the palette and the desktop app's menu run. The native menu
 * sends them as `mesa:command` events (cmd/desktop/main.go), so a menu item and its key do the
 * same thing whichever of the two sees the key first. `SHORTCUTS` is the reference Settings shows.
 * With vim keys on (vim.ts) a few more apply: Ctrl+h/j/k/l (Alt outside macOS) between panes,
 * and, outside text fields, gt / gT for tabs and `:` for the palette.
 */
import * as React from "react";
import { useNavigate, useParams } from "@tanstack/react-router";

import { isMac } from "@/lib/utils";
import { activeTab, closeTab, cycleTab, goToTabAt, newQueryTab, reopenTab } from "@/features/explorer/tabActions";
import { focusPane } from "@/features/workspace/focus";

import { useUi } from "./ui.store";
import { useVim } from "./vim";

export type Command =
  | "palette"
  | "settings"
  | "shortcuts"
  | "sidebar.toggle"
  | "objects.find"
  | "objects.focus"
  | "tab.new"
  | "tab.close"
  | "tab.reopen"
  | "tab.next"
  | "tab.prev"
  | "tab.last"
  | `tab.${1 | 2 | 3 | 4 | 5 | 6 | 7 | 8}`
  | "pane.left"
  | "pane.right"
  | "pane.up"
  | "pane.down";

export const COMMAND_EVENT = "mesa:command";

type KeyEventLike = Pick<KeyboardEvent, "key" | "code" | "metaKey" | "ctrlKey" | "shiftKey" | "altKey">;

const PANES: Record<string, Command> = { h: "pane.left", j: "pane.down", k: "pane.up", l: "pane.right" };

/**
 * The command a key press asks for, if any. `mod` is ⌘ on macOS and Ctrl elsewhere; with vim
 * keys on, Ctrl+h/j/k/l (Alt+h/j/k/l outside macOS, where Ctrl is `mod`) move between panes.
 */
export function matchShortcut(e: KeyEventLike, mac: boolean = isMac, vim = false): Command | null {
  const key = e.key.length === 1 ? e.key.toLowerCase() : e.key;
  if (vim && !e.shiftKey && !e.metaKey && (mac ? e.ctrlKey && !e.altKey : e.altKey && !e.ctrlKey)) {
    // ⌥ changes the character outside macOS too on some layouts: read the physical key.
    const pane = PANES[mac ? key : e.code.replace(/^Key/, "").toLowerCase()];
    if (pane) return pane;
  }
  // Ctrl+Tab and Ctrl+PageDown move between tabs on every platform (browsers keep them for
  // their own tabs; the desktop app gets them).
  if (e.ctrlKey && !e.metaKey && !e.altKey) {
    if (key === "Tab") return e.shiftKey ? "tab.prev" : "tab.next";
    if (!e.shiftKey && key === "PageDown") return "tab.next";
    if (!e.shiftKey && key === "PageUp") return "tab.prev";
  }
  const mod = mac ? e.metaKey && !e.ctrlKey : e.ctrlKey && !e.metaKey;
  if (!mod) return null;
  if (e.altKey) {
    // ⌥ changes the character on macOS (⌥N is a dead key), so read the physical key.
    if (!e.shiftKey && e.code === "KeyN") return "tab.new";
    if (key === "ArrowRight") return "tab.next";
    if (key === "ArrowLeft") return "tab.prev";
    return null;
  }
  if (e.shiftKey) {
    switch (key) {
      case "t":
        return "tab.reopen";
      case "e":
        return "objects.focus";
      case "]":
      case "}":
        return "tab.next";
      case "[":
      case "{":
        return "tab.prev";
    }
    return null;
  }
  switch (key) {
    case "k":
      return "palette";
    case "p":
      return "objects.find";
    case "b":
      return "sidebar.toggle";
    case "t":
      return "tab.new";
    case "w":
      return "tab.close";
    case ",":
      return "settings";
    case "/":
      return "shortcuts";
    case "9":
      return "tab.last";
  }
  if (key >= "1" && key <= "8" && key.length === 1) return `tab.${Number(key) as 1}`;
  return null;
}

function isTextField(target: EventTarget | null): boolean {
  return target instanceof HTMLElement && (target.isContentEditable || target.matches("input, textarea, select"));
}

/** Commands that still run while a dialog is open; the rest wait until it closes. */
const OVER_DIALOGS = new Set<Command>(["palette", "settings", "shortcuts"]);

/** Wires shortcuts and `mesa:command` events to the app. Mounted once, in the root route. */
export function useCommands() {
  const navigate = useNavigate();
  const params = useParams({ strict: false }) as { connectionId?: string };
  const connectionId = params.connectionId;

  const run = React.useCallback(
    (cmd: Command): boolean => {
      const ui = useUi.getState();
      switch (cmd) {
        case "palette":
          ui.setSettingsOpen(false);
          ui.setPaletteOpen(!ui.paletteOpen);
          return true;
        case "settings":
          if (ui.settingsOpen && ui.settingsSection !== "keyboard") ui.setSettingsOpen(false);
          else ui.openSettings("appearance");
          return true;
        case "shortcuts":
          if (ui.settingsOpen && ui.settingsSection === "keyboard") ui.setSettingsOpen(false);
          else ui.openSettings("keyboard");
          return true;
      }
      if (!connectionId) return false;
      switch (cmd) {
        case "sidebar.toggle":
          ui.toggleRail();
          return true;
        case "objects.find":
          ui.focusRail("filter");
          return true;
        case "objects.focus":
          ui.focusRail("tree");
          return true;
        case "tab.new":
          newQueryTab(navigate, connectionId);
          return true;
        case "tab.close": {
          const tab = activeTab(connectionId);
          return tab ? closeTab(navigate, connectionId, tab.id) : false;
        }
        case "tab.reopen":
          return reopenTab(navigate, connectionId);
        case "tab.next":
          return cycleTab(navigate, connectionId, 1);
        case "tab.prev":
          return cycleTab(navigate, connectionId, -1);
        case "tab.last":
          return goToTabAt(navigate, connectionId, -1);
        case "pane.left":
          if (document.activeElement?.closest(".m-rail")) return false;
          ui.focusRail("tree");
          return true;
        case "pane.right":
          return document.querySelector("main")?.contains(document.activeElement) ? false : focusPane(0);
        case "pane.down":
          return focusPane(1);
        case "pane.up":
          return focusPane(-1);
        default:
          return goToTabAt(navigate, connectionId, Number(cmd.slice("tab.".length)));
      }
    },
    [navigate, connectionId],
  );

  const lastG = React.useRef(0);

  React.useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      const vim = useVim.getState().vim;
      // gt / gT and `:` with vim keys on, outside text fields. A grid or the tree may have taken
      // the g (it starts their gg), so it is noted before looking at defaultPrevented.
      const plain = vim && !e.metaKey && !e.ctrlKey && !e.altKey && !isTextField(e.target);
      if (plain && e.key === "g") {
        lastG.current = Date.now();
        return;
      }
      if (plain && !e.defaultPrevented && (e.key === "t" || e.key === "T") && Date.now() - lastG.current < 1000) {
        lastG.current = 0;
        if (!document.querySelector(".m-overlay") && run(e.key === "t" ? "tab.next" : "tab.prev")) e.preventDefault();
        return;
      }
      if (plain && !e.defaultPrevented && e.key === ":" && !document.querySelector(".m-overlay")) {
        e.preventDefault();
        run("palette");
        return;
      }
      if (e.defaultPrevented) return;
      const cmd = matchShortcut(e, isMac, vim);
      if (!cmd) return;
      if (!OVER_DIALOGS.has(cmd) && document.querySelector(".m-overlay")) return;
      // Holding ⌘W should not empty the strip.
      if (e.repeat && (cmd === "tab.close" || cmd === "tab.reopen" || cmd === "tab.new")) {
        e.preventDefault();
        return;
      }
      if (run(cmd)) e.preventDefault();
    };
    const onCommand = (e: Event) => {
      const cmd = (e as CustomEvent<unknown>).detail;
      if (typeof cmd !== "string") return;
      if (!OVER_DIALOGS.has(cmd as Command) && document.querySelector(".m-overlay")) return;
      run(cmd as Command);
    };
    window.addEventListener("keydown", onKey);
    window.addEventListener(COMMAND_EVENT, onCommand);
    return () => {
      window.removeEventListener("keydown", onKey);
      window.removeEventListener(COMMAND_EVENT, onCommand);
    };
  }, [run]);
}

export interface ShortcutHelp {
  /** Alternatives, each a combo for `keyLabel` ("mod+shift+t") or a literal sequence ("g g"). */
  keys: string[];
  label: string;
  /** Shown only where the desktop app zooms the window itself. */
  nativeZoom?: boolean;
}

const PANE_MOD = isMac ? "ctrl" : "alt";

export const SHORTCUTS: { group: string; vim?: boolean; items: ShortcutHelp[] }[] = [
  {
    group: "General",
    items: [
      { keys: ["mod+k"], label: "Command palette" },
      { keys: ["mod+,"], label: "Settings" },
      { keys: ["mod+/"], label: "Keyboard shortcuts" },
      { keys: ["mod+b"], label: "Show or hide the sidebar" },
      { keys: ["mod+="], label: "Zoom in", nativeZoom: true },
      { keys: ["mod+-"], label: "Zoom out", nativeZoom: true },
      { keys: ["mod+0"], label: "Actual size", nativeZoom: true },
    ],
  },
  {
    group: "Tabs",
    items: [
      { keys: ["mod+t", "mod+alt+n"], label: "New query" },
      { keys: ["mod+w"], label: "Close tab" },
      { keys: ["mod+shift+t"], label: "Reopen closed tab" },
      { keys: ["mod+shift+]", "ctrl+tab"], label: "Next tab" },
      { keys: ["mod+shift+[", "ctrl+shift+tab"], label: "Previous tab" },
      { keys: ["mod+1…8"], label: "Go to tab 1 to 8" },
      { keys: ["mod+9"], label: "Go to the last tab" },
    ],
  },
  {
    group: "Objects",
    items: [
      { keys: ["mod+p"], label: "Find a table, view, function or column" },
      { keys: ["mod+shift+e"], label: "Focus the object tree" },
      { keys: ["down", "ctrl+n", "ctrl+j"], label: "Next result / next item" },
      { keys: ["up", "ctrl+p", "ctrl+k"], label: "Previous result / previous item" },
      { keys: ["enter"], label: "Open" },
      { keys: ["shift+enter"], label: "Show a result in the tree" },
      { keys: ["esc"], label: "Clear the search" },
      { keys: ["j", "k"], label: "Tree: next / previous item" },
      { keys: ["l", "h"], label: "Tree: expand / collapse (or go to parent)" },
      { keys: ["space"], label: "Tree: open and stay in the tree" },
      { keys: ["g g", "G"], label: "Tree: first / last item" },
      { keys: ["/"], label: "Tree: search" },
    ],
  },
  {
    group: "Tables",
    items: [
      { keys: ["mod+s"], label: "Review and commit staged changes" },
      { keys: ["mod+i"], label: "Show or hide the inspector" },
      { keys: ["enter", "F2"], label: "Edit the cell" },
      { keys: ["mod+c"], label: "Copy the cell" },
      { keys: ["mod+backspace"], label: "Delete the row" },
    ],
  },
  {
    group: "SQL editor",
    items: [
      { keys: ["mod+enter"], label: "Run the statement under the cursor" },
      { keys: ["mod+shift+enter"], label: "Explain it" },
      { keys: ["mod+s"], label: "Save the query" },
    ],
  },
  {
    group: "Lists",
    items: [
      { keys: ["j", "k"], label: "Next / previous row (connections, activity)" },
      { keys: ["enter", "o"], label: "Open the row" },
      { keys: ["l", "h"], label: "Expand / collapse it" },
    ],
  },
  {
    group: "Vim keys",
    vim: true,
    items: [
      { keys: ["h", "j", "k", "l"], label: "Grids: move between cells" },
      { keys: ["w", "b"], label: "Grids: next / previous column" },
      { keys: ["0", "$"], label: "Grids: first / last column" },
      { keys: ["g g", "G"], label: "Grids: first / last row (5G: row 5)" },
      { keys: ["ctrl+d", "ctrl+u"], label: "Grids: half a page down / up" },
      { keys: ["5 j"], label: "Grids: a count repeats the move" },
      { keys: ["i", "a"], label: "Grids: edit the cell (Esc leaves it)" },
      { keys: ["s", "c"], label: "Grids: replace the cell" },
      { keys: ["d d"], label: "Grids: mark the row for deletion" },
      { keys: ["y", "Y"], label: "Grids: copy the cell / the row" },
      { keys: ["o"], label: "Grids: add a row" },
      { keys: ["/"], label: "Grids: go to the filter (Esc comes back)" },
      { keys: ["g t", "g T"], label: "Next / previous tab" },
      { keys: [`${PANE_MOD}+h`, `${PANE_MOD}+l`], label: "Sidebar / view" },
      { keys: [`${PANE_MOD}+j`, `${PANE_MOD}+k`], label: "Next / previous pane in the view (SQL editor ↔ results)" },
      { keys: [":"], label: "Command palette" },
    ],
  },
];
