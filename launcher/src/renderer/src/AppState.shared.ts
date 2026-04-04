import type { LauncherSnapshot } from "@shared/launcher-models";

export type SectionId = "status" | "environment" | "diagnostics" | "settings";
export type SectionTransitionState = "idle" | "exiting" | "entering";

export const initialSnapshot: LauncherSnapshot = {
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
  environmentChecks: [],
  recentStderr: [],
  processId: null,
  serviceState: "stopped",
  serviceOwnership: "none",
  shutdownRequested: false,
  serviceDetail: "服务尚未启动。",
  lastError: "",
  releaseCheck: {
    status: "unavailable",
    currentVersion: "",
    latestVersion: "",
    summary: "版本信息不可用",
    detail: "",
    releasePageUrl: "",
    updateAvailable: false,
  },
};

export function buildDiagnosticsSummary(snapshot: LauncherSnapshot) {
  const checks = snapshot.environmentChecks
    .map(
      (item) =>
        `- ${item.title}：${item.summary}（${item.detail}${item.remediation ? `；${item.remediation}` : ""}）`,
    )
    .join("\n");
  const recentErrors =
    snapshot.recentStderr.length
      ? snapshot.recentStderr.join("\n")
      : "当前没有新的错误输出。";
  return [
    `服务状态：${snapshot.serviceDetail}`,
    `服务入口：${snapshot.endpoint.baseUrl}`,
    `安装目录：${snapshot.settings.installationRoot || "未设置"}`,
    `服务端：${snapshot.resolvedSettings.serverExecutablePath || "未设置"}`,
    `配置文件：${snapshot.resolvedSettings.configPath || "未设置"}`,
    `运行目录：${snapshot.resolvedSettings.workdir || "未设置"}`,
    "环境检查：",
    checks || "- 当前没有检查项。",
    "恢复兼容性：",
    snapshot.recoverySummary
      ? `${snapshot.recoverySummary.status} / ${snapshot.recoverySummary.operation} / ${snapshot.recoverySummary.phase}`
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
