import type {
  EnvironmentCheckResult,
  LauncherDiagnosticIssue,
  LauncherProcessOwnership,
  LauncherReadinessSnapshot,
  LauncherSnapshot,
  RecoveryCompatibilitySummary,
} from "./launcher-models";

export type LauncherPresentationState =
  | "stopped"
  | "starting"
  | "running"
  | "degraded"
  | "setup_required"
  | "stopping"
  | "failed";

export interface LauncherPresentation {
  state: LauncherPresentationState;
  label: string;
  detail: string;
  primaryActionLabel: string;
  canOpenWebUi: boolean;
  canStopService: boolean;
  canRunRecoveryActions: boolean;
  canRecheckRecovery: boolean;
  canRunServiceAction: boolean;
  recoverySummary: RecoveryCompatibilitySummary | null;
}

const stateLabels: Record<LauncherPresentationState, string> = {
  stopped: "未启动",
  starting: "启动中",
  running: "运行中",
  degraded: "运行条件受限",
  setup_required: "需要设置",
  stopping: "停止中",
  failed: "启动失败",
};

function firstReadinessIssue(readiness: LauncherReadinessSnapshot | null) {
  return readiness?.issues?.[0] ?? null;
}

function isBlockingEnvironmentIssue(check: EnvironmentCheckResult) {
  return check.scope === "preflight" && check.severity === "error";
}

export function hasBootstrapConfigAvailable(checks: EnvironmentCheckResult[]) {
  return checks.some((item) => item.code === "config.bootstrap_available");
}

export function getPrimaryEnvironmentIssue(checks: EnvironmentCheckResult[]) {
  return checks.find(isBlockingEnvironmentIssue)
    ?? checks.find((item) => item.severity === "warning")
    ?? null;
}

export function buildLocalDetail(fallback: string, checks: EnvironmentCheckResult[]) {
  const issue = getPrimaryEnvironmentIssue(checks);
  if (!issue) {
    return fallback;
  }

  const detail = issue.detail ? `${issue.summary} ${issue.detail}` : issue.summary;
  return issue.remediation ? `${detail} ${issue.remediation}` : detail;
}

export function detailFromReadiness(readiness: LauncherReadinessSnapshot, fallback: string) {
  return readiness.reason?.trim() || firstReadinessIssue(readiness)?.summary || fallback;
}

export function startingDetail(hasBootstrapConfig: boolean) {
  return hasBootstrapConfig
    ? "已基于 default.yaml 生成首份用户配置，正在准备运行环境并等待服务就绪。"
    : "正在准备运行环境并等待服务就绪。";
}

export function resolveRecoverySummary(snapshot: LauncherSnapshot) {
  return snapshot.server.systemStatus?.recovery_summary
    ?? snapshot.server.readiness?.recovery_summary
    ?? snapshot.launcher.localRecoverySummary
    ?? null;
}

function runningDetail(readiness: LauncherReadinessSnapshot, ownership: LauncherProcessOwnership) {
  return detailFromReadiness(
    readiness,
    ownership === "external"
      ? "检测到现有服务。可以直接打开管理界面，或确认后停止它。"
      : "服务正在运行。",
  );
}

function degradedDetail(readiness: LauncherReadinessSnapshot, ownership: LauncherProcessOwnership) {
  return detailFromReadiness(
    readiness,
    ownership === "external"
      ? "检测到现有服务，管理面可用，但当前仍有运行条件未满足。"
      : "管理面可用，但当前仍有运行条件未满足。",
  );
}

function failedDetail(readiness: LauncherReadinessSnapshot) {
  return detailFromReadiness(readiness, "服务已运行，但尚未达到就绪状态。");
}

function derivePresentationState(snapshot: LauncherSnapshot): Pick<LauncherPresentation, "state" | "detail"> {
  const { health, readiness, systemStatus } = snapshot.server;
  const {
    environmentChecks,
    lastLocalError,
    processLifecycle,
    processOwnership,
    statusHint,
  } = snapshot.launcher;
  const bootstrapConfigAvailable = hasBootstrapConfigAvailable(environmentChecks);
  const blockingIssue = environmentChecks.some(isBlockingEnvironmentIssue);
  const localHint = statusHint.trim();

  if (processLifecycle === "starting") {
    return {
      state: "starting",
      detail: localHint || startingDetail(bootstrapConfigAvailable),
    };
  }

  if (processLifecycle === "stopping") {
    return {
      state: "stopping",
      detail: localHint || (processOwnership === "external" ? "正在停止现有服务。" : "正在停止服务。"),
    };
  }

  if (health && readiness) {
    if (systemStatus?.status === "shutting_down") {
      return {
        state: "stopping",
        detail: detailFromReadiness(readiness, "服务正在停止。"),
      };
    }

    switch (readiness.status) {
      case "ready":
        return { state: "running", detail: runningDetail(readiness, processOwnership) };
      case "degraded":
        return { state: "degraded", detail: degradedDetail(readiness, processOwnership) };
      case "setup_required":
        return {
          state: "setup_required",
          detail: detailFromReadiness(readiness, "服务正在运行，需要完成管理员初始化。"),
        };
      case "failed":
      default:
        return { state: "failed", detail: failedDetail(readiness) };
    }
  }

  if (health) {
    return {
      state: "failed",
      detail: localHint || "服务存活，但无法读取正式就绪状态。",
    };
  }

  if (processLifecycle === "running") {
    return {
      state: "failed",
      detail: localHint || "子进程仍在运行，但健康检查失败。",
    };
  }

  if (bootstrapConfigAvailable) {
    return {
      state: "stopped",
      detail: localHint || "服务尚未启动。启动服务后会基于 default.yaml 生成首份用户配置。",
    };
  }

  if (blockingIssue) {
    return {
      state: "stopped",
      detail: localHint || buildLocalDetail("服务尚未启动。", environmentChecks),
    };
  }

  return {
    state: lastLocalError.trim() ? "failed" : "stopped",
    detail: localHint || (lastLocalError.trim() ? lastLocalError.trim() : "服务尚未启动。"),
  };
}

function primaryActionLabel(state: LauncherPresentationState, ownership: LauncherProcessOwnership) {
  if ((state === "running" || state === "degraded") && ownership === "external") {
    return "检测到现有服务";
  }
  if ((state === "running" || state === "degraded") && ownership === "launcher_managed") {
    return "重启服务";
  }
  if (state === "setup_required") {
    return "打开初始化";
  }
  return "启动 RayleaBot";
}

export function getLauncherStateLabel(state: LauncherPresentationState) {
  return stateLabels[state] ?? "未知状态";
}

export function getEnvironmentSummaryLabel(checks: EnvironmentCheckResult[]) {
  if (checks.some(isBlockingEnvironmentIssue)) {
    return "需要处理";
  }
  if (checks.some((item) => item.severity === "warning")) {
    return "可继续，但有警告";
  }
  return "可以启动";
}

export function formatReadinessIssue(issue: LauncherDiagnosticIssue) {
  return `${issue.code}：${issue.summary}${issue.remediation ? `（${issue.remediation}）` : ""}`;
}

export function deriveLauncherPresentation(snapshot: LauncherSnapshot): LauncherPresentation {
  const { state, detail } = derivePresentationState(snapshot);
  const recoverySummary = resolveRecoverySummary(snapshot);
  const canOpenWebUi = state === "running" || state === "degraded" || state === "setup_required";
  const canStopService =
    (state === "running" || state === "degraded" || state === "failed" || state === "setup_required")
    && snapshot.launcher.processOwnership !== "none";
  const canRunRecoveryActions = state === "running" || state === "degraded";

  return {
    state,
    label: getLauncherStateLabel(state),
    detail,
    primaryActionLabel: primaryActionLabel(state, snapshot.launcher.processOwnership),
    canOpenWebUi,
    canStopService,
    canRunRecoveryActions,
    canRecheckRecovery: canRunRecoveryActions && Boolean(recoverySummary),
    canRunServiceAction: state !== "starting" && state !== "stopping",
    recoverySummary,
  };
}
