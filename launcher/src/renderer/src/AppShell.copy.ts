import type {
  EnvironmentCheckScope,
  LauncherProcessLifecycle,
  LauncherProcessOwnership,
  LauncherReadinessSnapshot,
  LauncherSystemStatusSnapshot,
  LivenessStatusResponse,
  RecoveryCompatibilitySummary,
} from "@shared/launcher-models";

const readinessStatusLabels: Record<LauncherReadinessSnapshot["status"], string> = {
  ready: "已就绪",
  degraded: "部分功能受限",
  setup_required: "需要初始化",
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
  external: "外部服务",
};

const environmentScopeLabels: Record<EnvironmentCheckScope, string> = {
  preflight: "启动前检查",
  advisory: "运行建议",
};

const diagnosticCheckNameLabels: Record<string, string> = {
  adapter: "消息连接",
  bilibili_source: "Bilibili 来源",
  config: "配置",
  database: "数据库",
  dependencies: "运行依赖",
  filesystem: "文件系统",
  plugins: "插件",
  render: "图片渲染",
  runtime: "运行环境",
  scheduler: "定时任务",
  tasks: "任务队列",
  third_party: "第三方平台",
};

const diagnosticCheckValueLabels: Record<string, string> = {
  cached: "已缓存",
  degraded: "部分功能受限",
  failed: "未就绪",
  metadata_incomplete: "元数据不完整",
  missing: "缺失",
  ok: "正常",
  on_demand: "按需准备",
  ready: "已就绪",
  resource_missing: "资源缺失",
  setup_required: "需要初始化",
  unavailable: "不可用",
  unreadable: "无法读取",
  unknown: "未知",
};

const recoveryStatusLabels: Record<RecoveryCompatibilitySummary["status"], string> = {
  pending: "待检查",
  compatible: "兼容",
  degraded: "部分受限",
  blocked: "需要处理",
};

const recoveryOperationLabels: Record<RecoveryCompatibilitySummary["operation"], string> = {
  restore: "恢复",
  upgrade: "升级",
  rollback: "回滚",
};

const recoveryPhaseLabels: Record<RecoveryCompatibilitySummary["phase"], string> = {
  pre_restore: "恢复前",
  post_startup: "启动后",
};

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
  return diagnosticCheckNameLabels[value] ?? value;
}

export function formatDiagnosticCheckValue(value: string): string {
  return diagnosticCheckValueLabels[value] ?? value;
}

export function formatRecoverySummary(value: RecoveryCompatibilitySummary | null | undefined): string {
  if (!value) {
    return "没有恢复兼容性摘要。";
  }

  const status = recoveryStatusLabels[value.status] ?? value.status;
  const operation = recoveryOperationLabels[value.operation] ?? value.operation;
  const phase = recoveryPhaseLabels[value.phase] ?? value.phase;
  return `${status} · ${operation} · ${phase}`;
}
