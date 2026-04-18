import { createReleaseUnavailable } from "../../shared/launcher-copy";
import {
  buildLocalDetail,
  resolveRecoverySummary,
  startingDetail,
} from "../../shared/launcher-presentation";
import type {
  EnvironmentCheckResult,
  EnvironmentInspection,
  LauncherProcessLifecycle,
  LauncherProcessOwnership,
  LauncherReadinessSnapshot,
  LauncherResolvedSettings,
  RecoveryCompatibilitySummary,
  LauncherSettings,
  LauncherSnapshot,
  ReleaseCheckSnapshot,
  ServerEndpoint,
  LauncherSystemStatusSnapshot,
  TaskSummary,
} from "../../shared/launcher-models";
import { sanitizeLauncherWebTargetPath } from "../../shared/launcher-validation";
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
  getReadiness(endpoint: ServerEndpoint): Promise<LauncherReadinessSnapshot>;
  getSetupInitialized(endpoint: ServerEndpoint): Promise<boolean>;
  issueLauncherToken(endpoint: ServerEndpoint): Promise<string>;
  admitLauncherToken(endpoint: ServerEndpoint, launcherToken: string): Promise<string>;
  getSystemStatus(endpoint: ServerEndpoint, sessionToken: string): Promise<LauncherSystemStatusSnapshot>;
  findInProgressTask(endpoint: ServerEndpoint, sessionToken: string, taskType: string): Promise<TaskSummary | null>;
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
  startupReadinessGraceMs?: number;
  pollIntervalMs?: number;
  shutdownTimeoutMs?: number;
  resetAdminTimeoutMs?: number;
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
  confirmExternalServiceStop?(): Promise<boolean>;
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

interface LocalSnapshotOverrides {
  processLifecycle?: LauncherProcessLifecycle;
  processOwnership?: LauncherProcessOwnership;
  lastLocalError?: string;
  statusHint?: string;
  localRecoverySummary?: RecoveryCompatibilitySummary | null;
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
      settings,
      resolvedSettings,
      endpoint,
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

function currentProcessLifecycle(processController: ServerProcessController, fallback: LauncherProcessLifecycle = "stopped") {
  if (fallback === "starting" || fallback === "stopping") {
    return fallback;
  }
  return processController.isRunning ? "running" : "stopped";
}

export function createLauncherCoordinator(deps: LauncherCoordinatorDependencies): LauncherCoordinator {
  const listeners = new Set<(snapshot: LauncherSnapshot) => void>();
  const options = {
    startupTimeoutMs: deps.options?.startupTimeoutMs ?? 300000,
    startupReadinessGraceMs: deps.options?.startupReadinessGraceMs ?? 10000,
    pollIntervalMs: deps.options?.pollIntervalMs ?? 500,
    shutdownTimeoutMs: deps.options?.shutdownTimeoutMs ?? 5000,
    resetAdminTimeoutMs: deps.options?.resetAdminTimeoutMs ?? 30000,
  };

  let currentSettings: LauncherSettings | null = null;
  let currentResolvedSettings: LauncherResolvedSettings = defaultResolvedSettings();
  let sessionToken = "";
  let releaseCheckInFlight: Promise<void> | null = null;
  let snapshot = defaultSnapshot(
    {
      installationRoot: "",
      closeBehavior: "ask_every_time",
    },
    defaultResolvedSettings(),
    { host: "127.0.0.1", port: 8080, baseUrl: "http://127.0.0.1:8080/" },
  );

  async function publish(next: LauncherSnapshot) {
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

  function buildSnapshot(
    endpoint: ServerEndpoint,
    inspection: EnvironmentInspection,
    server: Partial<LauncherSnapshot["server"]> = {},
    launcherOverrides: LocalSnapshotOverrides = {},
  ): LauncherSnapshot {
    const settings = ensureSettings();
    const resolvedSettings = ensureResolvedSettings();
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
        settings,
        resolvedSettings,
        endpoint,
        localRecoverySummary: launcherOverrides.localRecoverySummary ?? snapshot.launcher.localRecoverySummary ?? null,
      },
    };
  }

  async function tryLoadSystemStatus(endpoint: ServerEndpoint): Promise<LauncherSystemStatusSnapshot | null> {
    try {
      await ensureSessionToken(endpoint);
      return await deps.managementClient.getSystemStatus(endpoint, sessionToken);
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

  async function ensureSessionToken(endpoint: ServerEndpoint) {
    if (sessionToken) {
      return sessionToken;
    }
    const launcherToken = await deps.managementClient.issueLauncherToken(endpoint);
    sessionToken = await deps.managementClient.admitLauncherToken(endpoint, launcherToken);
    return sessionToken;
  }

  async function buildSnapshotFromReadiness(
    endpoint: ServerEndpoint,
    inspection: EnvironmentInspection,
    readiness: LauncherReadinessSnapshot,
    forceReauthentication: boolean,
  ) {
    if (forceReauthentication) {
      sessionToken = "";
    }

    const systemStatus =
      readiness.status === "ready" || readiness.status === "degraded"
        ? await tryLoadSystemStatus(endpoint)
        : null;
    const processOwnership = deps.processController.isRunning ? "launcher_managed" : "external";
    const localRecoverySummary =
      systemStatus?.recovery_summary
      ?? readiness.recovery_summary
      ?? await tryReadLocalRecoverySummary();

    return buildSnapshot(
      endpoint,
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
        buildSnapshot(
          endpoint,
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

    const healthy = await deps.managementClient.isHealthy(endpoint);
    if (!healthy) {
      await publish(
        buildSnapshot(
          endpoint,
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
      readiness = await deps.managementClient.getReadiness(endpoint);
    } catch (error) {
      const detail = error instanceof Error ? error.message : "无法读取 /readyz。";
      await publish(
        buildSnapshot(
          endpoint,
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

    await publish(await buildSnapshotFromReadiness(endpoint, inspection, readiness, forceReauthentication));
  }

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

      await delay(options.pollIntervalMs);
    }

    return null;
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
          buildSnapshot(
            endpoint,
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

      if (await deps.managementClient.isHealthy(endpoint)) {
        await refreshCore(true);
        return;
      }

      if ((await deps.isEndpointListening(endpoint)) && !deps.processController.isRunning) {
        await publish(
          buildSnapshot(
            endpoint,
            inspection,
            {},
            {
              processLifecycle: "stopped",
              processOwnership: "none",
              statusHint: "目标端口已被现有进程占用，启动器不会重复拉起服务。",
              lastLocalError: `端口 ${endpoint.port} 已被占用。`,
            },
          ),
        );
        return;
      }

      try {
        await deps.processController.start(resolvedSettings);
      } catch (error) {
        const detail = error instanceof Error ? error.message : "服务进程启动失败。";
        await publish(
          buildSnapshot(
            endpoint,
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
      await publish(
        buildSnapshot(
          endpoint,
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
      while (Date.now() - startedAt < options.startupTimeoutMs) {
        if (await deps.managementClient.isHealthy(endpoint)) {
          try {
            const readiness = await deps.managementClient.getReadiness(endpoint);
            if (readiness.status === "failed") {
              lastFailedReadiness = readiness;
              if (firstFailedReadinessAt === null) {
                firstFailedReadinessAt = Date.now();
              }
              if (Date.now() - firstFailedReadinessAt < options.startupReadinessGraceMs) {
                await delay(options.pollIntervalMs);
                continue;
              }
            } else {
              firstFailedReadinessAt = null;
              lastFailedReadiness = null;
            }
            await publish(await buildSnapshotFromReadiness(endpoint, inspection, readiness, true));
            return;
          } catch {
            // Keep polling until /readyz becomes readable. A transient healthz success alone
            // should not lock the launcher into a failed state during restart windows.
          }
        }
        if (!deps.processController.isRunning) {
          const lastError = deps.processController.getRecentStderr().at(-1) ?? "服务进程在通过健康检查前已退出。";
          if (await deps.managementClient.isHealthy(endpoint).catch(() => false)) {
            await refreshCore(true);
            return;
          }
          if (await deps.isEndpointListening(endpoint).catch(() => false)) {
            await publish(
              buildSnapshot(
                endpoint,
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
          await publish(
            buildSnapshot(
              endpoint,
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
        await delay(options.pollIntervalMs);
      }

      if (lastFailedReadiness) {
        await publish(await buildSnapshotFromReadiness(endpoint, inspection, lastFailedReadiness, true));
        return;
      }

      await deps.processController.forceKill();
      await publish(
        buildSnapshot(
          endpoint,
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
      const settings = ensureSettings();
      currentResolvedSettings = await resolveLauncherSettings(settings, process.platform);
      const resolvedSettings = ensureResolvedSettings();
      const endpoint = await deps.endpointResolver.resolve(resolvedSettings.configPath);
      const inspection = await deps.inspectEnvironment(resolvedSettings);
      const healthy = await deps.managementClient.isHealthy(endpoint).catch(() => false);
      const isManagedProcess = deps.processController.isRunning;
      const processOwnership =
        isManagedProcess ? "launcher_managed" satisfies LauncherProcessOwnership
          : healthy ? "external" satisfies LauncherProcessOwnership
            : "none" satisfies LauncherProcessOwnership;

      if (healthy && processOwnership === "external") {
        const confirmed = await deps.confirmExternalServiceStop?.() ?? false;
        if (!confirmed) {
          await refreshCore(false);
          return;
        }

        await publish(
          buildSnapshot(
            endpoint,
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
          if (!await deps.managementClient.getSetupInitialized(endpoint)) {
            throw new Error("检测到现有服务仍处于初始化阶段，无法由启动器停止。");
          }

          sessionToken = "";
          const launcherToken = await deps.managementClient.issueLauncherToken(endpoint);
          sessionToken = await deps.managementClient.admitLauncherToken(endpoint, launcherToken);
          await deps.managementClient.shutdown(endpoint, sessionToken);
          sessionToken = "";
          await refreshCore(true);
        } catch (error) {
          sessionToken = "";
          await refreshCore(true);
          await publish({
            ...snapshot,
            launcher: {
              ...snapshot.launcher,
              lastLocalError: error instanceof Error ? error.message : "无法停止检测到的现有服务。",
              statusHint: "无法停止检测到的现有服务。",
            },
          });
        }
        return;
      }

      await publish(
        buildSnapshot(
          endpoint,
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

      if (deps.processController.isRunning || (await deps.managementClient.isHealthy(endpoint).catch(() => false))) {
        await publish(
          buildSnapshot(
            endpoint,
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
          await deps.tryStopEndpointProcess(endpoint);
        }
        await ensureManagedProcessStopped();
      }

      await deps.resetAdminRunner.run(resolvedSettings);
      sessionToken = "";

      try {
        await deps.processController.start(resolvedSettings);
      } catch (error) {
        const detail = error instanceof Error ? error.message : "服务进程启动失败。";
        await publish(
          buildSnapshot(
            endpoint,
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

      const readiness = await waitForReadinessStatus(endpoint, "setup_required", options.resetAdminTimeoutMs);
      if (!readiness) {
        const detail = deps.processController.getRecentStderr().at(-1) ?? "服务未在预期时间内恢复到初始化状态。";
        await publish(
          buildSnapshot(
            endpoint,
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

      await publish(await buildSnapshotFromReadiness(endpoint, inspection, readiness, true));

      const url = new URL(endpoint.baseUrl);
      await deps.externalOpener.openUri(url.toString());
    },
    async openWebUi(targetPath = "") {
      const settings = ensureSettings();
      currentResolvedSettings = await resolveLauncherSettings(settings, process.platform);
      const endpoint = await deps.endpointResolver.resolve(ensureResolvedSettings().configPath);
      const normalizedTarget = sanitizeLauncherWebTargetPath(targetPath);
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
            return;
          }
          url.search = "";
        }
      }

      await deps.externalOpener.openUri(url.toString());
    },
    async createRecoveryRecheck() {
      if (!resolveRecoverySummary(snapshot)) {
        return;
      }
      const settings = ensureSettings();
      currentResolvedSettings = await resolveLauncherSettings(settings, process.platform);
      const endpoint = await deps.endpointResolver.resolve(ensureResolvedSettings().configPath);
      const token = await ensureSessionToken(endpoint);
      const existingTask = await deps.managementClient.findInProgressTask(endpoint, token, "recovery.recheck");
      const taskId = existingTask?.task_id
        ?? (await deps.managementClient.createRecoveryRecheck(endpoint, token)).task_id;
      await coordinator.openWebUi(`/tasks?task_id=${encodeURIComponent(taskId)}`);
    },
    async createRuntimeBootstrap(resources) {
      const settings = ensureSettings();
      currentResolvedSettings = await resolveLauncherSettings(settings, process.platform);
      const endpoint = await deps.endpointResolver.resolve(ensureResolvedSettings().configPath);
      const token = await ensureSessionToken(endpoint);
      const existingTask = await deps.managementClient.findInProgressTask(endpoint, token, "runtime.bootstrap");
      const taskId = existingTask?.task_id
        ?? (await deps.managementClient.createRuntimeBootstrap(endpoint, token, resources)).task_id;
      await coordinator.openWebUi(`/tasks?task_id=${encodeURIComponent(taskId)}`);
    },
    async openReleasePage() {
      if (!snapshot.launcher.releaseCheck.releasePageUrl) {
        await publish({
          ...snapshot,
          launcher: {
            ...snapshot.launcher,
            statusHint: "当前运行没有可打开的发布页。",
            lastLocalError: "",
          },
        });
        return;
      }
      await deps.externalOpener.openUri(snapshot.launcher.releaseCheck.releasePageUrl);
    },
    async openLogsDirectory() {
      await deps.externalOpener.openDirectory(deps.processController.logDirectory);
    },
  };

  return coordinator;
}
