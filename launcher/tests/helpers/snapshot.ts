import type { LauncherSnapshot } from "@shared/launcher-models";

type DeepPartial<T> = {
  [K in keyof T]?: T[K] extends object ? DeepPartial<T[K]> : T[K];
};

export function createLauncherSnapshot(overrides: DeepPartial<LauncherSnapshot> = {}): LauncherSnapshot {
  return {
    server: {
      health: null,
      readiness: null,
      systemStatus: null,
      ...overrides.server,
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
      releaseCheck: {
        status: "unavailable",
        currentVersion: "",
        latestVersion: "",
        summary: "版本信息不可用",
        detail: "",
        releasePageUrl: "",
        updateAvailable: false,
        downloadProgress: null,
        downloadedBytes: null,
        totalBytes: null,
        artifactFileName: "",
        canCheck: false,
        canDownload: false,
        canInstall: false,
        ...overrides.launcher?.releaseCheck,
      },
      lastLocalError: "",
      statusHint: "",
      settings: {
        installationRoot: "",
        closeBehavior: "ask_every_time",
        ...overrides.launcher?.settings,
      },
      resolvedSettings: {
        installationRoot: "",
        serverExecutablePath: "",
        configPath: "",
        workdir: "",
        ...overrides.launcher?.resolvedSettings,
      },
      endpoint: {
        host: "127.0.0.1",
        port: 8080,
        baseUrl: "http://127.0.0.1:8080/",
        ...overrides.launcher?.endpoint,
      },
      localRecoverySummary: null,
      ...overrides.launcher,
    },
  };
}
