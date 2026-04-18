import { useEffect, useMemo, useRef, useState } from "react";
import type { LauncherResolvedSettings, LauncherSnapshot } from "@shared/launcher-models";

import { busyActionLabels, sortChecks } from "./AppShell.shared";
import { AppShellStatusHero } from "./AppShellStatusHero";
import { AppShellStatusLogs } from "./AppShellStatusLogs";
import { AppShellStatusRail } from "./AppShellStatusRail";
import { AppShellStatusSummary } from "./AppShellStatusSummary";

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

  const readiness = snapshot.readiness ?? null;
  const checks = useMemo(() => sortChecks(snapshot.environmentChecks || []), [snapshot.environmentChecks]);
  const nonOkChecks = useMemo(() => checks.filter((item) => item.severity !== "ok"), [checks]);
  const readinessIssues = readiness?.issues ?? [];
  const readinessReason = readiness?.reason?.trim() ?? "";
  const readinessReasonCodes = readiness?.reason_codes ?? [];
  const nonOkReadinessChecks = Object.entries(readiness?.checks ?? {}).filter(([, value]) => value && value !== "ok");
  const primaryReadinessIssue = readinessIssues[0] ?? null;
  const primaryEnvironmentIssue = nonOkChecks[0] ?? null;
  const recoveryStatusSummary = snapshot.recoverySummary
    ? `${snapshot.recoverySummary.status} · ${snapshot.recoverySummary.operation}`
    : "当前没有恢复摘要。";
  const hasRecentStderr = snapshot.recentStderr.length > 0;
  const statusAlert =
    snapshot.lastError
      ? "error"
      : primaryReadinessIssue
        ? primaryReadinessIssue.severity === "error" ? "error" : "warning"
        : readinessReason
          ? snapshot.serviceState === "failed" ? "error" : "warning"
          : nonOkChecks.length > 0
            ? "warning"
          : "none";
  const logAlert = hasRecentStderr ? "error" : "none";
  const statusReasonLabel =
    snapshot.serviceState === "degraded"
      || snapshot.serviceState === "setup_required"
      || snapshot.serviceState === "failed"
      || Boolean(readinessReason || primaryReadinessIssue)
      ? "当前限制"
      : "运行说明";
  const statusReasonText =
    readinessReason
    || primaryReadinessIssue?.summary
    || (snapshot.serviceState === "degraded" || snapshot.serviceState === "setup_required" || snapshot.serviceState === "failed"
      ? snapshot.serviceDetail
      : primaryEnvironmentIssue
        ? `${primaryEnvironmentIssue.title}：${primaryEnvironmentIssue.summary}`
        : snapshot.serviceDetail);
  const statusGuidanceLabel =
    snapshot.lastError
      ? "异常提示"
      : primaryReadinessIssue?.remediation || primaryEnvironmentIssue
        ? "处理提示"
        : "异常提示";
  const statusGuidanceText =
    snapshot.lastError
    || primaryReadinessIssue?.remediation
    || primaryEnvironmentIssue?.remediation
    || primaryEnvironmentIssue?.detail
    || "当前没有阻塞异常。";
  const hasStatusAlert = Boolean(snapshot.lastError || readinessReason || primaryReadinessIssue || primaryEnvironmentIssue);
  const hasReadinessDiagnostics = Boolean(
    readinessReason || readinessReasonCodes.length || readinessIssues.length || nonOkReadinessChecks.length,
  );
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
      <AppShellStatusHero
        busyLabel={busyLabel}
        canOpenWebUi={canOpenWebUi}
        controlsDisabled={controlsDisabled}
        hasStatusAlert={hasStatusAlert}
        onOpenWeb={onOpenWeb}
        onStart={onStart}
        onStop={onStop}
        primaryActionLabel={primaryActionLabel}
        snapshot={snapshot}
        startDisabled={startDisabled}
        statusGuidanceLabel={statusGuidanceLabel}
        statusGuidanceText={statusGuidanceText}
        statusHighlight={statusHighlight}
        statusReasonLabel={statusReasonLabel}
        statusReasonText={statusReasonText}
        stopDisabled={stopDisabled}
      />

      <div className="status-summary-grid status-grid">
        <div className="status-summary-main status-main-column">
          {hasReadinessDiagnostics ? (
            <article className="panel glass-panel glass-panel--subtle status-diagnostics-panel">
              <div className="brand-eyebrow">服务诊断</div>
              {readinessReason ? (
                <div className="status-diagnostics-lead">{readinessReason}</div>
              ) : null}

              {readinessReasonCodes.length > 0 ? (
                <div className="status-diagnostics-block">
                  <span className="status-label">原因代码</span>
                  <div className="status-diagnostics-codes">
                    {readinessReasonCodes.map((code) => (
                      <code key={code} className="glass-chip glass-chip--muted mono">{code}</code>
                    ))}
                  </div>
                </div>
              ) : null}

              {readinessIssues.length > 0 ? (
                <div className="status-diagnostics-block">
                  <span className="status-label">首要问题</span>
                  <div className="status-diagnostics-list">
                    {readinessIssues.slice(0, 3).map((issue) => (
                      <div
                        key={`${issue.code}-${issue.summary}`}
                        className={`status-diagnostics-item status-diagnostics-item--${issue.severity}`}
                      >
                        <div className="status-diagnostics-item__header">
                          <span className="status-diagnostics-item__summary">{issue.summary}</span>
                          <span className={`status-pill status-pill--${issue.severity === "error" ? "error" : "warning"}`}>
                            {issue.severity === "error" ? "阻塞" : "警告"}
                          </span>
                        </div>
                        <code className="status-diagnostics-item__code mono">{issue.code}</code>
                        {issue.remediation ? (
                          <div className="status-diagnostics-item__remediation">{issue.remediation}</div>
                        ) : null}
                      </div>
                    ))}
                  </div>
                </div>
              ) : null}

              {nonOkReadinessChecks.length > 0 ? (
                <div className="status-diagnostics-block">
                  <span className="status-label">检查项</span>
                  <div className="status-diagnostics-checks">
                    {nonOkReadinessChecks.map(([name, value]) => (
                      <div key={`${name}-${value}`} className="status-diagnostics-check">
                        <span className="status-diagnostics-check__name">{name}</span>
                        <span className="status-diagnostics-check__value mono">{value}</span>
                      </div>
                    ))}
                  </div>
                </div>
              ) : null}
            </article>
          ) : null}

          <AppShellStatusSummary snapshot={snapshot} resolvedSettings={resolvedSettings} />
        </div>

        <AppShellStatusRail
          canRecheckRecovery={canRecheckRecovery}
          canRunRecoveryActions={canRunRecoveryActions}
          checks={nonOkChecks}
          onOpenReleasePage={onOpenReleasePage}
          onRecoveryRecheck={onRecoveryRecheck}
          onRuntimeBootstrap={onRuntimeBootstrap}
          recoveryStatusSummary={recoveryStatusSummary}
          releaseSummary={snapshot.releaseCheck.summary}
        />
      </div>

      <AppShellStatusLogs
        hasRecentStderr={hasRecentStderr}
        logAlert={logAlert}
        logHighlight={logHighlight}
        logs={snapshot.recentStderr}
        onOpenLogs={onOpenLogs}
      />
    </div>
  );
}
