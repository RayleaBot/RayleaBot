import { Button } from "@fluentui/react-components";
import {
  ArrowSync20Regular,
  ArrowSync24Filled,
  Checkmark24Filled,
  Dismiss24Filled,
  Globe20Regular,
  Important24Filled,
  Play20Regular,
  Power24Filled,
  Stop20Regular,
} from "@fluentui/react-icons";
import type { LauncherPresentationState } from "@shared/launcher-presentation";
import { useId, type ReactNode } from "react";

import { serviceStateConfig } from "./AppShell.shared";
import { StatusLens } from "./StatusLens";

type AppShellServiceControlProps = {
  attention: {
    label: string;
    text: string;
    tone: "attention" | "warning" | "danger";
  } | null;
  busyLabel: string;
  canOpenWebUi: boolean;
  controlsDisabled: boolean;
  externalService: boolean;
  onOpenWeb: () => void;
  onStart: () => void;
  onStop: () => void;
  primaryActionLabel: string;
  showRunningActions: boolean;
  snapshot: {
    serviceDetail: string;
    serviceState: LauncherPresentationState;
  };
  startDisabled: boolean;
  stopDisabled: boolean;
};

const serviceStateGlyphs: Record<LauncherPresentationState, ReactNode> = {
  stopped: <Power24Filled />,
  starting: <ArrowSync24Filled />,
  running: <Checkmark24Filled />,
  degraded: <Important24Filled />,
  stopping: <ArrowSync24Filled />,
  failed: <Dismiss24Filled />,
};

export function AppShellServiceControl({
  attention,
  busyLabel,
  canOpenWebUi,
  controlsDisabled,
  externalService,
  onOpenWeb,
  onStart,
  onStop,
  primaryActionLabel,
  showRunningActions,
  snapshot,
  startDisabled,
  stopDisabled,
}: AppShellServiceControlProps) {
  const noteId = useId();
  const stateConfig = serviceStateConfig[snapshot.serviceState];
  const tone = stateConfig?.tone ?? "neutral";
  const stateLabel = stateConfig?.label ?? "未知";
  const primaryDisabled = showRunningActions ? !canOpenWebUi || controlsDisabled : startDisabled;
  const note = busyLabel
    || (!showRunningActions
      ? "服务启动后可进入管理界面"
      : externalService
        ? "服务由其他进程启动，无法在启动器中重启"
        : "");

  return (
    <section className="service-control" data-tone={tone} aria-labelledby="service-control-title">
      <div className="service-control__summary">
        <div className="service-control__state" aria-live="polite">
          <StatusLens tone={tone} icon={serviceStateGlyphs[snapshot.serviceState] ?? <Power24Filled />} />
          <div className="service-control__state-copy">
            <h2 id="service-control-title" className="service-control__state-value">
              <span className="visually-hidden">服务控制：</span>
              <span>{stateLabel}</span>
            </h2>
            <p className="service-control__detail">{snapshot.serviceDetail}</p>
          </div>
        </div>
        {attention ? (
          <div className="attention-note" data-severity={attention.tone}>
            <span className="attention-note__label">{attention.label}</span>
            <span>{attention.text}</span>
          </div>
        ) : null}
      </div>

      <div className="service-control__actions">
        <Button appearance="secondary" className="glass glass-button glass-button--danger" data-glass="regular" onClick={onStop} disabled={stopDisabled} icon={<Stop20Regular />}>停止服务</Button>
        {showRunningActions ? (
          <Button
            appearance="secondary"
            className="glass glass-button"
            data-glass="regular"
            onClick={onStart}
            disabled={startDisabled}
            aria-describedby={externalService ? noteId : undefined}
            icon={<ArrowSync20Regular />}
          >
            重启服务
          </Button>
        ) : (
          <Button appearance="secondary" className="glass glass-button" data-glass="regular" onClick={onOpenWeb} disabled icon={<Globe20Regular />}>管理界面</Button>
        )}
        <Button
          appearance="primary"
          className="service-control__primary glass"
          data-glass={primaryDisabled ? "regular" : "prominent"}
          onClick={showRunningActions ? onOpenWeb : onStart}
          disabled={primaryDisabled}
          icon={showRunningActions ? <Globe20Regular /> : <Play20Regular />}
        >
          {showRunningActions ? "管理界面" : primaryActionLabel}
        </Button>
        {note ? (
          <p id={noteId} className="operation-status" aria-live="polite">
            {note}
          </p>
        ) : null}
      </div>
    </section>
  );
}
