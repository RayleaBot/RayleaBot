import { createReleaseUnavailable } from "../../shared/launcher-copy";
import type {
  EnvironmentCheckResult,
  EnvironmentInspection,
  LauncherReadinessSnapshot,
  LauncherResolvedSettings,
  RecoveryCompatibilitySummary,
  LauncherSettings,
  LauncherSnapshot,
  ReleaseCheckSnapshot,
  ServerEndpoint,
  LauncherServiceState,
  LauncherServiceOwnership,
  LauncherSystemStatusSnapshot,
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
    serviceOwnership: "none",
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

function releaseChecksEqual(left: ReleaseCheckSnapshot, right: ReleaseCheckSnapshot) {
  return left.status === right.status
    && left.currentVersion === right.currentVersion
    && left.latestVersion === right.latestVersion
    && left.summary === right.summary
    && left.detail === right.detail
    && left.releasePageUrl === right.releasePageUrl
    && left.updateAvailable === right.updateAvailable;
}

function detailFromReadiness(readiness: LauncherReadinessSnapshot, fallback: string) {
  return readiness.reason?.trim() || fallback;
}

function startingDetail(canBootstrapUserConfig: boolean) {
  return canBootstrapUserConfig
    ? "已基于 default.yaml 生成首份用户配置，正在准备运行环境并等待服务就绪。"
    : "正在准备运行环境并等待服务就绪。";
}

function ownershipForHealthyService(isManagedProcess: boolean): LauncherServiceOwnership {
  return isManagedProcess ? "launcher_managed" : "external";
}

function stateFromReadiness(
  isManagedProcess: boolean,
  readiness: LauncherReadinessSnapshot,
  systemStatus: LauncherSystemStatusSnapshot | null,
): {
  serviceState: LauncherServiceState;
  serviceOwnership: LauncherServiceOwnership;
  serviceDetail: string;
  lastError: string;
} {
  const serviceOwnership = ownershipForHealthyService(isManagedProcess);

  if (systemStatus?.status === "shutting_down") {
    return {
      serviceState: "stopping" satisfies LauncherServiceState,
      serviceOwnership,
      serviceDetail: detailFromReadiness(readiness, "服务正在停止。"),
      lastError: "",
    };
  }

  switch (readiness.status) {
    case "ready":
      return {
        serviceState: "running" satisfies LauncherServiceState,
        serviceOwnership,
        serviceDetail: detailFromReadiness(
          readiness,
          isManagedProcess
            ? "服务正在运行。"
            : "检测到现有服务。可以直接打开管理界面，或确认后停止它。",
        ),
        lastError: "",
      };
    case "degraded":
      return {
        serviceState: "degraded" satisfies LauncherServiceState,
        serviceOwnership,
        serviceDetail: detailFromReadiness(
          readiness,
          isManagedProcess
            ? "管理面可用，但当前仍有运行条件未满足。"
            : "检测到现有服务，管理面可用，但当前仍有运行条件未满足。",
        ),
        lastError: "",
      };
    case "setup_required":
      return {
        serviceState: "setup_required" satisfies LauncherServiceState,
        serviceOwnership,
        serviceDetail: detailFromReadiness(readiness, "服务正在运行，需要完成管理员初始化。"),
        lastError: "",
      };
    case "failed":
    default:
      return {
        serviceState: "failed" satisfies LauncherServiceState,
        serviceOwnership,
        serviceDetail: detailFromReadiness(readiness, "服务已运行，但尚未达到就绪状态。"),
        lastError: readiness.reason?.trim() || "服务尚未就绪。",
      };
  }
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
    releaseCheckInFlight = withReleaseCheck(deps.releaseFeedClient, snapshot.releaseCheck)
      .then((releaseCheck) => {
        if (releaseChecksEqual(snapshot.releaseCheck, releaseCheck)) {
          return;
        }
        snapshot = {
          ...snapshot,
          releaseCheck,
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

  async function buildSnapshot(
    endpoint: ServerEndpoint,
    inspection: EnvironmentInspection,
    serviceState: LauncherServiceState,
    serviceOwnership: LauncherServiceOwnership,
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
      serviceOwnership,
      shutdownRequested: serviceState === "stopping",
      serviceDetail,
      lastError,
      releaseCheck: snapshot.releaseCheck,
      recoverySummary: snapshot.recoverySummary ?? null,
    } satisfies LauncherSnapshot;
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
    const contractState = stateFromReadiness(deps.processController.isRunning, readiness, systemStatus);
    const next = await buildSnapshot(
      endpoint,
      inspection,
      contractState.serviceState,
      contractState.serviceOwnership,
      contractState.serviceDetail,
      contractState.lastError,
    );
    next.recoverySummary =
      systemStatus?.recovery_summary
      ?? readiness.recovery_summary
      ?? await tryReadLocalRecoverySummary();
    return next;
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
          "none",
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
        deps.processController.isRunning ? "launcher_managed" : "none",
        deps.processController.isRunning ? "子进程仍在运行，但健康检查失败。" : "服务尚未启动。",
        deps.processController.isRunning ? "健康检查失败。" : "",
      );
      next.recoverySummary = await tryReadLocalRecoverySummary();
      await publish(next);
      return;
    }

    let readiness: LauncherReadinessSnapshot;
    try {
      readiness = await deps.managementClient.getReadiness(endpoint);
    } catch (error) {
      const detail = error instanceof Error ? error.message : "无法读取 /readyz。";
      const next = await buildSnapshot(
        endpoint,
        inspection,
        "failed",
        deps.processController.isRunning ? "launcher_managed" : "external",
        "服务存活，但无法读取正式就绪状态。",
        detail,
      );
      next.recoverySummary = await tryReadLocalRecoverySummary();
      await publish(next);
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
          await buildSnapshot(endpoint, inspection, "stopped", "none", buildLocalDetail("启动器预检发现阻塞项。", inspection.checks)),
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
            "none",
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
        await publish(await buildSnapshot(endpoint, inspection, "failed", "none", "无法启动服务进程。", detail));
        return;
      }
      await publish(
        await buildSnapshot(
          endpoint,
          inspection,
          "starting",
          "launcher_managed",
          startingDetail(inspection.canBootstrapUserConfig),
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
              await buildSnapshot(
                endpoint,
                inspection,
                "failed",
                "none",
                "目标端口已被现有进程占用，RayleaBot 无法监听当前地址。",
                lastError,
              ),
            );
            return;
          }
          await publish(await buildSnapshot(endpoint, inspection, "failed", "launcher_managed", "服务进程在启动阶段提前退出。", lastError));
          return;
        }
        await delay(options.pollIntervalMs);
      }

      if (lastFailedReadiness) {
        await publish(await buildSnapshotFromReadiness(endpoint, inspection, lastFailedReadiness, true));
        return;
      }

      await deps.processController.forceKill();
      await publish(await buildSnapshot(endpoint, inspection, "failed", "launcher_managed", "启动超时内未通过健康检查。", "服务启动已超时。"));
    },
    async stop() {
      const settings = ensureSettings();
      currentResolvedSettings = await resolveLauncherSettings(settings, process.platform);
      const resolvedSettings = ensureResolvedSettings();
      const endpoint = await deps.endpointResolver.resolve(resolvedSettings.configPath);
      const inspection = await deps.inspectEnvironment(resolvedSettings);
      const healthy = await deps.managementClient.isHealthy(endpoint).catch(() => false);
      const isManagedProcess = deps.processController.isRunning;
      const serviceOwnership =
        isManagedProcess ? "launcher_managed" satisfies LauncherServiceOwnership
          : healthy ? "external" satisfies LauncherServiceOwnership
            : "none" satisfies LauncherServiceOwnership;

      if (healthy && serviceOwnership === "external") {
        const confirmed = await deps.confirmExternalServiceStop?.() ?? false;
        if (!confirmed) {
          await refreshCore(false);
          return;
        }

        await publish(await buildSnapshot(endpoint, inspection, "stopping", "external", "正在停止现有服务。"));

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
            lastError: error instanceof Error ? error.message : "无法停止检测到的现有服务。",
            serviceDetail: "无法停止检测到的现有服务。",
          });
        }
        return;
      }

      await publish(await buildSnapshot(endpoint, inspection, "stopping", serviceOwnership, isManagedProcess ? "正在停止服务。" : "正在停止现有服务。"));

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

      // Stop the service first if running.
      if (deps.processController.isRunning || (await deps.managementClient.isHealthy(endpoint).catch(() => false))) {
        await publish(await buildSnapshot(endpoint, inspection, "stopping", "launcher_managed", "正在停止服务以执行管理员重置。"));
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
          await buildSnapshot(endpoint, inspection, "failed", "launcher_managed", "管理员凭据已重置，但服务重启失败。", detail),
        );
        return;
      }

      const readiness = await waitForReadinessStatus(endpoint, "setup_required", options.resetAdminTimeoutMs);
      if (!readiness) {
        const detail = deps.processController.getRecentStderr().at(-1) ?? "服务未在预期时间内恢复到初始化状态。";
        await publish(
          await buildSnapshot(endpoint, inspection, "failed", "launcher_managed", "管理员凭据已重置，但服务未进入初始化状态。", detail),
        );
        return;
      }

      await publish(
        await buildSnapshot(
          endpoint,
          inspection,
          "setup_required",
          "launcher_managed",
          detailFromReadiness(readiness, "管理员凭据已重置，请在浏览器中完成初始化。"),
        ),
      );

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
      if (!snapshot.recoverySummary) {
        return;
      }
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
