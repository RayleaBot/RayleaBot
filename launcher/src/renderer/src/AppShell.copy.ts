import type {
  EnvironmentCheckScope,
  LauncherDiagnosticIssue,
  LauncherProcessLifecycle,
  LauncherProcessOwnership,
  LauncherReadinessSnapshot,
  LauncherSystemStatusSnapshot,
  LivenessStatusResponse,
} from "@shared/launcher-models";

const readinessStatusLabels: Record<LauncherReadinessSnapshot["status"], string> = {
  ready: "已就绪",
  degraded: "运行条件受限",
  setup_required: "运行中",
  failed: "未就绪",
};

const processLifecycleLabels: Record<LauncherProcessLifecycle, string> = {
  stopped: "未启动",
  starting: "启动中",
  running: "运行中",
  stopping: "停止中",
};

const processOwnershipLabels: Record<LauncherProcessOwnership, string> = {
  none: "无运行进程",
  launcher_managed: "由启动器启动",
  external: "现有服务",
};

const environmentScopeLabels: Record<EnvironmentCheckScope, string> = {
  preflight: "启动前检查",
  advisory: "运行建议",
};

// Readiness check names and states use the management UI's terms.
const diagnosticCheckNameLabels: Record<keyof NonNullable<LauncherReadinessSnapshot["checks"]>, string> = {
  config: "配置",
  database: "数据库",
  runtime: "运行环境",
  render: "图片生成",
};

const diagnosticCheckValueLabels: Record<string, string> = {
  cached: "已缓存",
  degraded: "运行条件受限",
  failed: "失败",
  metadata_incomplete: "元数据不完整",
  missing: "缺失",
  ok: "通过",
  on_demand: "按需准备",
  ready: "已就绪",
  resource_missing: "缺少运行资源",
  setup_required: "运行中",
  unavailable: "不可用",
  unreadable: "无法读取",
  unknown: "未知",
};

const runtimeResourceLabels: Record<NonNullable<LauncherDiagnosticIssue["runtime_resources"]>[number], string> = {
  chromium: "图片渲染 Chromium",
  ffmpeg: "媒体工具 FFmpeg",
};

/** The Launcher reads only the output of a service it started itself. */
export const uncapturedOutputText = "启动器未捕获该服务的输出，请在管理界面的实时日志查看。";

export function formatHealthStatus(status: LivenessStatusResponse["status"] | null | undefined): string {
  return status === "ok" ? "可连接" : "不可用";
}

export function formatReadinessStatus(status: LauncherReadinessSnapshot["status"] | null | undefined): string {
  return status ? (readinessStatusLabels[status] ?? status) : "不可用";
}

export function formatSystemStatus(status: LauncherSystemStatusSnapshot["status"] | null | undefined): string {
  if (status === "running") {
    return "运行中";
  }
  if (status === "shutting_down") {
    return "正在关闭";
  }
  return "不可用";
}

export function formatProcessLifecycle(value: LauncherProcessLifecycle): string {
  return processLifecycleLabels[value] ?? value;
}

export function formatProcessOwnership(value: LauncherProcessOwnership): string {
  return processOwnershipLabels[value] ?? value;
}

export function formatEnvironmentScope(value: EnvironmentCheckScope): string {
  return environmentScopeLabels[value] ?? value;
}

export function formatDiagnosticCheckName(value: string): string {
  return diagnosticCheckNameLabels[value as keyof typeof diagnosticCheckNameLabels] ?? value;
}

export function formatDiagnosticCheckValue(value: string): string {
  return diagnosticCheckValueLabels[value] ?? value;
}

export function formatRuntimeResource(value: keyof typeof runtimeResourceLabels): string {
  return runtimeResourceLabels[value] ?? value;
}
