import { resolveRecoverySummary } from "../../shared/launcher-presentation";
import { sanitizeLauncherWebTargetPath } from "../../shared/launcher-validation";
import type {
  ExternalOpener,
  LauncherDesktopActions,
  LauncherManagementClient,
  LauncherRuntimeContext,
  LauncherSnapshotStore,
  ServerProcessController,
} from "./launcher-coordinator.types";

interface LauncherDesktopActionsDependencies {
  runtimeContext: LauncherRuntimeContext;
  snapshotStore: LauncherSnapshotStore;
  managementClient: LauncherManagementClient;
  externalOpener: ExternalOpener;
  processController: ServerProcessController;
}

export function createLauncherDesktopActions(deps: LauncherDesktopActionsDependencies): LauncherDesktopActions {
  async function openWebUi(targetPath = "") {
    const context = await deps.runtimeContext.createOperationContext();
    const normalizedTarget = sanitizeLauncherWebTargetPath(targetPath);
    const url = normalizedTarget ? new URL(normalizedTarget, context.endpoint.baseUrl) : new URL(context.endpoint.baseUrl);
    let initialized = false;

    try {
      initialized = await deps.managementClient.getSetupInitialized(context.endpoint);
    } catch {
      initialized = false;
    }

    if (initialized) {
      try {
        const launcherToken = await deps.managementClient.issueLauncherToken(context.endpoint);
        url.searchParams.set("token", launcherToken);
      } catch {
        if (normalizedTarget) {
          const fallbackURL = new URL(normalizedTarget, context.endpoint.baseUrl);
          await deps.externalOpener.openUri(fallbackURL.toString());
          return;
        }
        url.search = "";
      }
    }

    await deps.externalOpener.openUri(url.toString());
  }

  return {
    openWebUi,
    async createRecoveryRecheck() {
      if (!resolveRecoverySummary(deps.snapshotStore.snapshot)) {
        return;
      }
      const context = await deps.runtimeContext.createOperationContext();
      const token = await deps.runtimeContext.ensureSessionToken(context.endpoint);
      const existingTask = await deps.managementClient.findInProgressTask(context.endpoint, token, "recovery.recheck");
      const taskId = existingTask?.task_id
        ?? (await deps.managementClient.createRecoveryRecheck(context.endpoint, token)).task_id;
      await openWebUi(`/tasks?task_id=${encodeURIComponent(taskId)}`);
    },
    async createRuntimeBootstrap(resources) {
      const context = await deps.runtimeContext.createOperationContext();
      const token = await deps.runtimeContext.ensureSessionToken(context.endpoint);
      const existingTask = await deps.managementClient.findInProgressTask(context.endpoint, token, "runtime.bootstrap");
      const taskId = existingTask?.task_id
        ?? (await deps.managementClient.createRuntimeBootstrap(context.endpoint, token, resources)).task_id;
      await openWebUi(`/tasks?task_id=${encodeURIComponent(taskId)}`);
    },
    async openReleasePage() {
      if (!deps.snapshotStore.snapshot.launcher.releaseCheck.releasePageUrl) {
        await deps.snapshotStore.publish({
          ...deps.snapshotStore.snapshot,
          launcher: {
            ...deps.snapshotStore.snapshot.launcher,
            statusHint: "当前运行没有可打开的发布页。",
            lastLocalError: "",
          },
        });
        return;
      }
      await deps.externalOpener.openUri(deps.snapshotStore.snapshot.launcher.releaseCheck.releasePageUrl);
    },
    async openLogsDirectory() {
      await deps.externalOpener.openDirectory(deps.processController.logDirectory);
    },
  };
}
