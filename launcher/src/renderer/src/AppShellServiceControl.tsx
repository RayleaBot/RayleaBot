import { Button } from "@fluentui/react-components";
import {
  ArrowSync24Regular,
  CheckmarkCircle24Regular,
  ErrorCircle24Regular,
  Globe20Regular,
  Play20Regular,
  Power24Regular,
  Stop20Regular,
  Warning24Regular,
} from "@fluentui/react-icons";
import type { LauncherPresentationState } from "@shared/launcher-presentation";
import type { ReactNode } from "react";

import { serviceStateConfig } from "./AppShell.shared";
import { LiquidGlassFilter, useLiquidGlass } from "./liquidGlass";

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

const serviceStateIcons: Record<LauncherPresentationState, ReactNode> = {
  stopped: <Power24Regular />,
  starting: <ArrowSync24Regular />,
  running: <CheckmarkCircle24Regular />,
  degraded: <Warning24Regular />,
  stopping: <ArrowSync24Regular />,
  failed: <ErrorCircle24Regular />,
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
  const glass = useLiquidGlass<HTMLElement>();
  const stateConfig = serviceStateConfig[snapshot.serviceState];
  const tone = stateConfig?.tone ?? "neutral";
  const stateLabel = stateConfig?.label ?? "未知";

  return (
    <section
      ref={glass.surfaceRef}
      className="service-control glass-surface glass-surface--lens"
      data-tone={tone}
      aria-labelledby="service-control-title"
      style={glass.refractionStyle}
      onPointerMove={glass.onPointerMove}
      onPointerLeave={glass.onPointerLeave}
    >
      <LiquidGlassFilter id={glass.filterId} displacement={glass.displacement} />
      <div className="service-control__summary">
        <div className="service-control__state" aria-live="polite">
          <span className="service-state-mark" data-tone={tone} aria-hidden="true">
            {serviceStateIcons[snapshot.serviceState] ?? <Power24Regular />}
          </span>
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
        <Button
          appearance="primary"
          className="service-control__primary"
          onClick={canOpenWebUi ? onOpenWeb : onStart}
          disabled={canOpenWebUi ? controlsDisabled : startDisabled}
          icon={canOpenWebUi ? <Globe20Regular /> : <Play20Regular />}
        >
          {canOpenWebUi ? "管理界面" : primaryActionLabel}
        </Button>
        <div className="service-control__secondary">
          <Button appearance="secondary" className="glass-button glass-button--danger" onClick={onStop} disabled={stopDisabled} icon={<Stop20Regular />}>停止服务</Button>
          {canOpenWebUi ? (
            !startDisabled && <Button appearance="secondary" className="glass-button" onClick={onStart} disabled={controlsDisabled} icon={<Play20Regular />}>{primaryActionLabel}</Button>
          ) : (
            <Button appearance="secondary" className="glass-button" onClick={onOpenWeb} disabled icon={<Globe20Regular />}>管理界面</Button>
          )}
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
