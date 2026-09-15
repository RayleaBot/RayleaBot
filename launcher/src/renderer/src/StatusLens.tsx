import type { ReactNode } from "react";

import type { LauncherVisualTone } from "./AppShell.shared";

type StatusLensProps = {
  tone: LauncherVisualTone;
  icon: ReactNode;
  size?: "regular" | "small";
};

export function StatusLens({ tone, icon, size = "regular" }: StatusLensProps) {
  return (
    <span className={size === "small" ? "status-lens status-lens--small" : "status-lens"} data-tone={tone} aria-hidden="true">
      <span className="status-lens__core" />
      <span className="status-lens__glass glass" data-glass="clear">{icon}</span>
    </span>
  );
}
