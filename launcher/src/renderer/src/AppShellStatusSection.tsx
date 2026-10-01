import { useMemo } from "react";
import { deriveLauncherPresentation } from "@shared/launcher-presentation";
import type { LauncherResolvedSettings, LauncherSnapshot } from "@shared/launcher-models";

import { formatDiagnosticCheckName, formatDiagnosticCheckValue } from "./AppShell.copy";
import { busyActionLabels, sortChecks } from "./AppShell.shared";
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
  onOpenLogs,
}: StatusSectionProps) {
  const presentation = useMemo(() => deriveLauncherPresentation(snapshot), [snapshot]);
  const runtimePrepare = snapshot.launcher.runtimePrepare ?? null;
  const readiness = snapshot.server.readiness ?? null;
  const setupRequired = readiness?.status === "setup_required";
  const checks = useMemo(() => sortChecks(snapshot.launcher.preflightChecks || []), [snapshot.launcher.preflightChecks]);
  const nonOkChecks = useMemo(() => checks.filter((item) => item.severity !== "ok"), [checks]);
  const readinessIssues = setupRequired ? [] : readiness?.issues ?? [];
  const readinessReason = setupRequired ? "" : readiness?.reason?.trim() ?? "";
  const nonOkReadinessChecks = useMemo(
    () => setupRequired
      ? []
      : Object.entries(readiness?.checks ?? {}).filter(([, value]) => value && value !== "ok"),
    [readiness?.checks, setupRequired],
  );
  const remediationIssues = useMemo(
    () => readinessIssues.filter((issue) => issue.remediation).slice(0, 3),
    [readinessIssues],
  );
  const primaryReadinessIssue = readinessIssues[0] ?? null;
  const localError = snapshot.launcher.lastLocalError.trim();
  const statusHint = snapshot.launcher.statusHint.trim();
  const serviceFault = presentation.state === "failed" || presentation.state === "unhealthy";
  const statusAlert =
    localError
      ? "error"
      : primaryReadinessIssue
        ? primaryReadinessIssue.severity === "error" ? "error" : "warning"
        : readinessReason
          ? serviceFault ? "error" : "warning"
          : nonOkChecks.length > 0
            ? "warning"
          : "none";
  const statusReasonText = readinessReason || primaryReadinessIssue?.summary || presentation.detail;
  // A local error with a hint describes a failed Launcher operation, so the hint replaces the state description.
  // An error without one, such as a system status the Launcher may not read, keeps the description and is
  // shown as the cause below.
  const serviceControlDetail =
    runtimePrepare?.active
      ? "运行环境准备中。"
      : localError && statusHint
        ? statusHint
        : presentation.state === "degraded"
          ? "服务可以使用，但部分运行条件未满足。"
          : presentation.state === "unhealthy" && readiness
            ? "服务未通过就绪检查。"
            : presentation.detail;
  const hasReadinessDiagnostics = !runtimePrepare?.active && (remediationIssues.length > 0 || nonOkReadinessChecks.length > 0);
  const canOpenWebUi = presentation.canOpenWebUi;
  const preparableResources = controlsDisabled ? [] : presentation.preparableRuntimeResources;
  const showStatusRail = nonOkChecks.length > 0 || preparableResources.length > 0;
  const externalService = snapshot.launcher.processOwnership === "external";
  const startDisabled =
    controlsDisabled
    || ((presentation.state === "running" || presentation.state === "setup_required" || presentation.state === "degraded") && externalService)
    || presentation.state === "starting"
    || presentation.state === "stopping";
  const stopDisabled =
    controlsDisabled
    || presentation.state === "starting"
    || presentation.state === "stopping"
    || snapshot.launcher.processOwnership === "none";
  const busyLabel = busyAction ? (busyActionLabels[busyAction] ?? "正在执行操作") : "";
  // A restart keeps the running actions in place while the service stops and starts again.
  const showRunningActions = canOpenWebUi || busyAction === "restart";
  const serviceAttention = (() => {
    if (runtimePrepare?.active) {
      return { label: "准备进度", text: runtimePrepare.summary || "正在准备运行环境。", tone: "attention" as const };
    }
    const note = localError
      ? { label: "失败原因", text: localError, tone: "danger" as const }
      : serviceFault
        ? { label: "失败原因", text: statusReasonText, tone: "danger" as const }
        : presentation.state === "degraded"
          ? { label: "当前限制", text: statusReasonText, tone: "warning" as const }
          : null;
    return note && note.text !== serviceControlDetail ? note : null;
  })();

  return (
    <div className="status-homepage status-view-flow" data-state={presentation.state} data-busy={busyAction ?? "idle"} data-alert={statusAlert}>
      <AppShellServiceControl
        attention={serviceAttention}
        busyLabel={busyLabel}
        canOpenWebUi={canOpenWebUi}
        controlsDisabled={controlsDisabled}
        externalService={externalService}
        onOpenWeb={onOpenWeb}
        onStart={onStart}
        onStop={onStop}
        showRunningActions={showRunningActions}
        stopOpensWeb={snapshot.launcher.controlCapability === "open_web"}
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
            <section className="service-diagnostics content-group">
              <h3>服务诊断</h3>

              {remediationIssues.length > 0 ? (
                <div className="status-diagnostics-block">
                  <span className="status-label">问题与处理方式</span>
                  <div className="status-diagnostics-list">
                    {remediationIssues.map((issue) => (
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
                        <p className="status-diagnostics-item__remedy">处理方式：{issue.remediation}</p>
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
                        <span className="status-diagnostics-check__name">{formatDiagnosticCheckName(name)}</span>
                        <span className="status-diagnostics-check__value">{formatDiagnosticCheckValue(value)}</span>
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
            checks={nonOkChecks}
            runtimeResources={preparableResources}
            onOpenWeb={onOpenWeb}
          />
        ) : null}
      </div>

      <AppShellStatusLogs
        externalService={externalService}
        logs={snapshot.launcher.recentStderr}
        onOpenLogs={onOpenLogs}
      />
    </div>
  );
}
