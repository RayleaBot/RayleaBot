import { createLauncherDesktopActions } from "./launcher-desktop-actions";
import { createLauncherLifecycleService } from "./launcher-lifecycle-service";
import { createLauncherSnapshotStore } from "./launcher-snapshot-store";
import { createLauncherStatusService } from "./launcher-status-service";
import type {
  LauncherCoordinator,
  LauncherCoordinatorDependencies,
} from "./launcher-coordinator.types";
import { createLauncherRuntimeContext } from "./launcher-runtime-context";
import type { LauncherSnapshot } from "../../shared/launcher-models";

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
  };

  const runtimeContext = createLauncherRuntimeContext({
    settingsStore: deps.settingsStore,
    endpointResolver: deps.endpointResolver,
  });
  const snapshotStore = createLauncherSnapshotStore({
    processController: deps.processController,
    releaseFeedClient: deps.releaseFeedClient,
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

  function clearAutoRefreshTimer() {
    if (!autoRefreshTimer) {
      return;
    }
    clearTimeout(autoRefreshTimer);
    autoRefreshTimer = null;
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
    async openWebUi(targetPath = "") {
      await desktopActions.openWebUi(targetPath);
    },
    async openReleasePage() {
      await desktopActions.openReleasePage();
    },
    async openLogsDirectory() {
      await desktopActions.openLogsDirectory();
    },
  };
}
