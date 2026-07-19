import { Button } from "@fluentui/react-components";
import { Globe20Filled, Play20Filled, Stop20Filled } from "@fluentui/react-icons";
import type { LauncherPresentationState } from "@shared/launcher-presentation";

import { serviceStateConfig } from "./AppShell.shared";
import { RayleaMark } from "./RayleaMark";

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

const serviceStateHints: Partial<Record<LauncherPresentationState, string>> = {
  stopped: "服务尚未启动",
  starting: "服务正在启动",
  running: "服务运行正常",
  degraded: "服务部分受限",
  setup_required: "等待完成初始化",
  stopping: "服务正在停止",
  failed: "服务启动失败",
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
  const stateConfig = serviceStateConfig[snapshot.serviceState];
  const tone = stateConfig?.tone ?? "neutral";
  const stateLabel = stateConfig?.label ?? "未知";
  const stateHint = serviceStateHints[snapshot.serviceState] ?? "服务状态";

  return (
    <section className="service-control" data-tone={tone} aria-labelledby="service-control-title">
      <div className="service-control__summary">
        <div className="service-control__eyebrow">
          <span className="section-kicker">服务控制</span>
          <span className="service-control__state-chip" data-tone={tone}>
            {stateLabel}
          </span>
        </div>
        <div className="service-control__state" aria-live="polite">
          <RayleaMark className="service-state-mark" tone={tone} variant="neutral" />
          <div className="service-control__state-copy">
            <span className="service-control__state-hint">{stateHint}</span>
            <h2 id="service-control-title">{stateLabel}</h2>
          </div>
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
        <Button
          appearance="primary"
          className="service-control__primary"
          onClick={onStart}
          disabled={startDisabled}
          icon={<Play20Filled />}
        >
          {primaryActionLabel}
        </Button>
        <div className="service-control__secondary">
          <Button appearance="secondary" className="danger-outline-button" onClick={onStop} disabled={stopDisabled} icon={<Stop20Filled />}>停止服务</Button>
          <Button appearance="subtle" onClick={onOpenWeb} disabled={controlsDisabled || !canOpenWebUi} icon={<Globe20Filled />}>管理界面</Button>
        </div>
        {busyLabel || !canOpenWebUi ? (
          <p className="operation-status" aria-live="polite">
            {busyLabel || "服务启动后可进入管理界面"}
          </p>
        ) : null}
      </div>
    </section>
  );
}
