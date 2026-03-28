import { createReleaseUnavailable } from "../../shared/launcher-copy";
import type {
  EnvironmentCheckResult,
  EnvironmentInspection,
  LauncherSettings,
  LauncherSnapshot,
  ReleaseCheckSnapshot,
  ServerEndpoint,
  LauncherServiceState,
} from "../../shared/launcher-models";

export type { EnvironmentCheckResult, EnvironmentInspection, LauncherSettings, ServerEndpoint };

export interface LauncherSettingsStore {
  load(): Promise<LauncherSettings>;
  save(settings: LauncherSettings): Promise<void>;
}

export interface ServerEndpointResolver {
  resolve(configPath: string): ServerEndpoint;
}

export interface LauncherManagementClient {
  isHealthy(endpoint: ServerEndpoint): Promise<boolean>;
  getSetupInitialized(endpoint: ServerEndpoint): Promise<boolean>;
  issueLauncherToken(endpoint: ServerEndpoint): Promise<string>;
  admitLauncherToken(endpoint: ServerEndpoint, launcherToken: string): Promise<string>;
  shutdown(endpoint: ServerEndpoint, sessionToken: string): Promise<void>;
}

export interface ServerProcessController {
  isRunning: boolean;
  processId: number | null;
  logDirectory: string;
  start(settings: LauncherSettings): Promise<void>;
  forceKill(): Promise<void>;
  getRecentStderr(): string[];
}

export interface ExternalOpener {
  openUri(uri: string): Promise<void>;
  openDirectory(directoryPath: string): Promise<void>;
}

export interface ReleaseFeedClient {
  getSnapshot(): Promise<ReleaseCheckSnapshot>;
}

interface LauncherCoordinatorOptions {
  startupTimeoutMs?: number;
  pollIntervalMs?: number;
}

interface LauncherCoordinatorDependencies {
  settingsStore: LauncherSettingsStore;
  endpointResolver: ServerEndpointResolver;
  inspectEnvironment(settings: LauncherSettings): Promise<EnvironmentInspection>;
  managementClient: LauncherManagementClient;
  processController: ServerProcessController;
  isEndpointListening(endpoint: ServerEndpoint): Promise<boolean>;
  tryStopEndpointProcess(endpoint: ServerEndpoint): Promise<boolean>;
  externalOpener: ExternalOpener;
  releaseFeedClient?: ReleaseFeedClient;
  options?: LauncherCoordinatorOptions;
}

export interface LauncherCoordinator {
  snapshot: LauncherSnapshot;
  initialize(): Promise<void>;
  refresh(): Promise<void>;
  retry(): Promise<void>;
  start(): Promise<void>;
  stop(): Promise<void>;
  openWebUi(): Promise<void>;
  openReleasePage(): Promise<void>;
  openLogsDirectory(): Promise<void>;
  saveSettings(settings: LauncherSettings): Promise<void>;
  subscribe(listener: (snapshot: LauncherSnapshot) => void): () => void;
}

function delay(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function defaultSnapshot(settings: LauncherSettings, endpoint: ServerEndpoint): LauncherSnapshot {
  return {
    settings,
    endpoint,
    environmentChecks: [],
    recentStderr: [],
    processId: null,
    serviceState: "stopped",
    shutdownRequested: false,
    serviceDetail: "服务尚未启动。",
    lastError: "",
    releaseCheck: createReleaseUnavailable(),
  };
}

function primaryIssue(checks: EnvironmentCheckResult[]) {
  return checks.find((item) => item.severity === "error") ?? checks.find((item) => item.severity === "warning");
}

function buildLocalDetail(fallback: string, checks: EnvironmentCheckResult[]) {
  const issue = primaryIssue(checks);
  if (!issue) {
    return fallback;
  }
  const detail = issue.detail ? `${issue.summary} ${issue.detail}` : issue.summary;
  return issue.remediation ? `${detail} ${issue.remediation}` : detail;
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

export function createLauncherCoordinator(deps: LauncherCoordinatorDependencies): LauncherCoordinator {
  const listeners = new Set<(snapshot: LauncherSnapshot) => void>();
  const options = {
    startupTimeoutMs: deps.options?.startupTimeoutMs ?? 30000,
    pollIntervalMs: deps.options?.pollIntervalMs ?? 500,
  };

  let currentSettings: LauncherSettings | null = null;
  let sessionToken = "";
  let snapshot = defaultSnapshot(
    {
      serverExecutablePath: "",
      configPath: "",
      workdir: "",
      closeBehavior: "ask_every_time",
    },
    { host: "127.0.0.1", port: 8080, baseUrl: "http://127.0.0.1:8080/" },
  );

  async function publish(next: LauncherSnapshot) {
    snapshot = {
      ...next,
      releaseCheck: await withReleaseCheck(deps.releaseFeedClient, next.releaseCheck),
    };
    for (const listener of listeners) {
      listener(snapshot);
    }
  }

  function ensureSettings() {
    if (!currentSettings) {
      throw new Error("尚未加载启动器设置。");
    }
    return currentSettings;
  }

  async function buildSnapshot(
    endpoint: ServerEndpoint,
    inspection: EnvironmentInspection,
    serviceState: LauncherServiceState,
    serviceDetail: string,
    lastError = "",
  ) {
    const settings = ensureSettings();
    return {
      settings,
      endpoint,
      environmentChecks: inspection.checks,
      recentStderr: deps.processController.getRecentStderr(),
      processId: deps.processController.isRunning ? deps.processController.processId : null,
      serviceState,
      shutdownRequested: serviceState === "shutting_down",
      serviceDetail,
      lastError,
      releaseCheck: snapshot.releaseCheck,
    } satisfies LauncherSnapshot;
  }

  async function refreshCore(forceReauthentication: boolean) {
    if (forceReauthentication) {
      sessionToken = "";
    }
    const settings = ensureSettings();
    const endpoint = deps.endpointResolver.resolve(settings.configPath);
    const inspection = await deps.inspectEnvironment(settings);

    if (inspection.hasBlockingIssues || inspection.canBootstrapUserConfig) {
      await publish(
        await buildSnapshot(
          endpoint,
          inspection,
          "stopped",
          inspection.canBootstrapUserConfig
            ? "服务尚未启动。启动服务后会基于 default.yaml 生成首份用户配置。"
            : buildLocalDetail("服务尚未启动。", inspection.checks),
        ),
      );
      return;
    }

    const healthy = await deps.managementClient.isHealthy(endpoint);
    if (!healthy) {
      await publish(
        await buildSnapshot(
          endpoint,
          inspection,
          deps.processController.isRunning ? "failed" : "stopped",
          deps.processController.isRunning ? "子进程仍在运行，但健康检查失败。" : "服务尚未启动。",
          deps.processController.isRunning ? "健康检查失败。" : "",
        ),
      );
      return;
    }

    await publish(
      await buildSnapshot(
        endpoint,
        inspection,
        deps.processController.isRunning ? "ready" : "external_service",
        deps.processController.isRunning ? "服务正在运行。" : "端口上已有服务正在运行。可以直接打开管理界面，或先停止它再由启动器重新启动。",
      ),
    );
  }

  return {
    get snapshot() {
      return snapshot;
    },
    subscribe(listener) {
      listeners.add(listener);
      listener(snapshot);
      return () => listeners.delete(listener);
    },
    async initialize() {
      currentSettings = await deps.settingsStore.load();
      const endpoint = deps.endpointResolver.resolve(currentSettings.configPath);
      snapshot = defaultSnapshot(currentSettings, endpoint);
      await refreshCore(false);
    },
    async refresh() {
      await refreshCore(false);
    },
    async retry() {
      await refreshCore(true);
    },
    async saveSettings(settings) {
      currentSettings = settings;
      sessionToken = "";
      await deps.settingsStore.save(settings);
      await refreshCore(true);
    },
    async start() {
      const settings = ensureSettings();
      const endpoint = deps.endpointResolver.resolve(settings.configPath);
      const inspection = await deps.inspectEnvironment(settings);

      if (inspection.hasBlockingIssues && !inspection.canBootstrapUserConfig) {
        await publish(
          await buildSnapshot(endpoint, inspection, "stopped", buildLocalDetail("启动器预检发现阻塞项。", inspection.checks)),
        );
        return;
      }

      if (await deps.managementClient.isHealthy(endpoint)) {
        await refreshCore(true);
        return;
      }

      if ((await deps.isEndpointListening(endpoint)) && !deps.processController.isRunning) {
        await publish(
          await buildSnapshot(
            endpoint,
            inspection,
            "failed",
            "目标端口已被现有进程占用，启动器不会重复拉起服务。",
            `端口 ${endpoint.port} 已被占用。`,
          ),
        );
        return;
      }

      await deps.processController.start(settings);
      await publish(
        await buildSnapshot(
          endpoint,
          inspection,
          "starting",
          inspection.canBootstrapUserConfig
            ? "已基于 default.yaml 生成首份用户配置，正在等待 /healthz 返回正常。"
            : "正在等待 /healthz 返回正常。",
        ),
      );

      const startedAt = Date.now();
      while (Date.now() - startedAt < options.startupTimeoutMs) {
        if (await deps.managementClient.isHealthy(endpoint)) {
          await refreshCore(true);
          return;
        }
        await delay(options.pollIntervalMs);
      }

      await deps.processController.forceKill();
      await publish(await buildSnapshot(endpoint, inspection, "failed", "启动超时内未通过健康检查。", "服务启动已超时。"));
    },
    async stop() {
      const settings = ensureSettings();
      const endpoint = deps.endpointResolver.resolve(settings.configPath);
      const inspection = await deps.inspectEnvironment(settings);

      await publish(await buildSnapshot(endpoint, inspection, "shutting_down", deps.processController.isRunning ? "正在停止服务。" : "正在停止现有服务。"));

      if (await deps.managementClient.isHealthy(endpoint)) {
        if (await deps.managementClient.getSetupInitialized(endpoint)) {
          if (!sessionToken) {
            const launcherToken = await deps.managementClient.issueLauncherToken(endpoint);
            sessionToken = await deps.managementClient.admitLauncherToken(endpoint, launcherToken);
          }
          try {
            await deps.managementClient.shutdown(endpoint, sessionToken);
          } catch {
            if (deps.processController.isRunning) {
              await deps.processController.forceKill();
            } else {
              await deps.tryStopEndpointProcess(endpoint);
            }
          }
        } else if (deps.processController.isRunning) {
          await deps.processController.forceKill();
        } else {
          await deps.tryStopEndpointProcess(endpoint);
        }
      }

      sessionToken = "";
      await refreshCore(true);
    },
    async openWebUi() {
      const settings = ensureSettings();
      const endpoint = deps.endpointResolver.resolve(settings.configPath);
      const initialized = await deps.managementClient.getSetupInitialized(endpoint);
      const url = new URL(endpoint.baseUrl);

      if (initialized) {
        try {
          const launcherToken = await deps.managementClient.issueLauncherToken(endpoint);
          url.searchParams.set("token", launcherToken);
        } catch {
          url.search = "";
        }
      }

      await deps.externalOpener.openUri(url.toString());
      await publish({ ...snapshot, serviceDetail: "已在默认浏览器中打开管理界面。", lastError: "" });
    },
    async openReleasePage() {
      if (!snapshot.releaseCheck.releasePageUrl) {
        await publish({ ...snapshot, serviceDetail: "当前运行没有可打开的发布页。", lastError: "" });
        return;
      }
      await deps.externalOpener.openUri(snapshot.releaseCheck.releasePageUrl);
      await publish({ ...snapshot, serviceDetail: `已打开 ${snapshot.releaseCheck.releasePageUrl}`, lastError: "" });
    },
    async openLogsDirectory() {
      await deps.externalOpener.openDirectory(deps.processController.logDirectory);
    },
  };
}
