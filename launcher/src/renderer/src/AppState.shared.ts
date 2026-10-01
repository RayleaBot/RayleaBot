import { deriveLauncherPresentation, formatReadinessIssue } from "@shared/launcher-presentation";
import type { LauncherSnapshot } from "@shared/launcher-models";
import {
  formatDiagnosticCheckName,
  formatDiagnosticCheckValue,
  formatEnvironmentScope,
  formatHealthStatus,
  formatProcessLifecycle,
  formatProcessOwnership,
  formatReadinessStatus,
  formatSystemStatus,
  uncapturedOutputText,
} from "./AppShell.copy";

export const initialSnapshot: LauncherSnapshot = {
  server: {
    health: null,
    readiness: null,
    systemStatus: null,
  },
  launcher: {
    processId: null,
    processLifecycle: "stopped",
    processOwnership: "none",
    environmentChecks: [],
    preflightChecks: [],
    advisoryChecks: [],
    recentStderr: [],
    runtimePrepare: null,
    releaseCheck: {
      status: "disabled",
      currentVersion: "",
      latestVersion: "",
      summary: "版本信息不可用",
      detail: "",
      errorCode: "",
      releasePageUrl: "",
      updateAvailable: false,
      canCheck: false,
    },
    lastLocalError: "",
    statusHint: "",
    settings: {
      installationRoot: "",
      closeBehavior: "ask_every_time",
    },
    resolvedSettings: {
      installationRoot: "",
      serverExecutablePath: "",
      configPath: "",
      workdir: "",
    },
    endpoint: {
      host: "127.0.0.1",
      port: 8080,
      baseUrl: "http://127.0.0.1:8080/",
    },
  },
};

export function buildDiagnosticsSummary(snapshot: LauncherSnapshot) {
  const presentation = deriveLauncherPresentation(snapshot);
  const readiness = snapshot.server.readiness;
  const setupRequired = readiness?.status === "setup_required";
  const readinessChecks = Object.entries(setupRequired ? {} : readiness?.checks ?? {})
    .filter(([, value]) => value && value !== "ok")
    .map(([name, value]) => `- ${formatDiagnosticCheckName(name)}：${formatDiagnosticCheckValue(value)}`)
    .join("\n");
  const readinessIssues = (setupRequired ? [] : readiness?.issues ?? [])
    .map((item) => `- ${formatReadinessIssue(item)}`)
    .join("\n");
  const checks = snapshot.launcher.environmentChecks
    .map((item) => {
      const note = [item.detail, item.remediation].filter(Boolean).join("；");
      return `- ${formatEnvironmentScope(item.scope)}：${item.title}，${item.summary}${note ? `（${note}）` : ""}`;
    })
    .join("\n");
  const recentErrors =
    snapshot.launcher.recentStderr.length
      ? snapshot.launcher.recentStderr.join("\n")
      : snapshot.launcher.processOwnership === "external"
        ? uncapturedOutputText
        : "未发现新的异常输出。";

  return [
    `状态摘要：${presentation.label}`,
    `状态说明：${presentation.detail}`,
    `管理界面地址：${snapshot.launcher.endpoint.baseUrl}`,
    `安装目录：${snapshot.launcher.settings.installationRoot || "未设置"}`,
    `服务端程序：${snapshot.launcher.resolvedSettings.serverExecutablePath || "未设置"}`,
    `配置文件：${snapshot.launcher.resolvedSettings.configPath || "未设置"}`,
    `工作目录：${snapshot.launcher.resolvedSettings.workdir || "未设置"}`,
    "服务状态：",
    [
      `服务连接：${formatHealthStatus(snapshot.server.health?.status)}`,
      `就绪状态：${formatReadinessStatus(readiness?.status)}`,
      `系统状态：${formatSystemStatus(snapshot.server.systemStatus?.status)}`,
      `原因：${setupRequired ? "—" : readiness?.reason || "—"}`,
      `原因代码：${!setupRequired && readiness?.reason_codes?.length ? readiness.reason_codes.join(", ") : "—"}`,
      "就绪问题：",
      readinessIssues || "- 未发现就绪问题。",
      "就绪检查：",
      readinessChecks || "- 未发现异常检查项。",
    ].join("\n"),
    "启动器状态：",
    [
      `进程状态：${formatProcessLifecycle(snapshot.launcher.processLifecycle)}`,
      `启动方式：${formatProcessOwnership(snapshot.launcher.processOwnership)}`,
      `状态提示：${snapshot.launcher.statusHint || "—"}`,
      `启动器错误：${snapshot.launcher.lastLocalError || "—"}`,
    ].join("\n"),
    "环境检查：",
    checks || "- 没有需要显示的检查项。",
    "异常输出：",
    recentErrors,
  ].join("\n");
}

/**
 * Wails rejects a failed desktop call with the Go error's JSON as `cause`. The Go side sends a BoundaryError
 * as its code and readable message; any other error keeps the generic text, since it can carry internal details.
 */
export function describeLauncherError(error: unknown, fallback: string) {
  const cause = error instanceof Error ? error.cause : undefined;
  if (typeof cause !== "object" || cause === null) {
    return fallback;
  }
  const { code, message } = cause as { code?: unknown; message?: unknown };
  return typeof code === "string" && code && typeof message === "string" && message.trim() ? message.trim() : fallback;
}
