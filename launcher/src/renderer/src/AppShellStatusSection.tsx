import { Button, PresenceBadge, Text } from "@fluentui/react-components";
import {
  DocumentText20Filled,
  FolderOpen20Filled,
  Globe20Filled,
  Play20Filled,
  Status20Filled,
  Stop20Filled,
} from "@fluentui/react-icons";
import { useEffect, useMemo, useRef, useState } from "react";
import type { LauncherResolvedSettings, LauncherSnapshot } from "@shared/launcher-models";

import { busyActionLabels, severityConfig, serviceStateConfig, sortChecks } from "./AppShell.shared";

type StatusSectionProps = {
  snapshot: LauncherSnapshot;
  resolvedSettings: LauncherResolvedSettings;
  busyAction: string | null;
  controlsDisabled: boolean;
  onStart: () => void;
  onStop: () => void;
  onOpenWeb: () => void;
  onRecoveryRecheck: () => void;
  onRuntimeBootstrap: () => void;
  onOpenReleasePage: () => void;
  onOpenLogs: () => void;
};

export function AppShellStatusSection({
  snapshot,
  resolvedSettings,
  busyAction,
  controlsDisabled,
  onStart,
  onStop,
  onOpenWeb,
  onRecoveryRecheck,
  onRuntimeBootstrap,
  onOpenReleasePage,
  onOpenLogs,
}: StatusSectionProps) {
  const [statusHighlight, setStatusHighlight] = useState<"none" | "signal" | "alert">("none");
  const [logHighlight, setLogHighlight] = useState<"none" | "fresh">("none");

  const checks = useMemo(() => sortChecks(snapshot.environmentChecks || []), [snapshot.environmentChecks]);
  const nonOkChecks = useMemo(() => checks.filter((item) => item.severity !== "ok"), [checks]);
  const primaryEnvironmentIssue = nonOkChecks[0] ?? null;
  const recoveryStatusSummary = snapshot.recoverySummary
    ? `${snapshot.recoverySummary.status} · ${snapshot.recoverySummary.operation}`
    : "当前没有恢复摘要。";
  const hasRecentStderr = snapshot.recentStderr.length > 0;
  const statusAlert = snapshot.lastError ? "error" : nonOkChecks.length > 0 ? "warning" : "none";
  const logAlert = hasRecentStderr ? "error" : "none";
  const statusReasonLabel =
    snapshot.serviceState === "degraded" || snapshot.serviceState === "setup_required"
      ? "当前限制"
      : "运行说明";
  const statusReasonText =
    snapshot.serviceState === "degraded" || snapshot.serviceState === "setup_required"
      ? snapshot.serviceDetail
      : primaryEnvironmentIssue
        ? `${primaryEnvironmentIssue.title}：${primaryEnvironmentIssue.summary}`
        : snapshot.serviceDetail;
  const statusGuidanceLabel = snapshot.lastError ? "异常提示" : primaryEnvironmentIssue ? "处理提示" : "异常提示";
  const statusGuidanceText =
    snapshot.lastError
    || primaryEnvironmentIssue?.remediation
    || primaryEnvironmentIssue?.detail
    || "当前没有阻塞异常。";
  const isManagedRunnable =
    (snapshot.serviceState === "running" || snapshot.serviceState === "degraded")
    && snapshot.serviceOwnership === "launcher_managed";
  const isExternalRunnable =
    (snapshot.serviceState === "running" || snapshot.serviceState === "degraded")
    && snapshot.serviceOwnership === "external";
  const canOpenWebUi =
    snapshot.serviceState === "running"
    || snapshot.serviceState === "degraded"
    || snapshot.serviceState === "setup_required";
  const canRunRecoveryActions =
    (snapshot.serviceState === "running" || snapshot.serviceState === "degraded")
    && !controlsDisabled;
  const canRecheckRecovery = canRunRecoveryActions && Boolean(snapshot.recoverySummary);
  const primaryActionLabel =
    isExternalRunnable
      ? "检测到现有服务"
      : isManagedRunnable
        ? "重启服务"
        : snapshot.serviceState === "setup_required"
          ? "打开初始化"
          : "启动 RayleaBot";
  const startDisabled =
    controlsDisabled
    || busyAction === "start"
    || busyAction === "restart"
    || busyAction === "stop"
    || busyAction === "open-web"
    || isExternalRunnable
    || snapshot.serviceState === "starting"
    || snapshot.serviceState === "stopping";
  const stopDisabled =
    controlsDisabled
    || busyAction === "restart"
    || busyAction === "stop"
    || snapshot.serviceState === "starting"
    || snapshot.serviceState === "stopping"
    || snapshot.serviceOwnership === "none";
  const busyLabel = busyAction ? (busyActionLabels[busyAction] ?? "正在执行操作") : "";

  const previousStatusRef = useRef({
    serviceState: snapshot.serviceState,
    busyAction,
    lastError: snapshot.lastError,
  });
  const previousLogsRef = useRef(snapshot.recentStderr.join("\n"));

  useEffect(() => {
    const previous = previousStatusRef.current;
    const serviceStateChanged = previous.serviceState !== snapshot.serviceState;
    const actionChanged = previous.busyAction !== busyAction && busyAction !== null;
    const errorChanged = previous.lastError !== snapshot.lastError && Boolean(snapshot.lastError);

    previousStatusRef.current = {
      serviceState: snapshot.serviceState,
      busyAction,
      lastError: snapshot.lastError,
    };

    if (!(serviceStateChanged || actionChanged || errorChanged)) {
      return;
    }

    setStatusHighlight(errorChanged ? "alert" : "signal");
    const timeoutId = window.setTimeout(() => {
      setStatusHighlight("none");
    }, 1200);

    return () => window.clearTimeout(timeoutId);
  }, [snapshot.serviceState, snapshot.lastError, busyAction]);

  useEffect(() => {
    const nextLogState = snapshot.recentStderr.join("\n");
    const hadLogs = previousLogsRef.current.length > 0;
    const hasLogsNow = nextLogState.length > 0;
    previousLogsRef.current = nextLogState;

    if (!hasLogsNow || hadLogs === hasLogsNow) {
      return;
    }

    setLogHighlight("fresh");
    const timeoutId = window.setTimeout(() => {
      setLogHighlight("none");
    }, 1600);

    return () => window.clearTimeout(timeoutId);
  }, [snapshot.recentStderr]);

  return (
    <div className="status-homepage status-view-flow" data-state={snapshot.serviceState} data-busy={busyAction ?? "idle"} data-alert={statusAlert}>
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
            <div className={`hero-context-card ${snapshot.lastError || primaryEnvironmentIssue ? "hero-context-card--alert" : ""}`}>
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
          <div className="status-action-feedback" data-busy={busyAction ? "true" : "false"}>
            <span className="status-action-feedback__dot" aria-hidden="true"></span>
            <span>{busyLabel || "当前没有进行中的操作。"}</span>
          </div>
        </div>
      </section>

      <div className="status-summary-grid status-grid">
        <div className="status-summary-main status-main-column">
          <article className="panel glass-panel panel--interactive">
            <div className="brand-eyebrow">核心参数</div>
            <div className="status-list status-list--grid-modern">
              <div className="status-item-modern">
                <div className="status-item-modern__icon"><Status20Filled /></div>
                <div className="status-item-modern__content">
                  <span className="status-label">进程 ID</span>
                  <code className="status-value status-value--highlight">{snapshot.processId ?? "—"}</code>
                </div>
              </div>
              <div className="status-item-modern">
                <div className="status-item-modern__icon"><Globe20Filled /></div>
                <div className="status-item-modern__content">
                  <span className="status-label">本地访问地址</span>
                  <span className="status-value mono">{snapshot.endpoint.baseUrl}</span>
                </div>
              </div>
              <div className="status-item-modern status-item-modern--full">
                <div className="status-item-modern__icon"><FolderOpen20Filled /></div>
                <div className="status-item-modern__content">
                  <span className="status-label">安装目录</span>
                  <span className="status-value mono" title={snapshot.settings.installationRoot}>{snapshot.settings.installationRoot || "—"}</span>
                </div>
              </div>
              <div className="status-item-modern status-item-modern--full">
                <div className="status-item-modern__icon"><DocumentText20Filled /></div>
                <div className="status-item-modern__content">
                  <span className="status-label">运行目录</span>
                  <span className="status-value mono" title={resolvedSettings.workdir}>{resolvedSettings.workdir || "—"}</span>
                </div>
              </div>
            </div>
          </article>
        </div>

        <aside className="status-summary-rail status-side-column">
          {nonOkChecks.length > 0 && (
            <div className="checks-stack checks-stack--side panel glass-panel glass-panel--subtle panel--side">
              <div className="brand-eyebrow brand-eyebrow--tight">环境预警</div>
              {nonOkChecks.map((item) => (
                <div key={item.code} className={`check-item-mini check-item-mini--${item.severity}`}>
                  <div className="check-item-mini__icon">{severityConfig[item.severity as keyof typeof severityConfig]?.icon}</div>
                  <div className="check-item-mini__content">
                    <Text weight="bold" size={200}>{item.title}</Text>
                    <Text size={100} className="panel-muted">
                      {item.code === "os.long_paths_unknown" && item.severity === "warning"
                        ? "无法确认长路径支持状态。若资源展开遇到限制，请手动检查系统长路径设置。"
                        : item.summary}
                    </Text>
                    <Text size={100} className="panel-muted">{severityConfig[item.severity as keyof typeof severityConfig]?.label}</Text>
                  </div>
                </div>
              ))}
            </div>
          )}

          <article className="panel glass-panel glass-panel--subtle panel--side">
            <div className="brand-eyebrow brand-eyebrow--tight">恢复兼容性</div>
            <Text size={200} className="panel-muted">{recoveryStatusSummary}</Text>
            <div className="side-actions-stack">
              <Button appearance="transparent" size="small" className="frost-button frost-button--secondary frost-button--block" onClick={onRecoveryRecheck} disabled={!canRecheckRecovery}>重新检查</Button>
              <Button appearance="transparent" size="small" className="frost-button frost-button--secondary frost-button--block" onClick={onRuntimeBootstrap} disabled={!canRunRecoveryActions}>准备运行环境</Button>
            </div>
          </article>

          <article className="panel glass-panel glass-panel--subtle panel--side">
            <div className="brand-eyebrow brand-eyebrow--tight">版本监控</div>
            <div className="version-status">
              <Text size={200} className="panel-muted">{snapshot.releaseCheck.summary}</Text>
            </div>
            <Button appearance="transparent" size="small" className="frost-button frost-button--secondary frost-button--block frost-button--outline" onClick={onOpenReleasePage}>检查更新</Button>
          </article>
        </aside>
      </div>

      <article className="status-log-panel panel glass-panel" data-alert={logAlert} data-highlight={logHighlight}>
        <div className="panel-header-row">
          <div className="brand-eyebrow">异常输出监控</div>
          <span className={`status-log-indicator status-log-indicator--${hasRecentStderr ? "alert" : "quiet"}`}>
            {hasRecentStderr ? "已检测到异常输出" : "当前无新异常"}
          </span>
        </div>
        {hasRecentStderr ? (
          <pre className="log-surface status-log-surface--modern">{snapshot.recentStderr.join("\n")}</pre>
        ) : (
          <div className="log-empty-state">
            <div className="log-empty-state__title">当前没有新的异常日志</div>
            <Text size={200} className="panel-muted">服务输出保持安静，完整日志仍可随时打开。</Text>
          </div>
        )}
        <div className="panel-footer-actions">
          <Button appearance="transparent" size="small" className="frost-button frost-button--ghost-bright" onClick={onOpenLogs} icon={<FolderOpen20Filled />}>查看完整日志</Button>
        </div>
      </article>
    </div>
  );
}
