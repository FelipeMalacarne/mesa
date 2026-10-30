import * as React from "react";

/** One setting: its label and help on the left, the control on the right. */
export function Row({ label, help, children }: { label: string; help?: React.ReactNode; children: React.ReactNode }) {
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
