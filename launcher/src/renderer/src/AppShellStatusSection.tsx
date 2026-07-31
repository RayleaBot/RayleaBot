import { useMemo } from "react";
import { deriveLauncherPresentation, resolveRecoverySummary } from "@shared/launcher-presentation";
import type { LauncherResolvedSettings, LauncherSnapshot } from "@shared/launcher-models";

import { busyActionLabels, sortChecks } from "./AppShell.shared";
import { formatRecoverySummary } from "./AppShell.copy";
import { AppShellServiceControl } from "./AppShellServiceControl";
import { AppShellStatusLogs } from "./AppShellStatusLogs";
import { AppShellStatusRail } from "./AppShellStatusRail";
import { AppShellStatusSummary } from "./AppShellStatusSummary";
import { AppShellRuntimePreparePanel } from "./AppShellRuntimePreparePanel";

type StatusSectionProps = {
  snapshot: LauncherSnapshot;
  resolvedSettings: LauncherResolvedSettings;
  busyAction: string | null;
  controlsDisabled: boolean;
  onStart: () => void;
  onStop: () => void;
  onOpenWeb: () => void;
  onOpenRecoveryTasks: () => void;
  onOpenRuntimeTasks: () => void;
  onOpenLogs: () => void;
};

function isRuntimePreparationIssue(code: string) {
  return ["deps.", "chromium.", "python.", "nodejs.", "npm."].some((prefix) => code.startsWith(prefix));
}

export function AppShellStatusSection({
  snapshot,
  resolvedSettings,
  busyAction,
  controlsDisabled,
  onStart,
  onStop,
  onOpenWeb,
  onOpenRecoveryTasks,
  onOpenRuntimeTasks,
  onOpenLogs,
}: StatusSectionProps) {
  const presentation = useMemo(() => deriveLauncherPresentation(snapshot), [snapshot]);
  const recoverySummary = useMemo(() => resolveRecoverySummary(snapshot), [snapshot]);
  const runtimePrepare = snapshot.launcher.runtimePrepare ?? null;
  const readiness = snapshot.server.readiness ?? null;
  const setupRequired = readiness?.status === "setup_required";
  const checks = useMemo(() => sortChecks(snapshot.launcher.preflightChecks || []), [snapshot.launcher.preflightChecks]);
  const nonOkChecks = useMemo(() => checks.filter((item) => item.severity !== "ok"), [checks]);
  const readinessIssues = setupRequired ? [] : readiness?.issues ?? [];
  const readinessReason = setupRequired ? "" : readiness?.reason?.trim() ?? "";
  const readinessReasonCodes = setupRequired ? [] : readiness?.reason_codes ?? [];
  const nonOkReadinessChecks = setupRequired
    ? []
    : Object.entries(readiness?.checks ?? {}).filter(([, value]) => value && value !== "ok");
  const primaryReadinessIssue = setupRequired ? null : readinessIssues[0] ?? null;
  const primaryEnvironmentIssue = nonOkChecks[0] ?? null;
  const recoveryStatusSummary = formatRecoverySummary(recoverySummary);
  const hasRecentStderr = snapshot.launcher.recentStderr.length > 0;
  const statusAlert =
    snapshot.launcher.lastLocalError
      ? "error"
      : primaryReadinessIssue
        ? primaryReadinessIssue.severity === "error" ? "error" : "warning"
        : readinessReason
          ? presentation.state === "failed" ? "error" : "warning"
          : nonOkChecks.length > 0
            ? "warning"
          : "none";
  const logAlert = hasRecentStderr ? "error" : "none";
  const statusReasonText =
    runtimePrepare?.active
      ? (runtimePrepare.summary || "正在准备运行环境。")
      : readinessReason
    || primaryReadinessIssue?.summary
    || (presentation.state === "degraded" || presentation.state === "failed"
      ? presentation.detail
      : primaryEnvironmentIssue
        ? `${primaryEnvironmentIssue.title}：${primaryEnvironmentIssue.summary}`
        : presentation.detail);
  const serviceControlDetail =
    runtimePrepare?.active
      ? "运行环境准备中。"
      : snapshot.launcher.lastLocalError
      ? "启动器检测到本地异常。"
      : presentation.state === "degraded"
        ? "服务可运行，部分能力受限。"
        : presentation.state === "failed"
            ? "服务未通过就绪检查。"
            : presentation.detail;
  const hasReadinessDiagnostics = !runtimePrepare?.active && Boolean(
    readinessReasonCodes.length
      || readinessIssues.some((issue) => issue.remediation)
      || nonOkReadinessChecks.length,
  );
  const canOpenWebUi = presentation.canOpenWebUi;
  const canRunRecoveryActions = presentation.canRunRecoveryActions && !controlsDisabled;
  const canRecheckRecovery = canRunRecoveryActions && presentation.canRecheckRecovery;
  const canPrepareRuntime = canRunRecoveryActions && nonOkChecks.some((item) => isRuntimePreparationIssue(item.code));
  const showRecoveryPanel = Boolean(recoverySummary) || canPrepareRuntime;
  const showStatusRail = nonOkChecks.length > 0 || showRecoveryPanel;
  const startDisabled =
    controlsDisabled
    || busyAction === "start"
    || busyAction === "restart"
    || busyAction === "stop"
    || busyAction === "open-web"
    || ((presentation.state === "running" || presentation.state === "degraded")
      && snapshot.launcher.processOwnership === "external")
    || presentation.state === "starting"
    || presentation.state === "stopping";
  const stopDisabled =
    controlsDisabled
    || busyAction === "restart"
    || busyAction === "stop"
    || presentation.state === "starting"
    || presentation.state === "stopping"
    || snapshot.launcher.processOwnership === "none";
  const busyLabel = busyAction ? (busyActionLabels[busyAction] ?? "正在执行操作") : "";
  const serviceAttention = (() => {
    if (runtimePrepare?.active) {
      return { label: "准备进度", text: statusReasonText, tone: "attention" as const };
    }
    if (snapshot.launcher.lastLocalError || presentation.state === "failed") {
      return { label: "当前限制", text: statusReasonText, tone: "danger" as const };
    }
    if (presentation.state === "degraded") {
      return { label: "当前限制", text: statusReasonText, tone: "warning" as const };
    }
    if (readinessReason || primaryReadinessIssue) {
      return {
        label: "当前限制",
        text: statusReasonText,
        tone: statusAlert === "error" ? "danger" as const : "warning" as const,
      };
    }
    return null;
  })();

  return (
    <div className="status-homepage status-view-flow" data-state={presentation.state} data-busy={busyAction ?? "idle"} data-alert={statusAlert}>
      <AppShellServiceControl
        attention={serviceAttention}
        busyLabel={busyLabel}
        canOpenWebUi={canOpenWebUi}
        controlsDisabled={controlsDisabled}
        onOpenWeb={onOpenWeb}
        onStart={onStart}
        onStop={onStop}
        primaryActionLabel={presentation.primaryActionLabel}
        snapshot={{
          serviceDetail: serviceControlDetail,
          serviceState: presentation.state,
        }}
        startDisabled={startDisabled}
        stopDisabled={stopDisabled}
      />

      <div className="status-layout" data-has-rail={showStatusRail ? "true" : "false"}>
        <div className="status-main-column">
          <AppShellRuntimePreparePanel runtimePrepare={runtimePrepare} />

          {hasReadinessDiagnostics ? (
            <section className="data-section service-diagnostics">
              <h3>服务诊断</h3>

              {readinessReasonCodes.length > 0 ? (
                <div className="status-diagnostics-block">
                  <span className="status-label">原因代码</span>
                  <div className="status-diagnostics-codes">
                    {readinessReasonCodes.map((code) => (
                      <code key={code} className="status-chip status-chip--muted mono">{code}</code>
                    ))}
                  </div>
                </div>
              ) : null}

              {readinessIssues.some((issue) => issue.remediation) ? (
                <div className="status-diagnostics-block">
                  <span className="status-label">处理方式</span>
                  <div className="status-diagnostics-list">
                    {readinessIssues.filter((issue) => issue.remediation).slice(0, 3).map((issue) => (
                      <div
                        key={`${issue.code}-${issue.summary}`}
                        className={`status-diagnostics-item status-diagnostics-item--${issue.severity}`}
                      >
                        <div className="status-diagnostics-item__header">
                          <span className="status-diagnostics-item__summary">{issue.remediation}</span>
                          <span className={`status-pill status-pill--${issue.severity === "error" ? "error" : "warning"}`}>
                            {issue.severity === "error" ? "阻塞" : "警告"}
                          </span>
                        </div>
                        <code className="status-diagnostics-item__code mono">{issue.code}</code>
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
            </section>
          ) : null}

          <AppShellStatusSummary snapshot={snapshot} resolvedSettings={resolvedSettings} />
        </div>

        {showStatusRail ? (
          <AppShellStatusRail
            canPrepareRuntime={canPrepareRuntime}
            canRecheckRecovery={canRecheckRecovery}
            checks={nonOkChecks}
            onOpenRecoveryTasks={onOpenRecoveryTasks}
            onOpenRuntimeTasks={onOpenRuntimeTasks}
            recoveryStatusSummary={recoveryStatusSummary}
            showRecoverySummary={Boolean(recoverySummary)}
          />
        ) : null}
      </div>

      <AppShellStatusLogs
        hasRecentStderr={hasRecentStderr}
        logAlert={logAlert}
        logs={snapshot.launcher.recentStderr}
        onOpenLogs={onOpenLogs}
      />
    </div>
  );
}
