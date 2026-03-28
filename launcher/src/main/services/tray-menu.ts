import { launcherCopy } from "../../shared/launcher-copy";
import type { TrayMenuEntry, TrayMenuState } from "../../shared/launcher-models";

export function buildTrayMenuEntries(state: TrayMenuState): TrayMenuEntry[] {
  return [
    { label: launcherCopy.trayTitleLabel, enabled: false, action: null },
    { label: `状态：${state.trayStatusSummary}`, enabled: false, action: null },
    { label: "", enabled: false, action: "separator" },
    { label: launcherCopy.restoreLauncherLabel, enabled: true, action: "restore" },
    { label: launcherCopy.openWebUiLabel, enabled: state.canOpenWebUi, action: "open_web" },
    {
      label: state.trayServiceActionLabel,
      enabled: state.canRunTrayServiceAction,
      action: state.trayServiceAction === "start" ? "start" : "stop",
    },
    { label: "", enabled: false, action: "separator" },
    { label: launcherCopy.trayOpenLogsLabel, enabled: true, action: "open_logs" },
    { label: "", enabled: false, action: "separator" },
    { label: launcherCopy.exitAppLabel, enabled: true, action: "exit" },
  ];
}
