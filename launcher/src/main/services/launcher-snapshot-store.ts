import { createReleaseUnavailable } from "../../shared/launcher-copy";
import type {
  LauncherOperationContext,
  ReleaseFeedClient,
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
  releaseFeedClient?: ReleaseFeedClient;
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

async function withReleaseCheck(
  releaseFeedClient: ReleaseFeedClient | undefined,
  current: ReleaseCheckSnapshot,
): Promise<ReleaseCheckSnapshot> {
  if (!releaseFeedClient) {
    return current;
  }
  try {
    return await releaseFeedClient.getSnapshot();
  } catch (error) {
    const detail = error instanceof Error ? error.message : "release feed unavailable";
    return {
      ...current,
      status: "error",
      summary: "暂时无法连接版本源。",
      detail,
      updateAvailable: false,
    };
  }
}

function releaseChecksEqual(left: ReleaseCheckSnapshot, right: ReleaseCheckSnapshot) {
  return left.status === right.status
    && left.currentVersion === right.currentVersion
    && left.latestVersion === right.latestVersion
    && left.summary === right.summary
    && left.detail === right.detail
    && left.releasePageUrl === right.releasePageUrl
    && left.updateAvailable === right.updateAvailable;
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

export function createLauncherSnapshotStore(deps: LauncherSnapshotStoreDependencies): LauncherSnapshotStore {
  const listeners = new Set<(snapshot: LauncherSnapshot) => void>();
  let snapshot = defaultSnapshot();
  let releaseCheckInFlight: Promise<void> | null = null;

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
      for (const listener of listeners) {
        listener(snapshot);
      }
      if (!deps.releaseFeedClient || releaseCheckInFlight) {
        return;
      }

      releaseCheckInFlight = withReleaseCheck(deps.releaseFeedClient, snapshot.launcher.releaseCheck)
        .then((releaseCheck) => {
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
          for (const listener of listeners) {
            listener(snapshot);
          }
        })
        .finally(() => {
          releaseCheckInFlight = null;
        });
    },
  };
}
