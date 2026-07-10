import { Button } from "@fluentui/react-components";
import { Globe20Filled, Play20Filled, Stop20Filled } from "@fluentui/react-icons";
import type { LauncherPresentationState } from "@shared/launcher-presentation";

import { serviceStateConfig } from "./AppShell.shared";

type AppShellServiceControlProps = {
  attention: {
    label: string;
    text: string;
    tone: "attention" | "warning" | "danger";
  } | null;
  busyLabel: string;
  canOpenWebUi: boolean;
  controlsDisabled: boolean;
  onOpenWeb: () => void;
  onStart: () => void;
  onStop: () => void;
  primaryActionLabel: string;
  snapshot: {
    serviceDetail: string;
    serviceState: LauncherPresentationState;
  };
  startDisabled: boolean;
  stopDisabled: boolean;
};

export function AppShellServiceControl({
  attention,
  busyLabel,
  canOpenWebUi,
  controlsDisabled,
  onOpenWeb,
  onStart,
  onStop,
  primaryActionLabel,
  snapshot,
  startDisabled,
  stopDisabled,
}: AppShellServiceControlProps) {
  return (
    <section className="service-control" aria-labelledby="service-control-title">
      <div className="service-control__summary">
        <div className="section-kicker">服务控制</div>
        <div className="service-control__state" aria-live="polite">
          <span className="service-state-mark" data-tone={serviceStateConfig[snapshot.serviceState]?.tone ?? "neutral"} aria-hidden="true" />
          <h2 id="service-control-title">{serviceStateConfig[snapshot.serviceState]?.label ?? "未知"}</h2>
        </div>
        <p className="service-control__detail">{snapshot.serviceDetail}</p>
        {attention ? (
          <div className="attention-note" data-severity={attention.tone}>
            <span className="attention-note__label">{attention.label}</span>
            <span>{attention.text}</span>
          </div>
        ) : null}
      </div>

      <div className="service-control__actions">
        <Button appearance="primary" onClick={onStart} disabled={startDisabled} icon={<Play20Filled />}>
          {primaryActionLabel}
        </Button>
        <Button appearance="secondary" className="danger-outline-button" onClick={onStop} disabled={stopDisabled} icon={<Stop20Filled />}>停止服务</Button>
        <Button appearance="subtle" onClick={onOpenWeb} disabled={controlsDisabled || !canOpenWebUi} icon={<Globe20Filled />}>管理界面</Button>
        {busyLabel || !canOpenWebUi ? (
          <p className="operation-status" aria-live="polite">
            {busyLabel || "服务启动后可进入管理界面"}
          </p>
        ) : null}
      </div>
    </section>
  );
}
