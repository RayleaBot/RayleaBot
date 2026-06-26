import { createLauncherDesktopActions } from "./launcher-desktop-actions";
import { createLauncherLifecycleService } from "./launcher-lifecycle-service";
import { createLauncherSnapshotStore } from "./launcher-snapshot-store";
import { createLauncherStatusService } from "./launcher-status-service";
import type {
  LauncherCoordinator,
  LauncherCoordinatorDependencies,
} from "./launcher-coordinator.types";
import { createLauncherRuntimeContext } from "./launcher-runtime-context";
import type { LauncherSnapshot, ReleaseCheckSnapshot } from "../../shared/launcher-models";

export type {
  EnvironmentCheckResult,
  EnvironmentInspection,
  ExternalOpener,
  LauncherCoordinator,
  LauncherCoordinatorDependencies,
  LauncherCoordinatorOptions,
  LauncherDesktopActions,
  LauncherLifecycleService,
  LauncherManagementClient,
  LauncherOperationContext,
  LauncherResetAdminRunner,
  LauncherRuntimeContext,
  LauncherSettings,
  LauncherSettingsStore,
  LauncherSnapshotStore,
  LauncherStatusService,
  RecoverySummaryReader,
  ReleaseFeedClient,
  ServerEndpoint,
  ServerEndpointResolver,
  ServerProcessController,
} from "./launcher-coordinator.types";

export function createLauncherCoordinator(deps: LauncherCoordinatorDependencies): LauncherCoordinator {
  const options = {
    startupTimeoutMs: deps.options?.startupTimeoutMs ?? 900000,
    startupReadinessGraceMs: deps.options?.startupReadinessGraceMs ?? 10000,
    pollIntervalMs: deps.options?.pollIntervalMs ?? 500,
    shutdownTimeoutMs: deps.options?.shutdownTimeoutMs ?? 5000,
    resetAdminTimeoutMs: deps.options?.resetAdminTimeoutMs ?? 30000,
    autoRefreshIntervalMs: deps.options?.autoRefreshIntervalMs ?? 2000,
    releaseCheckIntervalMs: deps.options?.releaseCheckIntervalMs ?? 6 * 60 * 60 * 1000,
  };

  const runtimeContext = createLauncherRuntimeContext({
    settingsStore: deps.settingsStore,
    endpointResolver: deps.endpointResolver,
  });
  const snapshotStore = createLauncherSnapshotStore({
    processController: deps.processController,
  });
  const statusService = createLauncherStatusService({
    runtimeContext,
    snapshotStore,
    inspectEnvironment: deps.inspectEnvironment,
    managementClient: deps.managementClient,
    processController: deps.processController,
    recoverySummaryReader: deps.recoverySummaryReader,
  });
  const lifecycleService = createLauncherLifecycleService({
    runtimeContext,
    snapshotStore,
    statusService,
    inspectEnvironment: deps.inspectEnvironment,
    managementClient: deps.managementClient,
    processController: deps.processController,
    isEndpointListening: deps.isEndpointListening,
    tryStopEndpointProcess: deps.tryStopEndpointProcess,
    externalOpener: deps.externalOpener,
    confirmExternalServiceStop: deps.confirmExternalServiceStop,
    resetAdminRunner: deps.resetAdminRunner,
    options,
  });
  const desktopActions = createLauncherDesktopActions({
    runtimeContext,
    snapshotStore,
    externalOpener: deps.externalOpener,
    processController: deps.processController,
  });
  let autoRefreshTimer: ReturnType<typeof setTimeout> | null = null;
  let autoRefreshInFlight = false;
  let releaseCheckTimer: ReturnType<typeof setTimeout> | null = null;
  let releaseCheckInFlight: Promise<void> | null = null;
  let releaseDownloadInFlight = false;
  let releaseInstallInFlight = false;

  function clearAutoRefreshTimer() {
    if (!autoRefreshTimer) {
      return;
    }
    clearTimeout(autoRefreshTimer);
    autoRefreshTimer = null;
  }

  function clearReleaseCheckTimer() {
    if (!releaseCheckTimer) {
      return;
    }
    clearTimeout(releaseCheckTimer);
    releaseCheckTimer = null;
  }

  function shouldAutoRefresh(snapshot: LauncherSnapshot) {
    return snapshot.launcher.processLifecycle === "running"
      || snapshot.server.health?.status === "ok"
      || snapshot.server.readiness !== null;
  }

  function scheduleAutoRefresh() {
    clearAutoRefreshTimer();
    if (autoRefreshInFlight || options.autoRefreshIntervalMs <= 0 || !shouldAutoRefresh(snapshotStore.snapshot)) {
      return;
    }
    autoRefreshTimer = setTimeout(async () => {
      autoRefreshTimer = null;
      await runAutoRefresh();
    }, options.autoRefreshIntervalMs);
    if (typeof autoRefreshTimer === "object" && autoRefreshTimer && "unref" in autoRefreshTimer) {
      autoRefreshTimer.unref();
    }
  }

  function releaseBusy() {
    const status = snapshotStore.snapshot.launcher.releaseCheck.status;
    return releaseCheckInFlight !== null
      || releaseDownloadInFlight
      || releaseInstallInFlight
      || status === "checking"
      || status === "downloading"
      || status === "installing";
  }

  function releaseCheckCanSchedule() {
    if (!deps.releaseFeedClient || options.releaseCheckIntervalMs <= 0) {
      return false;
    }
    const status = snapshotStore.snapshot.launcher.releaseCheck.status;
    return status !== "disabled"
      && status !== "downloaded"
      && status !== "downloading"
      && status !== "installing";
  }

  function scheduleReleaseCheck() {
    clearReleaseCheckTimer();
    if (!releaseCheckCanSchedule() || releaseBusy()) {
      return;
    }
    releaseCheckTimer = setTimeout(() => {
      releaseCheckTimer = null;
      void runReleaseCheck(false);
    }, options.releaseCheckIntervalMs);
    if (typeof releaseCheckTimer === "object" && releaseCheckTimer && "unref" in releaseCheckTimer) {
      releaseCheckTimer.unref();
    }
  }

  function checkingSnapshot(previous: ReleaseCheckSnapshot): ReleaseCheckSnapshot {
    return {
      ...previous,
      status: "checking",
      summary: "正在检查更新。",
      detail: "",
      downloadProgress: null,
      downloadedBytes: null,
      totalBytes: previous.totalBytes ?? null,
      canCheck: false,
      canDownload: false,
      canInstall: false,
    };
  }

  function releaseErrorSnapshot(previous: ReleaseCheckSnapshot, error: unknown): ReleaseCheckSnapshot {
    return {
      ...previous,
      status: "error",
      summary: "暂时无法连接版本源。",
      detail: error instanceof Error ? error.message : "版本源不可用。",
      updateAvailable: false,
      canCheck: Boolean(previous.currentVersion),
      canDownload: false,
      canInstall: false,
    };
  }

  async function runReleaseCheck(force: boolean) {
    if (!deps.releaseFeedClient || releaseBusy()) {
      return;
    }
    clearReleaseCheckTimer();
    const previous = snapshotStore.snapshot.launcher.releaseCheck;
    await snapshotStore.publishReleaseCheck(checkingSnapshot(previous));
    releaseCheckInFlight = (async () => {
      try {
        const releaseCheck = await deps.releaseFeedClient!.getSnapshot({ force });
        await snapshotStore.publishReleaseCheck(releaseCheck);
      } catch (error) {
        await snapshotStore.publishReleaseCheck(releaseErrorSnapshot(previous, error));
      } finally {
        releaseCheckInFlight = null;
        scheduleReleaseCheck();
      }
    })();
    await releaseCheckInFlight;
  }

  async function runReleaseDownload() {
    if (!deps.releaseFeedClient || releaseBusy()) {
      return;
    }
    clearReleaseCheckTimer();
    releaseDownloadInFlight = true;
    try {
      const releaseCheck = await deps.releaseFeedClient.downloadUpdate((snapshot) =>
        snapshotStore.publishReleaseCheck(snapshot),
      );
      await snapshotStore.publishReleaseCheck(releaseCheck);
    } finally {
      releaseDownloadInFlight = false;
      scheduleReleaseCheck();
    }
  }

  async function prepareUpdateInstall(appProcessId: number) {
    if (!deps.releaseFeedClient || releaseInstallInFlight) {
      return;
    }
    if (snapshotStore.snapshot.launcher.processOwnership === "external") {
      throw new Error("检测到外部服务正在运行，请先停止服务后再安装更新。");
    }
    clearReleaseCheckTimer();
    releaseInstallInFlight = true;
    try {
      if (deps.processController.isRunning) {
        await lifecycleService.stop();
      }
      const releaseCheck = await deps.releaseFeedClient.installDownloadedUpdate(appProcessId);
      await snapshotStore.publishReleaseCheck(releaseCheck);
      if (releaseCheck.status !== "installing") {
        throw new Error(releaseCheck.detail || releaseCheck.summary);
      }
    } finally {
      releaseInstallInFlight = false;
    }
  }

  async function runAutoRefresh() {
    if (autoRefreshInFlight || !shouldAutoRefresh(snapshotStore.snapshot)) {
      return;
    }
    autoRefreshInFlight = true;
    try {
      await statusService.refresh(false);
    } catch {
      // Keep the current snapshot and try again on the next interval.
    } finally {
      autoRefreshInFlight = false;
      scheduleAutoRefresh();
    }
  }

  snapshotStore.subscribe(() => {
    if (!autoRefreshInFlight) {
      scheduleAutoRefresh();
    }
  });

  return {
    get snapshot() {
      return snapshotStore.snapshot;
    },
    subscribe(listener) {
      return snapshotStore.subscribe(listener);
    },
    async initialize() {
      const context = await runtimeContext.initialize();
      snapshotStore.reset(context);
      await statusService.refresh(false);
      void runReleaseCheck(false);
    },
    async refresh() {
      await statusService.refresh(false);
    },
    async retry() {
      await statusService.refresh(true);
    },
    async saveSettings(settings) {
      await runtimeContext.saveSettings(settings);
      await statusService.refresh(true);
    },
    async start() {
      await lifecycleService.start();
    },
    async stop() {
      await lifecycleService.stop();
    },
    async resetAdmin() {
      await lifecycleService.resetAdmin();
    },
    async checkForUpdates() {
      await runReleaseCheck(true);
    },
    async downloadUpdate() {
      await runReleaseDownload();
    },
    async prepareUpdateInstall(appProcessId) {
      await prepareUpdateInstall(appProcessId);
    },
    async openWebUi(targetPath = "") {
      await desktopActions.openWebUi(targetPath);
    },
    async openReleasePage() {
      await desktopActions.openReleasePage();
    },
    async openRepositoryPage() {
      await desktopActions.openRepositoryPage();
    },
    async openLogsDirectory() {
      await desktopActions.openLogsDirectory();
    },
  };
}
