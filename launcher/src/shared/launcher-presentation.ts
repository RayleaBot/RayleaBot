import type {
  EnvironmentCheckResult,
  LauncherDiagnosticIssue,
  LauncherProcessOwnership,
  LauncherReadinessSnapshot,
  LauncherSnapshot,
} from "./launcher-models";

export type LauncherPresentationState =
  | "stopped"
  | "starting"
  | "running"
  | "setup_required"
  | "degraded"
  | "unhealthy"
  | "stopping"
  | "failed";

type RuntimeResource = NonNullable<LauncherDiagnosticIssue["runtime_resources"]>[number];

export interface LauncherPresentation {
  state: LauncherPresentationState;
  label: string;
  detail: string;
  canOpenWebUi: boolean;
  canStopService: boolean;
  canRunServiceAction: boolean;
  /** Resources the management UI's status page can prepare, from the service's readiness issues. */
  preparableRuntimeResources: RuntimeResource[];
}

const stateLabels: Record<LauncherPresentationState, string> = {
  stopped: "未启动",
  starting: "启动中",
  running: "运行中",
  setup_required: "待初始化",
  degraded: "运行条件受限",
  unhealthy: "运行异常",
  stopping: "停止中",
  failed: "启动失败",
};

function firstReadinessIssue(readiness: LauncherReadinessSnapshot | null) {
  return readiness?.issues?.[0] ?? null;
}

export function isBlockingEnvironmentIssue(check: EnvironmentCheckResult) {
  return check.scope === "preflight" && check.severity === "error";
}

function getPrimaryEnvironmentIssue(checks: EnvironmentCheckResult[]) {
  return checks.find(isBlockingEnvironmentIssue)
    ?? checks.find((item) => item.severity === "warning")
    ?? null;
}

function buildLocalDetail(fallback: string, checks: EnvironmentCheckResult[]) {
  const issue = getPrimaryEnvironmentIssue(checks);
  if (!issue) {
    return fallback;
  }

  const detail = issue.detail ? `${issue.summary} ${issue.detail}` : issue.summary;
  return issue.remediation ? `${detail} ${issue.remediation}` : detail;
}

function detailFromReadiness(readiness: LauncherReadinessSnapshot, fallback: string) {
  return readiness.reason?.trim() || firstReadinessIssue(readiness)?.summary || fallback;
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
      ? "检测到现有服务，可以使用，但部分运行条件未满足。"
      : "服务可以使用，但部分运行条件未满足。",
  );
}

function unhealthyDetail(readiness: LauncherReadinessSnapshot) {
  return detailFromReadiness(readiness, "服务已运行，但尚未达到就绪状态。");
}

function derivePresentationState(snapshot: LauncherSnapshot): Pick<LauncherPresentation, "state" | "detail"> {
  const { health, readiness, systemStatus } = snapshot.server;
  const {
    preflightChecks,
    lastLocalError,
    processLifecycle,
    processOwnership,
    statusHint,
  } = snapshot.launcher;
  const blockingIssue = preflightChecks.some(isBlockingEnvironmentIssue);
  const localHint = statusHint.trim();

  if (processLifecycle === "starting") {
    return {
      state: "starting",
      detail: localHint || "正在准备运行环境并等待服务就绪。",
    };
  }

  if (processLifecycle === "stopping") {
    return {
      state: "stopping",
      detail: localHint || (processOwnership === "external" ? "正在停止现有服务。" : "正在停止服务。"),
    };
  }

  // A reachable service or a live process has started, so its faults are running faults, not failed starts.
  if (health && readiness) {
    if (systemStatus?.status === "shutting_down") {
      return {
        state: "stopping",
        detail: detailFromReadiness(readiness, "服务正在停止。"),
      };
    }

    // A hint here explains a service the Launcher cannot control, such as one started by another program.
    switch (readiness.status) {
      case "ready":
        return { state: "running", detail: localHint || runningDetail(readiness, processOwnership) };
      case "degraded":
        return { state: "degraded", detail: localHint || degradedDetail(readiness, processOwnership) };
      // The service runs but has no administrator yet. The Launcher opens the setup page with its own token; a
      // service started elsewhere printed its one-time setup address in that console.
      case "setup_required":
        return {
          state: "setup_required",
          detail: localHint || (processOwnership === "external"
            ? "检测到尚未初始化的现有服务。请用启动该服务时控制台显示的首次设置地址创建管理员账号。"
            : "服务已启动，还没有管理员账号。打开管理界面即可创建。"),
        };
      case "failed":
      default:
        return { state: "unhealthy", detail: unhealthyDetail(readiness) };
    }
  }

  if (health) {
    return {
      state: "unhealthy",
      detail: localHint || "服务正在运行，但无法读取就绪状态。",
    };
  }

  if (processLifecycle === "running") {
    return {
      state: "unhealthy",
      detail: localHint || "服务进程仍在运行，但健康检查失败。",
    };
  }

  if (blockingIssue) {
    return {
      state: "stopped",
      detail: localHint || buildLocalDetail("服务尚未启动。", preflightChecks),
    };
  }

  return {
    state: lastLocalError.trim() ? "failed" : "stopped",
    detail: localHint || (lastLocalError.trim() ? lastLocalError.trim() : "服务尚未启动。"),
  };
}

export function getLauncherStateLabel(state: LauncherPresentationState) {
  return stateLabels[state] ?? "未知状态";
}

export function getEnvironmentSummaryLabel(checks: EnvironmentCheckResult[]) {
  if (checks.length === 0) {
    return "尚未检查";
  }
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

// The management UI's status page offers preparation for the runtime resources of the readiness issues it
// lists, so the Launcher points there only for the same resources and never infers them from local checks.
function preparableRuntimeResources(readiness: LauncherReadinessSnapshot | null): RuntimeResource[] {
  const issues = (readiness?.issues ?? []).filter((issue) => issue.severity === "error" || issue.severity === "warning");
  return [...new Set(issues.flatMap((issue) => issue.runtime_resources ?? []))];
}

export function deriveLauncherPresentation(snapshot: LauncherSnapshot): LauncherPresentation {
  const { state, detail } = derivePresentationState(snapshot);
  const setupRequired = snapshot.server.readiness?.status === "setup_required";
  const canOpenWebUi = state === "running" || state === "setup_required" || state === "degraded";
  const canStopService =
    (state === "running" || state === "setup_required" || state === "degraded" || state === "unhealthy" || state === "failed")
    && snapshot.launcher.processOwnership !== "none";

  return {
    state,
    label: getLauncherStateLabel(state),
    detail,
    canOpenWebUi,
    canStopService,
    canRunServiceAction: state !== "starting" && state !== "stopping",
    preparableRuntimeResources: canOpenWebUi && !setupRequired ? preparableRuntimeResources(snapshot.server.readiness) : [],
  };
}
