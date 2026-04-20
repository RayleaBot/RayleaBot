import { buildLocalDetail, startingDetail } from "../../shared/launcher-presentation";
import type {
  EnvironmentInspection,
  LauncherCoordinatorOptions,
  LauncherLifecycleService,
  LauncherManagementClient,
  LauncherResetAdminRunner,
  LauncherRuntimeContext,
  LauncherSnapshotStore,
  LauncherStatusService,
  ServerProcessController,
  ExternalOpener,
} from "./launcher-coordinator.types";
import type {
  LauncherReadinessSnapshot,
  LauncherResolvedSettings,
  ServerEndpoint,
} from "../../shared/launcher-models";

interface LauncherLifecycleServiceDependencies {
  runtimeContext: LauncherRuntimeContext;
  snapshotStore: LauncherSnapshotStore;
  statusService: LauncherStatusService;
  inspectEnvironment(settings: LauncherResolvedSettings): Promise<EnvironmentInspection>;
  managementClient: LauncherManagementClient;
  processController: ServerProcessController;
  isEndpointListening(endpoint: ServerEndpoint): Promise<boolean>;
  tryStopEndpointProcess(endpoint: ServerEndpoint): Promise<boolean>;
  externalOpener: ExternalOpener;
  confirmExternalServiceStop?(): Promise<boolean>;
  resetAdminRunner?: LauncherResetAdminRunner;
  options: Required<LauncherCoordinatorOptions>;
}

function delay(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

export function createLauncherLifecycleService(deps: LauncherLifecycleServiceDependencies): LauncherLifecycleService {
  async function waitForReadinessStatus(
    endpoint: ServerEndpoint,
    expectedStatus: LauncherReadinessSnapshot["status"],
    timeoutMs: number,
  ) {
    const deadline = Date.now() + timeoutMs;

    while (Date.now() < deadline) {
      if (!deps.processController.isRunning) {
        return null;
      }

      if (await deps.managementClient.isHealthy(endpoint).catch(() => false)) {
        try {
          const readiness = await deps.managementClient.getReadiness(endpoint);
          if (readiness.status === expectedStatus) {
            return readiness;
          }
        } catch {
          // Keep polling until the server recovers enough to expose /readyz.
        }
      }

      await delay(deps.options.pollIntervalMs);
    }

    return null;
  }

  async function ensureManagedProcessStopped() {
    if (!deps.processController.isRunning) {
      return;
    }

    const stopDeadline = Date.now() + deps.options.shutdownTimeoutMs;
    while (deps.processController.isRunning && Date.now() < stopDeadline) {
      await delay(deps.options.pollIntervalMs);
    }

    if (deps.processController.isRunning) {
      await deps.processController.forceKill();
    }
  }

  return {
    async start() {
      const context = await deps.runtimeContext.createOperationContext();
      const inspection = await deps.inspectEnvironment(context.resolvedSettings);

      if (inspection.hasBlockingIssues && !inspection.canBootstrapUserConfig) {
        await deps.snapshotStore.publish(
          deps.snapshotStore.buildSnapshot(
            context,
            inspection,
            {},
            {
              processLifecycle: "stopped",
              processOwnership: "none",
              statusHint: buildLocalDetail("启动器预检发现阻塞项。", inspection.preflightChecks),
              lastLocalError: "",
            },
          ),
        );
        return;
      }

      if (await deps.managementClient.isHealthy(context.endpoint)) {
        await deps.statusService.refresh(true);
        return;
      }

      if ((await deps.isEndpointListening(context.endpoint)) && !deps.processController.isRunning) {
        await deps.snapshotStore.publish(
          deps.snapshotStore.buildSnapshot(
            context,
            inspection,
            {},
            {
              processLifecycle: "stopped",
              processOwnership: "none",
              statusHint: "目标端口已被现有进程占用，启动器不会重复拉起服务。",
              lastLocalError: `端口 ${context.endpoint.port} 已被占用。`,
            },
          ),
        );
        return;
      }

      try {
        await deps.processController.start(context.resolvedSettings);
      } catch (error) {
        const detail = error instanceof Error ? error.message : "服务进程启动失败。";
        await deps.snapshotStore.publish(
          deps.snapshotStore.buildSnapshot(
            context,
            inspection,
            {},
            {
              processLifecycle: "stopped",
              processOwnership: "none",
              statusHint: "无法启动服务进程。",
              lastLocalError: detail,
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
            processLifecycle: "starting",
            processOwnership: "launcher_managed",
            statusHint: startingDetail(inspection.canBootstrapUserConfig),
            lastLocalError: "",
          },
        ),
      );

      const startedAt = Date.now();
      let firstFailedReadinessAt: number | null = null;
      let lastFailedReadiness: LauncherReadinessSnapshot | null = null;
      while (Date.now() - startedAt < deps.options.startupTimeoutMs) {
        if (await deps.managementClient.isHealthy(context.endpoint)) {
          try {
            const readiness = await deps.managementClient.getReadiness(context.endpoint);
            if (readiness.status === "failed") {
              lastFailedReadiness = readiness;
              if (firstFailedReadinessAt === null) {
                firstFailedReadinessAt = Date.now();
              }
              if (Date.now() - firstFailedReadinessAt < deps.options.startupReadinessGraceMs) {
                await delay(deps.options.pollIntervalMs);
                continue;
              }
            } else {
              firstFailedReadinessAt = null;
              lastFailedReadiness = null;
            }
            await deps.snapshotStore.publish(await deps.statusService.buildSnapshotFromReadiness(context, inspection, readiness, true));
            return;
          } catch {
            // Keep polling until /readyz becomes readable. A transient healthz success alone
            // should not lock the launcher into a failed state during restart windows.
          }
        }
        if (!deps.processController.isRunning) {
          const lastError = deps.processController.getRecentStderr().at(-1) ?? "服务进程在通过健康检查前已退出。";
          if (await deps.managementClient.isHealthy(context.endpoint).catch(() => false)) {
            await deps.statusService.refresh(true);
            return;
          }
          if (await deps.isEndpointListening(context.endpoint).catch(() => false)) {
            await deps.snapshotStore.publish(
              deps.snapshotStore.buildSnapshot(
                context,
                inspection,
                {},
                {
                  processLifecycle: "stopped",
                  processOwnership: "none",
                  statusHint: "目标端口已被现有进程占用，RayleaBot 无法监听当前地址。",
                  lastLocalError: lastError,
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
                processLifecycle: "stopped",
                processOwnership: "none",
                statusHint: "服务进程在启动阶段提前退出。",
                lastLocalError: lastError,
              },
            ),
          );
          return;
        }
        await delay(deps.options.pollIntervalMs);
      }

      if (lastFailedReadiness) {
        await deps.snapshotStore.publish(await deps.statusService.buildSnapshotFromReadiness(context, inspection, lastFailedReadiness, true));
        return;
      }

      await deps.processController.forceKill();
      await deps.snapshotStore.publish(
        deps.snapshotStore.buildSnapshot(
          context,
          inspection,
          {},
          {
            processLifecycle: "stopped",
            processOwnership: "none",
            statusHint: "启动超时内未通过健康检查。",
            lastLocalError: "服务启动已超时。",
          },
        ),
      );
    },
    async stop() {
      const context = await deps.runtimeContext.createOperationContext();
      const inspection = await deps.inspectEnvironment(context.resolvedSettings);
      const healthy = await deps.managementClient.isHealthy(context.endpoint).catch(() => false);
      const isManagedProcess = deps.processController.isRunning;
      const processOwnership =
        isManagedProcess ? "launcher_managed"
          : healthy ? "external"
            : "none";

      if (healthy && processOwnership === "external") {
        const confirmed = await deps.confirmExternalServiceStop?.() ?? false;
        if (!confirmed) {
          await deps.statusService.refresh(false);
          return;
        }

        await deps.snapshotStore.publish(
          deps.snapshotStore.buildSnapshot(
            context,
            inspection,
            {
              health: { status: "ok" },
            },
            {
              processLifecycle: "stopping",
              processOwnership: "external",
              statusHint: "正在停止现有服务。",
              lastLocalError: "",
            },
          ),
        );

        try {
          if (!await deps.managementClient.getSetupInitialized(context.endpoint)) {
            throw new Error("检测到现有服务仍处于初始化阶段，无法由启动器停止。");
          }

          deps.runtimeContext.clearSessionToken();
          const token = await deps.runtimeContext.ensureSessionToken(context.endpoint);
          await deps.managementClient.shutdown(context.endpoint, token);
          deps.runtimeContext.clearSessionToken();
          await deps.statusService.refresh(true);
        } catch (error) {
          deps.runtimeContext.clearSessionToken();
          await deps.statusService.refresh(true);
          await deps.snapshotStore.publish({
            ...deps.snapshotStore.snapshot,
            launcher: {
              ...deps.snapshotStore.snapshot.launcher,
              lastLocalError: error instanceof Error ? error.message : "无法停止检测到的现有服务。",
              statusHint: "无法停止检测到的现有服务。",
            },
          });
        }
        return;
      }

      await deps.snapshotStore.publish(
        deps.snapshotStore.buildSnapshot(
          context,
          inspection,
          healthy ? { health: { status: "ok" } } : {},
          {
            processLifecycle: "stopping",
            processOwnership,
            statusHint: isManagedProcess ? "正在停止服务。" : "正在停止现有服务。",
            lastLocalError: "",
          },
        ),
      );

      if (healthy) {
        try {
          if (await deps.managementClient.getSetupInitialized(context.endpoint)) {
            const token = await deps.runtimeContext.ensureSessionToken(context.endpoint);
            await deps.managementClient.shutdown(context.endpoint, token);
          } else if (deps.processController.isRunning) {
            await deps.processController.forceKill();
          } else {
            await deps.tryStopEndpointProcess(context.endpoint);
          }
        } catch {
          if (deps.processController.isRunning) {
            await deps.processController.forceKill();
          } else {
            await deps.tryStopEndpointProcess(context.endpoint);
          }
        }
      }

      await ensureManagedProcessStopped();
      deps.runtimeContext.clearSessionToken();
      await deps.statusService.refresh(true);
    },
    async resetAdmin() {
      if (!deps.resetAdminRunner) {
        throw new Error("管理员重置功能不可用。");
      }

      const context = await deps.runtimeContext.createOperationContext();
      const inspection = await deps.inspectEnvironment(context.resolvedSettings);

      if (deps.processController.isRunning || (await deps.managementClient.isHealthy(context.endpoint).catch(() => false))) {
        await deps.snapshotStore.publish(
          deps.snapshotStore.buildSnapshot(
            context,
            inspection,
            {},
            {
              processLifecycle: "stopping",
              processOwnership: deps.processController.isRunning ? "launcher_managed" : "external",
              statusHint: "正在停止服务以执行管理员重置。",
            },
          ),
        );
        if (deps.processController.isRunning) {
          await deps.processController.forceKill();
        } else {
          await deps.tryStopEndpointProcess(context.endpoint);
        }
        await ensureManagedProcessStopped();
      }

      await deps.resetAdminRunner.run(context.resolvedSettings);
      deps.runtimeContext.clearSessionToken();

      try {
        await deps.processController.start(context.resolvedSettings);
      } catch (error) {
        const detail = error instanceof Error ? error.message : "服务进程启动失败。";
        await deps.snapshotStore.publish(
          deps.snapshotStore.buildSnapshot(
            context,
            inspection,
            {},
            {
              processLifecycle: "stopped",
              processOwnership: "none",
              statusHint: "管理员凭据已重置，但服务重启失败。",
              lastLocalError: detail,
            },
          ),
        );
        return;
      }

      const readiness = await waitForReadinessStatus(context.endpoint, "setup_required", deps.options.resetAdminTimeoutMs);
      if (!readiness) {
        const detail = deps.processController.getRecentStderr().at(-1) ?? "服务未在预期时间内恢复到初始化状态。";
        await deps.snapshotStore.publish(
          deps.snapshotStore.buildSnapshot(
            context,
            inspection,
            {},
            {
              processLifecycle: deps.processController.isRunning ? "running" : "stopped",
              processOwnership: deps.processController.isRunning ? "launcher_managed" : "none",
              statusHint: "管理员凭据已重置，但服务未进入初始化状态。",
              lastLocalError: detail,
            },
          ),
        );
        return;
      }

      await deps.snapshotStore.publish(await deps.statusService.buildSnapshotFromReadiness(context, inspection, readiness, true));
      await deps.externalOpener.openUri(new URL(context.endpoint.baseUrl).toString());
    },
  };
}
