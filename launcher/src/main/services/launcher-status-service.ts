import { buildLocalDetail } from "../../shared/launcher-presentation";
import type {
  LauncherManagementClient,
  LauncherOperationContext,
  LauncherSnapshotStore,
  LauncherStatusService,
  LauncherRuntimeContext,
  RecoverySummaryReader,
  ServerProcessController,
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
  recoverySummaryReader?: RecoverySummaryReader;
}

export function createLauncherStatusService(deps: LauncherStatusServiceDependencies): LauncherStatusService {
  async function tryLoadSystemStatus(endpoint: LauncherOperationContext["endpoint"]) {
    try {
      const token = await deps.runtimeContext.ensureSessionToken(endpoint);
      return await deps.managementClient.getSystemStatus(endpoint, token);
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
    forceReauthentication: boolean,
  ): Promise<LauncherSnapshot> {
    if (forceReauthentication) {
      deps.runtimeContext.clearSessionToken();
    }

    const systemStatus =
      readiness.status === "ready" || readiness.status === "degraded"
        ? await tryLoadSystemStatus(context.endpoint)
        : null;
    const processOwnership = deps.processController.isRunning ? "launcher_managed" : "external";
    const localRecoverySummary =
      systemStatus?.recovery_summary
      ?? readiness.recovery_summary
      ?? await tryReadLocalRecoverySummary();

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

  async function refresh(forceReauthentication: boolean) {
    if (forceReauthentication) {
      deps.runtimeContext.clearSessionToken();
    }

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
              ? "服务尚未启动。启动服务后会基于 default.yaml 生成首份用户配置。"
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

    await deps.snapshotStore.publish(await buildSnapshotFromReadiness(context, inspection, readiness, forceReauthentication));
  }

  return {
    refresh,
    buildSnapshotFromReadiness,
  };
}
