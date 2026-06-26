import { deriveLauncherPresentation, formatReadinessIssue, resolveRecoverySummary } from "@shared/launcher-presentation";
import type { LauncherSnapshot } from "@shared/launcher-models";
import {
  formatDiagnosticCheckName,
  formatDiagnosticCheckValue,
  formatEnvironmentScope,
  formatHealthStatus,
  formatProcessLifecycle,
  formatProcessOwnership,
  formatReadinessStatus,
  formatRecoverySummary,
  formatSystemStatus,
} from "./AppShell.copy";

export type SectionId = "status" | "environment" | "diagnostics" | "settings" | "about";
export type SectionTransitionState = "idle" | "exiting" | "entering";

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
      status: "unavailable",
      currentVersion: "",
      latestVersion: "",
      summary: "版本信息不可用",
      detail: "",
      releasePageUrl: "",
      updateAvailable: false,
      downloadProgress: null,
      downloadedBytes: null,
      totalBytes: null,
      artifactFileName: "",
      canCheck: false,
      canDownload: false,
      canInstall: false,
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
    localRecoverySummary: null,
  },
};

export function buildDiagnosticsSummary(snapshot: LauncherSnapshot) {
  const presentation = deriveLauncherPresentation(snapshot);
  const readiness = snapshot.server.readiness;
  const readinessChecks = Object.entries(readiness?.checks ?? {})
    .filter(([, value]) => value && value !== "ok")
    .map(([name, value]) => `- ${formatDiagnosticCheckName(name)}：${formatDiagnosticCheckValue(value)}`)
    .join("\n");
  const readinessIssues = (readiness?.issues ?? [])
    .map((item) => `- ${formatReadinessIssue(item)}`)
    .join("\n");
  const checks = snapshot.launcher.environmentChecks
    .map(
      (item) =>
        `- ${formatEnvironmentScope(item.scope)}：${item.title}，${item.summary}（${item.detail}${item.remediation ? `；${item.remediation}` : ""}）`,
    )
    .join("\n");
  const recentErrors =
    snapshot.launcher.recentStderr.length
      ? snapshot.launcher.recentStderr.join("\n")
      : "未发现新的错误日志。";
  const recoverySummary = resolveRecoverySummary(snapshot);

  return [
    `状态摘要：${presentation.label}`,
    `状态说明：${presentation.detail}`,
    `本地端点：${snapshot.launcher.endpoint.baseUrl}`,
    `安装目录：${snapshot.launcher.settings.installationRoot || "未设置"}`,
    `服务端：${snapshot.launcher.resolvedSettings.serverExecutablePath || "未设置"}`,
    `配置文件：${snapshot.launcher.resolvedSettings.configPath || "未设置"}`,
    `进程工作目录：${snapshot.launcher.resolvedSettings.workdir || "未设置"}`,
    "服务状态：",
    [
      `服务连接：${formatHealthStatus(snapshot.server.health?.status)}`,
      `就绪状态：${formatReadinessStatus(readiness?.status)}`,
      `系统状态：${formatSystemStatus(snapshot.server.systemStatus?.status)}`,
      `原因：${readiness?.reason || "—"}`,
      `原因代码：${readiness?.reason_codes?.length ? readiness.reason_codes.join(", ") : "—"}`,
      "就绪问题：",
      readinessIssues || "- 未发现就绪问题。",
      "就绪检查：",
      readinessChecks || "- 未发现异常检查项。",
    ].join("\n"),
    "启动器状态：",
    [
      `进程状态：${formatProcessLifecycle(snapshot.launcher.processLifecycle)}`,
      `进程来源：${formatProcessOwnership(snapshot.launcher.processOwnership)}`,
      `状态提示：${snapshot.launcher.statusHint || "—"}`,
      `启动器错误：${snapshot.launcher.lastLocalError || "—"}`,
    ].join("\n"),
    "环境检查：",
    checks || "- 没有需要显示的检查项。",
    "恢复兼容性：",
    formatRecoverySummary(recoverySummary),
    "最近错误日志：",
    recentErrors,
  ].join("\n");
}

export function describeLauncherError(error: unknown, fallback: string) {
  if (error instanceof Error && error.message) {
    return error.message;
  }
  return fallback;
}
