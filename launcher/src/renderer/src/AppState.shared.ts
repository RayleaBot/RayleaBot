import { deriveLauncherPresentation, formatReadinessIssue, resolveRecoverySummary } from "@shared/launcher-presentation";
import type { LauncherSnapshot } from "@shared/launcher-models";

export type SectionId = "status" | "environment" | "diagnostics" | "settings";
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
    releaseCheck: {
      status: "unavailable",
      currentVersion: "",
      latestVersion: "",
      summary: "版本信息不可用",
      detail: "",
      releasePageUrl: "",
      updateAvailable: false,
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
    .map(([name, value]) => `- ${name}：${value}`)
    .join("\n");
  const readinessIssues = (readiness?.issues ?? [])
    .map((item) => `- ${formatReadinessIssue(item)}`)
    .join("\n");
  const checks = snapshot.launcher.environmentChecks
    .map(
      (item) =>
        `- ${item.scope} / ${item.title}：${item.summary}（${item.detail}${item.remediation ? `；${item.remediation}` : ""}）`,
    )
    .join("\n");
  const recentErrors =
    snapshot.launcher.recentStderr.length
      ? snapshot.launcher.recentStderr.join("\n")
      : "当前没有新的错误输出。";
  const recoverySummary = resolveRecoverySummary(snapshot);

  return [
    `状态摘要：${presentation.label}`,
    `状态说明：${presentation.detail}`,
    `本地端点：${snapshot.launcher.endpoint.baseUrl}`,
    `安装目录：${snapshot.launcher.settings.installationRoot || "未设置"}`,
    `服务端：${snapshot.launcher.resolvedSettings.serverExecutablePath || "未设置"}`,
    `配置文件：${snapshot.launcher.resolvedSettings.configPath || "未设置"}`,
    `运行目录：${snapshot.launcher.resolvedSettings.workdir || "未设置"}`,
    "服务端状态：",
    [
      `healthz：${snapshot.server.health?.status ?? "不可用"}`,
      `readyz：${readiness?.status ?? "不可用"}`,
      `system/status：${snapshot.server.systemStatus?.status ?? "不可用"}`,
      `原因：${readiness?.reason || "—"}`,
      `原因代码：${readiness?.reason_codes?.length ? readiness.reason_codes.join(", ") : "—"}`,
      "问题：",
      readinessIssues || "- 当前没有 readiness 问题。",
      "检查：",
      readinessChecks || "- 当前没有非正常检查项。",
    ].join("\n"),
    "启动器本地状态：",
    [
      `进程观察：${snapshot.launcher.processLifecycle}`,
      `进程归属：${snapshot.launcher.processOwnership}`,
      `本地提示：${snapshot.launcher.statusHint || "—"}`,
      `本地错误：${snapshot.launcher.lastLocalError || "—"}`,
    ].join("\n"),
    "环境检查：",
    checks || "- 当前没有检查项。",
    "恢复兼容性：",
    recoverySummary
      ? `${recoverySummary.status} / ${recoverySummary.operation} / ${recoverySummary.phase}`
      : "当前没有恢复摘要。",
    "最近错误输出：",
    recentErrors,
  ].join("\n");
}

export function describeLauncherError(error: unknown, fallback: string) {
  if (error instanceof Error && error.message) {
    return error.message;
  }
  return fallback;
}
