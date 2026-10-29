/**
 * Settings (⌘,): appearance (theme, window zoom, row density), the keyboard reference (⌘/) and
 * the version. Choices apply at once and are kept per viewer; the zoom belongs to the desktop
 * window, so in a browser the page points at the browser's own zoom instead.
 */
import * as React from "react";
import { Info, Keyboard, Minus, Palette, Plus } from "lucide-react";
import { toast } from "sonner";

import type { Engine } from "@/lib/api/mesaAPI.schemas";
import { useGetSystemInfo } from "@/lib/api/system/system";
import { engineLabel } from "@/lib/format";
import { keyLabel } from "@/lib/keys";
import { cn, isMac } from "@/lib/utils";
import { Button } from "@/ui/Button";
import { Kbd } from "@/ui/controls";
import { Dialog, DialogContent, DialogDescription, DialogTitle } from "@/ui/primitives/dialog";

import { SHORTCUTS } from "./commands";
import { useDensity, type Density } from "./density";
import { useTheme, type Theme } from "./theme";
import { useUi, type SettingsSection } from "./ui.store";
import { useDesktopZoom } from "./zoom";

const SECTIONS: { id: SettingsSection; label: string; icon: typeof Palette }[] = [
  { id: "appearance", label: "Appearance", icon: Palette },
  { id: "keyboard", label: "Keyboard", icon: Keyboard },
  { id: "about", label: "About", icon: Info },
];

export function SettingsDialog() {
  const open = useUi((s) => s.settingsOpen);
  const section = useUi((s) => s.settingsSection);
  const setOpen = useUi((s) => s.setSettingsOpen);
  const openSettings = useUi((s) => s.openSettings);

  // ↑/↓ (or k/j) move between sections while the section list has focus.
  const onNavKey = (e: React.KeyboardEvent) => {
    const step = e.key === "ArrowDown" || e.key === "j" ? 1 : e.key === "ArrowUp" || e.key === "k" ? -1 : 0;
    if (!step) return;
    e.preventDefault();
    const i = SECTIONS.findIndex((s) => s.id === section);
    const next = SECTIONS[(i + step + SECTIONS.length) % SECTIONS.length];
    openSettings(next.id);
    e.currentTarget.querySelector<HTMLElement>(`[data-section="${next.id}"]`)?.focus();
  };

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogContent className="m-settings" aria-describedby={undefined}>
        <nav className="m-settings__nav" aria-label="Settings sections" role="tablist" aria-orientation="vertical" onKeyDown={onNavKey}>
          <DialogTitle className="m-settings__title">Settings</DialogTitle>
          {SECTIONS.map((s) => (
            <button
              key={s.id}
              type="button"
              role="tab"
              data-section={s.id}
              aria-selected={section === s.id}
              tabIndex={section === s.id ? 0 : -1}
              className={cn("m-tree__item", section === s.id && "is-selected")}
              onClick={() => openSettings(s.id)}
            >
              <s.icon className="m-icon m-icon--sm" aria-hidden="true" />
              <span className="m-tree__name">{s.label}</span>
            </button>
          ))}
        </nav>
        <div className="m-settings__body" role="tabpanel">
          {section === "appearance" ? <Appearance /> : section === "keyboard" ? <KeyboardHelp /> : <About />}
        </div>
      </DialogContent>
    </Dialog>
  );
}

function Row({ label, help, children }: { label: string; help?: React.ReactNode; children: React.ReactNode }) {
  return (
    <div className="m-settings__row">
      <div style={{ minWidth: 0 }}>
        <div className="m-label">{label}</div>
        {help ? <div className="m-help">{help}</div> : null}
      </div>
      <div className="m-settings__control">{children}</div>
    </div>
  );
}

function Segmented<T extends string>({ label, value, options, onChange }: { label: string; value: T; options: { value: T; label: string }[]; onChange: (v: T) => void }) {
  // A radio group: one tab stop, the arrows (or h/l) pick.
  const onKeyDown = (e: React.KeyboardEvent<HTMLDivElement>) => {
    const step = e.key === "ArrowRight" || e.key === "ArrowDown" || e.key === "l" ? 1 : e.key === "ArrowLeft" || e.key === "ArrowUp" || e.key === "h" ? -1 : 0;
    if (!step) return;
    e.preventDefault();
    const i = options.findIndex((o) => o.value === value);
    const next = options[(i + step + options.length) % options.length];
    onChange(next.value);
    e.currentTarget.querySelector<HTMLElement>(`[data-value="${next.value}"]`)?.focus();
  };
  return (
    <div className="m-seg" role="radiogroup" aria-label={label} onKeyDown={onKeyDown}>
      {options.map((o) => (
        <button
          key={o.value}
          type="button"
          role="radio"
          data-value={o.value}
          aria-checked={value === o.value}
          tabIndex={value === o.value ? 0 : -1}
          className={cn("m-seg__item", value === o.value && "is-active")}
          onClick={() => onChange(o.value)}
        >
          {o.label}
        </button>
      ))}
    </div>
  );
}

function Appearance() {
  const theme = useTheme((s) => s.theme);
  const setTheme = useTheme((s) => s.setTheme);
  const density = useDensity((s) => s.density);
  const setDensity = useDensity((s) => s.setDensity);
  return (
    <>
      <h2 className="m-section-title">Appearance</h2>
      <Row label="Theme">
        <Segmented<Theme>
          label="Theme"
          value={theme}
          onChange={setTheme}
          options={[
            { value: "dark", label: "Dark" },
            { value: "light", label: "Light" },
          ]}
        />
      </Row>
      <ZoomRow />
      <Row label="Row density" help="Row height in tables, results and the object tree.">
        <Segmented<Density>
          label="Row density"
          value={density}
          onChange={setDensity}
          options={[
            { value: "compact", label: "Compact" },
            { value: "comfortable", label: "Comfortable" },
          ]}
        />
      </Row>
    </>
  );
}

function ZoomRow() {
  const { zoom, loading, step } = useDesktopZoom();
  const change = (to: "in" | "out" | "reset") => step(to).catch((e: unknown) => toast.error(e instanceof Error ? e.message : String(e)));
  if (loading) return <Row label="Zoom">{null}</Row>;
  if (!zoom) {
    return (
      <Row label="Zoom" help="Mesa follows your browser's zoom.">
        <span className="m-help">
          <Kbd>{keyLabel("mod+=", isMac)}</Kbd> <Kbd>{keyLabel("mod+-", isMac)}</Kbd> <Kbd>{keyLabel("mod+0", isMac)}</Kbd>
        </span>
      </Row>
    );
  }
  if (!zoom.native) {
    return (
      <Row label="Zoom" help="The window zooms with Ctrl + = and Ctrl + −, or Ctrl and the mouse wheel; Ctrl + 0 resets it.">
        {null}
      </Row>
    );
  }
  const pct = `${Math.round(zoom.level * 100)}%`;
  return (
    <Row label="Zoom" help={<>Scales the whole window. Also in the View menu: <Kbd>{keyLabel("mod+=")}</Kbd> <Kbd>{keyLabel("mod+-")}</Kbd> <Kbd>{keyLabel("mod+0")}</Kbd></>}>
      <div className="m-settings__zoom">
        <Button variant="outline" size="sm" icon aria-label="Zoom out" disabled={zoom.level <= 0.5} onClick={() => void change("out")}>
          <Minus className="m-icon m-icon--sm" aria-hidden="true" />
        </Button>
        <output className="m-mono" aria-live="polite" aria-label="Zoom level">
          {pct}
        </output>
        <Button variant="outline" size="sm" icon aria-label="Zoom in" disabled={zoom.level >= 3} onClick={() => void change("in")}>
          <Plus className="m-icon m-icon--sm" aria-hidden="true" />
        </Button>
        <Button variant="ghost" size="sm" disabled={zoom.level === 1} onClick={() => void change("reset")}>
          Reset
        </Button>
      </div>
    </Row>
  );
}

function Keys({ combo }: { combo: string }) {
  // "g g" is a sequence: two presses.
  return (
    <span className="m-settings__keys">
      {combo.split(" ").map((k, i) => (
        <Kbd key={i}>{keyLabel(k)}</Kbd>
      ))}
    </span>
  );
}

function KeyboardHelp() {
  const { zoom } = useDesktopZoom();
  return (
    <>
      <h2 className="m-section-title">Keyboard</h2>
      <DialogDescription className="m-help" style={{ margin: 0 }}>
        Everything in Mesa works from the keyboard. The object tree moves like a vim buffer; the search lists use the arrows or Ctrl with n/p or j/k.
      </DialogDescription>
      {SHORTCUTS.map((g) => (
        <section key={g.group} aria-label={g.group}>
          <h3 className="m-settings__group">{g.group}</h3>
          <dl className="m-settings__shortcuts">
            {g.items
              .filter((s) => !s.nativeZoom || zoom?.native)
              .map((s) => (
                <div key={s.label} className="m-settings__shortcut">
                  <dt>{s.label}</dt>
                  <dd>
                    {s.keys.map((k, i) => (
                      <React.Fragment key={k}>
                        {i > 0 ? <span className="m-muted"> or </span> : null}
                        <Keys combo={k} />
                      </React.Fragment>
                    ))}
                  </dd>
                </div>
              ))}
          </dl>
        </section>
      ))}
    </>
  );
}

function About() {
  const info = useGetSystemInfo();
  return (
    <>
      <h2 className="m-section-title">About</h2>
      <Row label="Version">
        <span className="m-mono">{info.data?.version ?? "…"}</span>
      </Row>
      <Row label="Running as">
        <span>{info.data ? (info.data.mode === "desktop" ? "Desktop app" : info.data.mode === "server" ? "Server" : info.data.mode) : "…"}</span>
      </Row>
      <Row label="Engines">
        <span>{info.data ? info.data.engines.map((e) => engineLabel(e.name as Engine)).join(", ") : "…"}</span>
      </Row>
      <Row label="Source" help="Issues and releases.">
        <a className="m-btn m-btn--outline m-btn--sm" href="https://github.com/felipemalacarne/mesa" target="_blank" rel="noreferrer">
          GitHub
        </a>
      </Row>
    </>
  );
}
