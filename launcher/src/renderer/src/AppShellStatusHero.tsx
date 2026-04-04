import { Button, PresenceBadge, Text } from "@fluentui/react-components";
import { Globe20Filled, Play20Filled, Stop20Filled } from "@fluentui/react-icons";

import { serviceStateConfig } from "./AppShell.shared";

type AppShellStatusHeroProps = {
  busyLabel: string;
  canOpenWebUi: boolean;
  controlsDisabled: boolean;
  hasStatusAlert: boolean;
  onOpenWeb: () => void;
  onStart: () => void;
  onStop: () => void;
  primaryActionLabel: string;
  snapshot: {
    lastError: string;
    serviceDetail: string;
    serviceState: string;
  };
  startDisabled: boolean;
  statusGuidanceLabel: string;
  statusGuidanceText: string;
  statusHighlight: "none" | "signal" | "alert";
  statusReasonLabel: string;
  statusReasonText: string;
  stopDisabled: boolean;
};

export function AppShellStatusHero({
  busyLabel,
  canOpenWebUi,
  controlsDisabled,
  hasStatusAlert,
  onOpenWeb,
  onStart,
  onStop,
  primaryActionLabel,
  snapshot,
  startDisabled,
  statusGuidanceLabel,
  statusGuidanceText,
  statusHighlight,
  statusReasonLabel,
  statusReasonText,
  stopDisabled,
}: AppShellStatusHeroProps) {
  return (
    <section className="status-hero glass-panel hero-card hero-card--fancy" data-highlight={statusHighlight}>
      <div className="status-hero__body hero-copy">
        <div className="brand-eyebrow brand-eyebrow--faded">Service Control</div>
        <div className="hero-status-row hero-status-row--main">
          <div className="hero-status-indicator">
            <PresenceBadge status={serviceStateConfig[snapshot.serviceState]?.status ?? "unknown"} size="extra-large" />
            <div className={`hero-status-glow hero-status-glow--${serviceStateConfig[snapshot.serviceState]?.status}`} />
          </div>
          <div className="hero-status-content">
            <Text weight="bold" size={800} className="hero-status-text hero-status-text--huge">{serviceStateConfig[snapshot.serviceState]?.label ?? "未知"}</Text>
            <Text size={300} className="hero-detail hero-detail--bright">{snapshot.serviceDetail}</Text>
          </div>
        </div>

        <div className="hero-context-grid">
          <div className="hero-context-card">
            <span className="hero-context-card__label">当前状态</span>
            <span className="hero-context-card__value">{serviceStateConfig[snapshot.serviceState]?.label ?? "未知"}</span>
          </div>
          <div className="hero-context-card">
            <span className="hero-context-card__label">{statusReasonLabel}</span>
            <span className="hero-context-card__value">{statusReasonText}</span>
          </div>
          <div className={`hero-context-card ${hasStatusAlert ? "hero-context-card--alert" : ""}`}>
            <span className="hero-context-card__label">{statusGuidanceLabel}</span>
            <span className="hero-context-card__value">{statusGuidanceText}</span>
          </div>
        </div>
      </div>

      <div className="status-hero__actions hero-actions hero-actions--premium">
        <div className="status-hero__primary-action">
          <Button appearance="transparent" size="large" className="frost-button frost-button--primary status-action status-action--primary" onClick={onStart} disabled={startDisabled} icon={<Play20Filled />}>
            <span className="button-text-large">{primaryActionLabel}</span>
          </Button>
        </div>
        <div className="status-hero__secondary-actions hero-actions-row">
          <Button appearance="transparent" size="large" className="frost-button frost-button--secondary status-action" onClick={onStop} disabled={stopDisabled} icon={<Stop20Filled />}>停止服务</Button>
          <Button appearance="transparent" size="large" className="frost-button frost-button--secondary status-action" onClick={onOpenWeb} disabled={controlsDisabled || !canOpenWebUi} icon={<Globe20Filled />}>管理面板</Button>
        </div>
        <div className="status-action-feedback" data-busy={busyLabel ? "true" : "false"}>
          <span className="status-action-feedback__dot" aria-hidden="true"></span>
          <span>{busyLabel || "当前没有进行中的操作。"}</span>
        </div>
      </div>
    </section>
  );
}
