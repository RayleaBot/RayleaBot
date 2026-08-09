import { buildLocalDetail } from "../../shared/launcher-presentation";
import type {
  LauncherManagementClient,
  LauncherOperationContext,
  LauncherSnapshotStore,
  LauncherStatusService,
  LauncherRuntimeContext,
  RecoverySummaryReader,
  ServerProcessController,
  DevelopmentServerWatcher,
} from "./launcher-coordinator.types";
import type {
  EnvironmentInspection,
  LauncherReadinessSnapshot,
  LauncherSnapshot,
} from "../../shared/launcher-models";

interface LauncherStatusServiceDependencies {
  runtimeContext: LauncherRuntimeContext;
  snapshotStore: LauncherSnapshotStore;
  inspectEnvironment(settings: LauncherOperationContext["resolvedSettings"]): Promise<EnvironmentInspection>;
  managementClient: LauncherManagementClient;
  processController: ServerProcessController;
  developmentServerWatcher?: DevelopmentServerWatcher;
  recoverySummaryReader?: RecoverySummaryReader;
}

export function createLauncherStatusService(deps: LauncherStatusServiceDependencies): LauncherStatusService {
  function watcherRestartHint() {
    const processId = deps.developmentServerWatcher?.processId;
    return processId
      ? `开发 watcher 正在重启服务（PID ${processId}），Launcher 不会重复启动。`
      : "开发 watcher 正在重启服务，Launcher 不会重复启动。";
  }

  function writeWatcherTransitionLog(message: string, context: LauncherOperationContext) {
    deps.processController.writeLauncherLog?.(message, context.resolvedSettings.workdir);
  }

  async function tryLoadSystemStatus(endpoint: LauncherOperationContext["endpoint"]) {
    try {
      return await deps.managementClient.getLauncherStatus(endpoint);
    } catch {
      return null;
    }
  }

  async function tryReadLocalRecoverySummary() {
    if (!deps.recoverySummaryReader) {
      return null;
    }
    try {
      return await deps.recoverySummaryReader.read(deps.processController.logDirectory);
    } catch {
      return null;
    }
  }

  async function buildSnapshotFromReadiness(
    context: LauncherOperationContext,
    inspection: EnvironmentInspection,
    readiness: LauncherReadinessSnapshot,
    _forceReauthentication: boolean,
  ): Promise<LauncherSnapshot> {
    const previous = deps.snapshotStore.snapshot;
    const systemStatus =
      readiness.status === "ready" || readiness.status === "degraded"
        ? await tryLoadSystemStatus(context.endpoint)
        : null;
    const processOwnership = deps.processController.isRunning ? "launcher_managed" : "external";
    const localRecoverySummary =
      systemStatus?.recovery_summary
      ?? readiness.recovery_summary
      ?? await tryReadLocalRecoverySummary();

    if (
      previous.launcher.processLifecycle === "starting"
      && previous.launcher.processOwnership === "external"
      && deps.developmentServerWatcher?.isActive()
    ) {
      writeWatcherTransitionLog(
        `开发 watcher 管理的服务已恢复（watcher PID ${deps.developmentServerWatcher.processId ?? "unknown"}，服务地址 ${context.endpoint.baseUrl}）。`,
        context,
      );
    }

    return deps.snapshotStore.buildSnapshot(
      context,
      inspection,
      {
        health: { status: "ok" },
        readiness,
        systemStatus,
      },
      {
        processOwnership,
        processLifecycle: systemStatus?.status === "shutting_down"
          ? "stopping"
          : deps.processController.isRunning ? "running" : "stopped",
        lastLocalError: "",
        statusHint: "",
        localRecoverySummary,
      },
    );
  }

  async function refresh(_forceReauthentication: boolean) {
    const context = await deps.runtimeContext.createOperationContext();
    const inspection = await deps.inspectEnvironment(context.resolvedSettings);

    if (inspection.hasBlockingIssues || inspection.canBootstrapUserConfig) {
      await deps.snapshotStore.publish(
        deps.snapshotStore.buildSnapshot(
          context,
          inspection,
          {},
          {
            processLifecycle: "stopped",
            processOwnership: "none",
            statusHint: inspection.canBootstrapUserConfig
              ? "服务尚未启动。Launcher 会在启动服务前基于 default.yaml 生成首份用户配置。"
              : buildLocalDetail("服务尚未启动。", inspection.preflightChecks),
            lastLocalError: "",
            localRecoverySummary: await tryReadLocalRecoverySummary(),
          },
        ),
      );
      return;
    }

    const healthy = await deps.managementClient.isHealthy(context.endpoint);
    if (!healthy) {
      const watcherActive = deps.developmentServerWatcher?.isActive() ?? false;
      if (watcherActive && !deps.processController.isRunning) {
        const previous = deps.snapshotStore.snapshot;
        if (
          previous.launcher.processLifecycle !== "starting"
          || previous.launcher.processOwnership !== "external"
        ) {
          writeWatcherTransitionLog(
            `开发 watcher 管理的服务暂时不可用，正在等待自动重启（watcher PID ${deps.developmentServerWatcher?.processId ?? "unknown"}，服务地址 ${context.endpoint.baseUrl}）。`,
            context,
          );
        }
        await deps.snapshotStore.publish(
          deps.snapshotStore.buildSnapshot(
            context,
            inspection,
            {},
            {
              processLifecycle: "starting",
              processOwnership: "external",
              statusHint: watcherRestartHint(),
              lastLocalError: "",
              localRecoverySummary: await tryReadLocalRecoverySummary(),
            },
          ),
        );
        return;
      }
      await deps.snapshotStore.publish(
        deps.snapshotStore.buildSnapshot(
          context,
          inspection,
          {},
          {
            processLifecycle: deps.processController.isRunning ? "running" : "stopped",
            processOwnership: deps.processController.isRunning ? "launcher_managed" : "none",
            statusHint: deps.processController.isRunning ? "子进程仍在运行，但健康检查失败。" : "服务尚未启动。",
            lastLocalError: deps.processController.isRunning ? "健康检查失败。" : "",
            localRecoverySummary: await tryReadLocalRecoverySummary(),
          },
        ),
      );
      return;
    }

    let readiness: LauncherReadinessSnapshot;
    try {
      readiness = await deps.managementClient.getReadiness(context.endpoint);
    } catch (error) {
      const detail = error instanceof Error ? error.message : "无法读取 /readyz。";
      await deps.snapshotStore.publish(
        deps.snapshotStore.buildSnapshot(
          context,
          inspection,
          {
            health: { status: "ok" },
          },
          {
            processLifecycle: deps.processController.isRunning ? "running" : "stopped",
            processOwnership: deps.processController.isRunning ? "launcher_managed" : "external",
            statusHint: "服务存活，但无法读取正式就绪状态。",
            lastLocalError: detail,
            localRecoverySummary: await tryReadLocalRecoverySummary(),
          },
        ),
      );
      return;
    }

    await deps.snapshotStore.publish(await buildSnapshotFromReadiness(context, inspection, readiness, _forceReauthentication));
  }

  return {
    refresh,
    buildSnapshotFromReadiness,
  };
}
