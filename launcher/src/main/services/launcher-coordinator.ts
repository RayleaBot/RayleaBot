import { createReleaseUnavailable } from "../../shared/launcher-copy";
import type {
  EnvironmentCheckResult,
  EnvironmentInspection,
  LauncherResolvedSettings,
  RecoveryCompatibilitySummary,
  LauncherSettings,
  LauncherSnapshot,
  ReleaseCheckSnapshot,
  ServerEndpoint,
  LauncherServiceState,
} from "../../shared/launcher-models";
import { resolveLauncherSettings } from "./settings-store";

export type { EnvironmentCheckResult, EnvironmentInspection, LauncherSettings, ServerEndpoint };

export interface LauncherSettingsStore {
  load(): Promise<LauncherSettings>;
  save(settings: LauncherSettings): Promise<void>;
}

export interface ServerEndpointResolver {
  resolve(configPath: string): ServerEndpoint | Promise<ServerEndpoint>;
}

export interface LauncherManagementClient {
  isHealthy(endpoint: ServerEndpoint): Promise<boolean>;
  getSetupInitialized(endpoint: ServerEndpoint): Promise<boolean>;
  issueLauncherToken(endpoint: ServerEndpoint): Promise<string>;
  admitLauncherToken(endpoint: ServerEndpoint, launcherToken: string): Promise<string>;
  getSystemStatus(endpoint: ServerEndpoint, sessionToken: string): Promise<{ recovery_summary?: RecoveryCompatibilitySummary | null }>;
  createRecoveryRecheck(endpoint: ServerEndpoint, sessionToken: string): Promise<{ task_id: string }>;
  createRuntimeBootstrap(endpoint: ServerEndpoint, sessionToken: string, resources?: string[]): Promise<{ task_id: string }>;
  shutdown(endpoint: ServerEndpoint, sessionToken: string): Promise<void>;
}

export interface ServerProcessController {
  isRunning: boolean;
  processId: number | null;
  logDirectory: string;
  start(settings: LauncherResolvedSettings): Promise<void>;
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

export interface RecoverySummaryReader {
  read(logDirectory: string): Promise<RecoveryCompatibilitySummary | null>;
}

export interface LauncherResetAdminRunner {
  run(settings: LauncherResolvedSettings): Promise<void>;
}

interface LauncherCoordinatorOptions {
  startupTimeoutMs?: number;
  pollIntervalMs?: number;
  shutdownTimeoutMs?: number;
}

interface LauncherCoordinatorDependencies {
  settingsStore: LauncherSettingsStore;
  endpointResolver: ServerEndpointResolver;
  inspectEnvironment(settings: LauncherResolvedSettings): Promise<EnvironmentInspection>;
  managementClient: LauncherManagementClient;
  processController: ServerProcessController;
  isEndpointListening(endpoint: ServerEndpoint): Promise<boolean>;
  tryStopEndpointProcess(endpoint: ServerEndpoint): Promise<boolean>;
  externalOpener: ExternalOpener;
  releaseFeedClient?: ReleaseFeedClient;
  resetAdminRunner?: LauncherResetAdminRunner;
  recoverySummaryReader?: RecoverySummaryReader;
  options?: LauncherCoordinatorOptions;
}

export interface LauncherCoordinator {
  snapshot: LauncherSnapshot;
  initialize(): Promise<void>;
  refresh(): Promise<void>;
  retry(): Promise<void>;
  start(): Promise<void>;
  stop(): Promise<void>;
  resetAdmin(): Promise<void>;
  openWebUi(targetPath?: string): Promise<void>;
  createRecoveryRecheck(): Promise<void>;
  createRuntimeBootstrap(resources?: string[]): Promise<void>;
  openReleasePage(): Promise<void>;
  openLogsDirectory(): Promise<void>;
  saveSettings(settings: LauncherSettings): Promise<void>;
  subscribe(listener: (snapshot: LauncherSnapshot) => void): () => void;
}

function delay(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

function defaultResolvedSettings(): LauncherResolvedSettings {
  return {
    installationRoot: "",
    serverExecutablePath: "",
    configPath: "",
    workdir: "",
  };
}

function defaultSnapshot(settings: LauncherSettings, resolvedSettings: LauncherResolvedSettings, endpoint: ServerEndpoint): LauncherSnapshot {
  return {
    settings,
    resolvedSettings,
    endpoint,
    environmentChecks: [],
    recentStderr: [],
    processId: null,
    serviceState: "stopped",
    shutdownRequested: false,
    serviceDetail: "服务尚未启动。",
    lastError: "",
    releaseCheck: createReleaseUnavailable(),
    recoverySummary: null,
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
    shutdownTimeoutMs: deps.options?.shutdownTimeoutMs ?? 5000,
  };

  let currentSettings: LauncherSettings | null = null;
  let currentResolvedSettings: LauncherResolvedSettings = defaultResolvedSettings();
  let sessionToken = "";
  let snapshot = defaultSnapshot(
    {
      installationRoot: "",
      closeBehavior: "ask_every_time",
    },
    defaultResolvedSettings(),
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

  function ensureResolvedSettings() {
    return currentResolvedSettings;
  }

  async function buildSnapshot(
    endpoint: ServerEndpoint,
    inspection: EnvironmentInspection,
    serviceState: LauncherServiceState,
    serviceDetail: string,
    lastError = "",
  ) {
    const settings = ensureSettings();
    const resolvedSettings = ensureResolvedSettings();
    return {
      settings,
      resolvedSettings,
      endpoint,
      environmentChecks: inspection.checks,
      recentStderr: deps.processController.getRecentStderr(),
      processId: deps.processController.isRunning ? deps.processController.processId : null,
      serviceState,
      shutdownRequested: serviceState === "shutting_down",
      serviceDetail,
      lastError,
      releaseCheck: snapshot.releaseCheck,
      recoverySummary: snapshot.recoverySummary ?? null,
    } satisfies LauncherSnapshot;
  }

  async function tryLoadRecoverySummary(endpoint: ServerEndpoint): Promise<RecoveryCompatibilitySummary | null> {
    try {
      await ensureSessionToken(endpoint);
      const status = await deps.managementClient.getSystemStatus(endpoint, sessionToken);
      return status.recovery_summary ?? null;
    } catch {
      if (!deps.recoverySummaryReader) {
        return null;
      }
      try {
        return await deps.recoverySummaryReader.read(deps.processController.logDirectory);
      } catch {
        return null;
      }
    }
  }

  async function ensureSessionToken(endpoint: ServerEndpoint) {
    if (sessionToken) {
      return sessionToken;
    }
    const launcherToken = await deps.managementClient.issueLauncherToken(endpoint);
    sessionToken = await deps.managementClient.admitLauncherToken(endpoint, launcherToken);
    return sessionToken;
  }

  async function refreshCore(forceReauthentication: boolean) {
    if (forceReauthentication) {
      sessionToken = "";
    }
    const settings = ensureSettings();
    currentResolvedSettings = await resolveLauncherSettings(settings, process.platform);
    const resolvedSettings = ensureResolvedSettings();
    const endpoint = await deps.endpointResolver.resolve(resolvedSettings.configPath);
    const inspection = await deps.inspectEnvironment(resolvedSettings);

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
      const next = await buildSnapshot(
        endpoint,
        inspection,
        deps.processController.isRunning ? "failed" : "stopped",
        deps.processController.isRunning ? "子进程仍在运行，但健康检查失败。" : "服务尚未启动。",
        deps.processController.isRunning ? "健康检查失败。" : "",
      );
      next.recoverySummary = await tryLoadRecoverySummary(endpoint);
      await publish(next);
      return;
    }

    let setupInitialized = true;
    try {
      setupInitialized = await deps.managementClient.getSetupInitialized(endpoint);
    } catch {
      // isHealthy already passed — a transient getSetupInitialized failure
      // should not flash a misleading setup_required state to the user.
      setupInitialized = true;
    }

    if (!setupInitialized) {
      await publish(
        await buildSnapshot(
          endpoint,
          inspection,
          "setup_required",
          "服务正在运行，需要完成管理员初始化。",
        ),
      );
      return;
    }

    const next = await buildSnapshot(
      endpoint,
      inspection,
      deps.processController.isRunning ? "ready" : "external_service",
      deps.processController.isRunning ? "服务正在运行。" : "端口上已有服务正在运行。可以直接打开管理界面，或先停止它再由启动器重新启动。",
    );
    next.recoverySummary = await tryLoadRecoverySummary(endpoint);
    if (next.recoverySummary && next.recoverySummary.status !== "compatible") {
      next.serviceState = "degraded";
      next.serviceDetail = "恢复兼容性检查需要关注，请先处理摘要中的问题。";
    }
    await publish(next);
  }

  async function ensureManagedProcessStopped() {
    if (!deps.processController.isRunning) {
      return;
    }

    const stopDeadline = Date.now() + options.shutdownTimeoutMs;
    while (deps.processController.isRunning && Date.now() < stopDeadline) {
      await delay(options.pollIntervalMs);
    }

    if (deps.processController.isRunning) {
      await deps.processController.forceKill();
    }
  }

  const coordinator: LauncherCoordinator = {
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
      currentResolvedSettings = await resolveLauncherSettings(currentSettings, process.platform);
      const endpoint = await deps.endpointResolver.resolve(currentResolvedSettings.configPath);
      snapshot = defaultSnapshot(currentSettings, currentResolvedSettings, endpoint);
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
      currentResolvedSettings = await resolveLauncherSettings(settings, process.platform);
      sessionToken = "";
      await deps.settingsStore.save(settings);
      await refreshCore(true);
    },
    async start() {
      const settings = ensureSettings();
      currentResolvedSettings = await resolveLauncherSettings(settings, process.platform);
      const resolvedSettings = ensureResolvedSettings();
      const endpoint = await deps.endpointResolver.resolve(resolvedSettings.configPath);
      const inspection = await deps.inspectEnvironment(resolvedSettings);

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

      try {
        await deps.processController.start(resolvedSettings);
      } catch (error) {
        const detail = error instanceof Error ? error.message : "服务进程启动失败。";
        await publish(await buildSnapshot(endpoint, inspection, "failed", "无法启动服务进程。", detail));
        return;
      }
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
        if (!deps.processController.isRunning) {
          const lastError = deps.processController.getRecentStderr().at(-1) ?? "服务进程在通过健康检查前已退出。";
          await publish(await buildSnapshot(endpoint, inspection, "failed", "服务进程在启动阶段提前退出。", lastError));
          return;
        }
        await delay(options.pollIntervalMs);
      }

      await deps.processController.forceKill();
      await publish(await buildSnapshot(endpoint, inspection, "failed", "启动超时内未通过健康检查。", "服务启动已超时。"));
    },
    async stop() {
      const settings = ensureSettings();
      currentResolvedSettings = await resolveLauncherSettings(settings, process.platform);
      const resolvedSettings = ensureResolvedSettings();
      const endpoint = await deps.endpointResolver.resolve(resolvedSettings.configPath);
      const inspection = await deps.inspectEnvironment(resolvedSettings);

      await publish(await buildSnapshot(endpoint, inspection, "shutting_down", deps.processController.isRunning ? "正在停止服务。" : "正在停止现有服务。"));

      if (await deps.managementClient.isHealthy(endpoint)) {
        try {
          if (await deps.managementClient.getSetupInitialized(endpoint)) {
            if (!sessionToken) {
              const launcherToken = await deps.managementClient.issueLauncherToken(endpoint);
              sessionToken = await deps.managementClient.admitLauncherToken(endpoint, launcherToken);
            }
            await deps.managementClient.shutdown(endpoint, sessionToken);
          } else if (deps.processController.isRunning) {
            await deps.processController.forceKill();
          } else {
            await deps.tryStopEndpointProcess(endpoint);
          }
        } catch {
          if (deps.processController.isRunning) {
            await deps.processController.forceKill();
          } else {
            await deps.tryStopEndpointProcess(endpoint);
          }
        }
      }

      await ensureManagedProcessStopped();
      sessionToken = "";
      await refreshCore(true);
    },
    async resetAdmin() {
      if (!deps.resetAdminRunner) {
        throw new Error("管理员重置功能不可用。");
      }
      const settings = ensureSettings();
      currentResolvedSettings = await resolveLauncherSettings(settings, process.platform);
      const resolvedSettings = ensureResolvedSettings();
      const endpoint = await deps.endpointResolver.resolve(resolvedSettings.configPath);
      const inspection = await deps.inspectEnvironment(resolvedSettings);

      // Stop the service first if running.
      if (deps.processController.isRunning || (await deps.managementClient.isHealthy(endpoint).catch(() => false))) {
        await publish(await buildSnapshot(endpoint, inspection, "shutting_down", "正在停止服务以执行管理员重置。"));
        if (deps.processController.isRunning) {
          await deps.processController.forceKill();
        } else {
          await deps.tryStopEndpointProcess(endpoint);
        }
        await ensureManagedProcessStopped();
      }

      // Run the reset-admin CLI.
      await deps.resetAdminRunner.run(resolvedSettings);
      sessionToken = "";

      // Restart the service.
      try {
        await deps.processController.start(resolvedSettings);
      } catch (error) {
        const detail = error instanceof Error ? error.message : "服务进程启动失败。";
        await publish(
          await buildSnapshot(endpoint, inspection, "failed", "管理员凭据已重置，但服务重启失败。", detail),
        );
        return;
      }

      // After reset-admin, the server enters setup_required on next start.
      // Open the setup entry directly without waiting for full health recovery.
      await publish(
        await buildSnapshot(endpoint, inspection, "setup_required", "管理员凭据已重置，请在浏览器中完成初始化。"),
      );

      const url = new URL(endpoint.baseUrl);
      await deps.externalOpener.openUri(url.toString());
    },
    async openWebUi(targetPath = "") {
      const settings = ensureSettings();
      currentResolvedSettings = await resolveLauncherSettings(settings, process.platform);
      const endpoint = await deps.endpointResolver.resolve(ensureResolvedSettings().configPath);
      const normalizedTarget = targetPath.startsWith("/") ? targetPath.slice(1) : targetPath;
      const url = normalizedTarget ? new URL(normalizedTarget, endpoint.baseUrl) : new URL(endpoint.baseUrl);
      let initialized = false;

      try {
        initialized = await deps.managementClient.getSetupInitialized(endpoint);
      } catch {
        initialized = false;
      }

      if (initialized) {
        try {
          const launcherToken = await deps.managementClient.issueLauncherToken(endpoint);
          url.searchParams.set("token", launcherToken);
        } catch {
          if (normalizedTarget) {
            const fallbackURL = new URL(normalizedTarget, endpoint.baseUrl);
            await deps.externalOpener.openUri(fallbackURL.toString());
            await publish({ ...snapshot, serviceDetail: "已在默认浏览器中打开管理界面。", lastError: "" });
            return;
          }
          url.search = "";
        }
      }

      await deps.externalOpener.openUri(url.toString());
      await publish({ ...snapshot, serviceDetail: "已在默认浏览器中打开管理界面。", lastError: "" });
    },
    async createRecoveryRecheck() {
      const settings = ensureSettings();
      currentResolvedSettings = await resolveLauncherSettings(settings, process.platform);
      const endpoint = await deps.endpointResolver.resolve(ensureResolvedSettings().configPath);
      const accepted = await deps.managementClient.createRecoveryRecheck(endpoint, await ensureSessionToken(endpoint));
      await coordinator.openWebUi(`/tasks?task_id=${encodeURIComponent(accepted.task_id)}`);
    },
    async createRuntimeBootstrap(resources) {
      const settings = ensureSettings();
      currentResolvedSettings = await resolveLauncherSettings(settings, process.platform);
      const endpoint = await deps.endpointResolver.resolve(ensureResolvedSettings().configPath);
      const accepted = await deps.managementClient.createRuntimeBootstrap(endpoint, await ensureSessionToken(endpoint), resources);
      await coordinator.openWebUi(`/tasks?task_id=${encodeURIComponent(accepted.task_id)}`);
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

  return coordinator;
}
