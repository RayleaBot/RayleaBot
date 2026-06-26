import { createReleaseUnavailable } from "../../shared/launcher-copy";
import type {
  LauncherOperationContext,
  ServerProcessController,
} from "./launcher-coordinator.types";
import type {
  LauncherProcessLifecycle,
  LauncherResolvedSettings,
  LauncherSnapshot,
  ReleaseCheckSnapshot,
} from "../../shared/launcher-models";
import type { LauncherSnapshotStore } from "./launcher-coordinator.types";

interface LauncherSnapshotStoreDependencies {
  processController: ServerProcessController;
}

function defaultResolvedSettings(): LauncherResolvedSettings {
  return {
    installationRoot: "",
    serverExecutablePath: "",
    configPath: "",
    workdir: "",
  };
}

function defaultSnapshot(
  context: LauncherOperationContext = {
    settings: {
      installationRoot: "",
      closeBehavior: "ask_every_time",
    },
    resolvedSettings: defaultResolvedSettings(),
    endpoint: { host: "127.0.0.1", port: 8080, baseUrl: "http://127.0.0.1:8080/" },
  },
): LauncherSnapshot {
  return {
    server: {
      health: null,
      readiness: null,
      systemStatus: null,
    },
    launcher: {
      processId: null,
      processLifecycle: "stopped",
      processOwnership: "none",
      environmentChecks: [],
      preflightChecks: [],
      advisoryChecks: [],
      recentStderr: [],
      runtimePrepare: null,
      releaseCheck: createReleaseUnavailable(),
      lastLocalError: "",
      statusHint: "",
      settings: context.settings,
      resolvedSettings: context.resolvedSettings,
      endpoint: context.endpoint,
      localRecoverySummary: null,
    },
  };
}

function releaseChecksEqual(left: ReleaseCheckSnapshot, right: ReleaseCheckSnapshot) {
  return left.status === right.status
    && left.currentVersion === right.currentVersion
    && left.latestVersion === right.latestVersion
    && left.summary === right.summary
    && left.detail === right.detail
    && left.releasePageUrl === right.releasePageUrl
    && left.updateAvailable === right.updateAvailable
    && left.downloadProgress === right.downloadProgress
    && left.downloadedBytes === right.downloadedBytes
    && left.totalBytes === right.totalBytes
    && left.artifactFileName === right.artifactFileName
    && left.canCheck === right.canCheck
    && left.canDownload === right.canDownload
    && left.canInstall === right.canInstall;
}

function currentProcessLifecycle(
  processController: ServerProcessController,
  fallback: LauncherProcessLifecycle = "stopped",
) {
  if (fallback === "starting" || fallback === "stopping") {
    return fallback;
  }
  return processController.isRunning ? "running" : "stopped";
}

function getRuntimePrepareSnapshot(processController: ServerProcessController) {
  const maybeProcessController = processController as Partial<ServerProcessController>;
  return typeof maybeProcessController.getRuntimePrepareSnapshot === "function"
    ? maybeProcessController.getRuntimePrepareSnapshot()
    : null;
}

export function createLauncherSnapshotStore(deps: LauncherSnapshotStoreDependencies): LauncherSnapshotStore {
  const listeners = new Set<(snapshot: LauncherSnapshot) => void>();
  let snapshot = defaultSnapshot();

  function notify() {
    for (const listener of listeners) {
      listener(snapshot);
    }
  }

  return {
    get snapshot() {
      return snapshot;
    },
    reset(context) {
      snapshot = defaultSnapshot(context);
    },
    subscribe(listener) {
      listeners.add(listener);
      listener(snapshot);
      return () => listeners.delete(listener);
    },
    buildSnapshot(context, inspection, server = {}, launcherOverrides = {}) {
      return {
        server: {
          health: server.health ?? null,
          readiness: server.readiness ?? null,
          systemStatus: server.systemStatus ?? null,
        },
        launcher: {
          processId: deps.processController.isRunning ? deps.processController.processId : null,
          processLifecycle: currentProcessLifecycle(deps.processController, launcherOverrides.processLifecycle),
          processOwnership: launcherOverrides.processOwnership ?? snapshot.launcher.processOwnership ?? "none",
          environmentChecks: inspection.checks,
          preflightChecks: inspection.preflightChecks,
          advisoryChecks: inspection.advisoryChecks,
          recentStderr: deps.processController.getRecentStderr(),
          runtimePrepare: launcherOverrides.runtimePrepare === undefined
            ? getRuntimePrepareSnapshot(deps.processController)
            : launcherOverrides.runtimePrepare,
          releaseCheck: snapshot.launcher.releaseCheck,
          lastLocalError: launcherOverrides.lastLocalError ?? "",
          statusHint: launcherOverrides.statusHint ?? "",
          settings: context.settings,
          resolvedSettings: context.resolvedSettings,
          endpoint: context.endpoint,
          localRecoverySummary: launcherOverrides.localRecoverySummary ?? snapshot.launcher.localRecoverySummary ?? null,
        },
      };
    },
    async publish(next) {
      snapshot = next;
      notify();
    },
    async publishReleaseCheck(releaseCheck) {
      if (releaseChecksEqual(snapshot.launcher.releaseCheck, releaseCheck)) {
        return;
      }
      snapshot = {
        ...snapshot,
        launcher: {
          ...snapshot.launcher,
          releaseCheck,
        },
      };
      notify();
    },
  };
}
