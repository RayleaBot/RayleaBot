import { afterEach, describe, expect, test, vi } from "vitest";
import type { LauncherReadinessSnapshot, LauncherSnapshot, ReleaseCheckSnapshot } from "@shared/launcher-models";
import type { RuntimePrepareSnapshot } from "@shared/launcher-models";
import { deriveLauncherPresentation, resolveRecoverySummary } from "@shared/launcher-presentation";
import {
  createLauncherCoordinator,
  type EnvironmentCheckResult,
  type EnvironmentInspection,
  type ExternalOpener,
  type LauncherManagementClient,
  type RecoverySummaryReader,
  type LauncherSettings,
  type LauncherSettingsStore,
  type ReleaseFeedClient,
  type RecoveryCompatibilitySummary,
  type ServerEndpoint,
  type ServerEndpointResolver,
  type ServerProcessController,
  type LauncherResetAdminRunner,
} from "@main/services/launcher-coordinator";

class FakeSettingsStore implements LauncherSettingsStore {
  settings: LauncherSettings = {
    installationRoot: "C:\\RayleaBot",
    closeBehavior: "ask_every_time",
  };

  async load() {
    return this.settings;
  }

  async save(settings: LauncherSettings) {
    this.settings = settings;
  }
}

class FakeEndpointResolver implements ServerEndpointResolver {
  resolve(): ServerEndpoint {
    return { host: "127.0.0.1", port: 8080, baseUrl: "http://127.0.0.1:8080/" };
  }
}

class FakeManagementClient implements LauncherManagementClient {
  health = true;
  setupInitialized = true;
  systemStatusCalls = 0;
  readiness: LauncherReadinessSnapshot = {
    status: "ready",
  };
  systemStatus = {
    status: "running",
    recovery_summary: null as RecoveryCompatibilitySummary | null,
  };
  recoverySummary: RecoveryCompatibilitySummary | null = null;

  async isHealthy() {
    return this.health;
  }

  async getSetupInitialized() {
    return this.setupInitialized;
  }

  async getReadiness() {
    return this.readiness;
  }

  async getLauncherStatus() {
    this.systemStatusCalls += 1;
    return {
      ...this.systemStatus,
      recovery_summary: this.recoverySummary ?? this.systemStatus.recovery_summary,
    };
  }

  async shutdownFromLauncher() {}
}

class FakeRecoverySummaryReader implements RecoverySummaryReader {
  summary: RecoveryCompatibilitySummary | null = null;

  async read() {
    return this.summary;
  }
}

class FakeProcessController implements ServerProcessController {
  isRunning = false;
  processId: number | null = 4242;
  startCalls = 0;
  forceKillCalls = 0;
  recentStderr = ["stderr line"];
  runtimePrepare: RuntimePrepareSnapshot | null = null;
  logDirectory = "C:\\RayleaBot\\logs";

  async start() {
    this.startCalls += 1;
    this.isRunning = true;
  }

  async forceKill() {
    this.forceKillCalls += 1;
    this.isRunning = false;
  }

  getRecentStderr() {
    return this.recentStderr;
  }

  getRuntimePrepareSnapshot() {
    return this.runtimePrepare;
  }

  clearRuntimePrepareSnapshot() {
    this.runtimePrepare = null;
  }
}

class FakeExternalOpener implements ExternalOpener {
  openedUris: string[] = [];
  openedDirectories: string[] = [];

  async openUri(uri: string) {
    this.openedUris.push(uri);
  }

  async openDirectory(directoryPath: string) {
    this.openedDirectories.push(directoryPath);
  }
}

class FakeReleaseFeedClient implements ReleaseFeedClient {
  async getSnapshot() {
    return releaseSnapshot({
      status: "up_to_date",
      currentVersion: "0.1.0",
      latestVersion: "0.1.0",
      summary: "当前版本 0.1.0 已是最新。",
      detail: "",
      releasePageUrl: "https://example.invalid/releases/v0.1.0",
      updateAvailable: false,
      canCheck: true,
    });
  }

  async downloadUpdate() {
    return this.getSnapshot();
  }

  async installDownloadedUpdate() {
    return this.getSnapshot();
  }
}

function releaseSnapshot(overrides: Partial<ReleaseCheckSnapshot> = {}): ReleaseCheckSnapshot {
  return {
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
    ...overrides,
  };
}

class FakeResetAdminRunner implements LauncherResetAdminRunner {
  calls = 0;

  async run() {
    this.calls += 1;
  }
}

function okInspection(overrides: Partial<EnvironmentInspection> = {}): EnvironmentInspection {
  const checks: EnvironmentCheckResult[] = [
    {
      scope: "preflight",
      code: "server.executable",
      title: "服务端可执行文件",
      severity: "ok",
      summary: "已找到可执行文件。",
      detail: "ok",
      remediation: "",
    },
    {
      scope: "preflight",
      code: "config.file",
      title: "用户配置",
      severity: "ok",
      summary: "配置文件可读。",
      detail: "ok",
      remediation: "",
    },
  ];

  return {
    checks,
    preflightChecks: checks,
    advisoryChecks: [],
    hasBlockingIssues: false,
    canBootstrapUserConfig: false,
    ...overrides,
  };
}

function presentationState(snapshot: LauncherSnapshot) {
  return deriveLauncherPresentation(snapshot);
}

afterEach(() => {
  vi.useRealTimers();
});

async function waitForPresentationState(
  coordinator: { readonly snapshot: LauncherSnapshot },
  expectedState: ReturnType<typeof presentationState>["state"],
  timeoutMs = 500,
) {
  const deadline = Date.now() + timeoutMs;
  let latest = presentationState(coordinator.snapshot);
  while (latest.state !== expectedState && Date.now() < deadline) {
    await new Promise((resolve) => setTimeout(resolve, 5));
    latest = presentationState(coordinator.snapshot);
  }
  expect(latest.state).toBe(expectedState);
  return latest;
}

async function waitForCondition(assertion: () => void, timeoutMs = 500) {
  const deadline = Date.now() + timeoutMs;
  let lastError: unknown;
  while (Date.now() < deadline) {
    try {
      assertion();
      return;
    } catch (error) {
      lastError = error;
      await new Promise((resolve) => setTimeout(resolve, 5));
    }
  }
  if (lastError) {
    throw lastError;
  }
}

describe("launcher coordinator", () => {
  test("initialize reports externally managed running service with formal state names", async () => {
    const settingsStore = new FakeSettingsStore();
    const endpointResolver = new FakeEndpointResolver();
    const managementClient = new FakeManagementClient();
    const processController = new FakeProcessController();
    const externalOpener = new FakeExternalOpener();
    const inspect = vi.fn(async () => okInspection());

    const coordinator = createLauncherCoordinator({
      settingsStore,
      endpointResolver,
      inspectEnvironment: inspect,
      managementClient,
      processController,
      isEndpointListening: vi.fn(async () => false),
      tryStopEndpointProcess: vi.fn(async () => false),
      externalOpener,
      releaseFeedClient: new FakeReleaseFeedClient(),
    });

    await coordinator.initialize();

    expect(presentationState(coordinator.snapshot).state).toBe("running");
    expect(coordinator.snapshot.launcher.processOwnership).toBe("external");
    expect(managementClient.systemStatusCalls).toBe(1);
    await waitForCondition(() => expect(coordinator.snapshot.launcher.releaseCheck.status).toBe("up_to_date"));
    expect(coordinator.snapshot.launcher.releaseCheck.status).toBe("up_to_date");
  });

  test("initialize reports launcher-managed running service with separate ownership metadata", async () => {
    const settingsStore = new FakeSettingsStore();
    const endpointResolver = new FakeEndpointResolver();
    const managementClient = new FakeManagementClient();
    const processController = new FakeProcessController();
    processController.isRunning = true;

    const coordinator = createLauncherCoordinator({
      settingsStore,
      endpointResolver,
      inspectEnvironment: vi.fn(async () => okInspection()),
      managementClient,
      processController,
      isEndpointListening: vi.fn(async () => false),
      tryStopEndpointProcess: vi.fn(async () => false),
      externalOpener: new FakeExternalOpener(),
      releaseFeedClient: new FakeReleaseFeedClient(),
    });

    await coordinator.initialize();

    expect(presentationState(coordinator.snapshot).state).toBe("running");
    expect(coordinator.snapshot.launcher.processOwnership).toBe("launcher_managed");
  });

  test("initialize supports async endpoint resolution", async () => {
    const settingsStore = new FakeSettingsStore();
    const managementClient = new FakeManagementClient();
    const processController = new FakeProcessController();
    const externalOpener = new FakeExternalOpener();
    const asyncEndpointResolver = {
      resolve: vi.fn(async () => ({ host: "127.0.0.1", port: 8080, baseUrl: "http://127.0.0.1:8080/" })),
    } as unknown as ServerEndpointResolver;

    const coordinator = createLauncherCoordinator({
      settingsStore,
      endpointResolver: asyncEndpointResolver,
      inspectEnvironment: vi.fn(async () => okInspection()),
      managementClient,
      processController,
      isEndpointListening: vi.fn(async () => false),
      tryStopEndpointProcess: vi.fn(async () => false),
      externalOpener,
      releaseFeedClient: new FakeReleaseFeedClient(),
    });

    await coordinator.initialize();

    expect(coordinator.snapshot.launcher.endpoint.baseUrl).toBe("http://127.0.0.1:8080/");
  });

  test("initialize loads recovery summary from management api when service is healthy", async () => {
    const settingsStore = new FakeSettingsStore();
    const endpointResolver = new FakeEndpointResolver();
    const managementClient = new FakeManagementClient();
    managementClient.recoverySummary = {
      status: "degraded",
      phase: "post_startup",
      operation: "upgrade",
      created_at: "2026-04-02T08:00:00Z",
      updated_at: "2026-04-02T08:01:00Z",
    };

    const coordinator = createLauncherCoordinator({
      settingsStore,
      endpointResolver,
      inspectEnvironment: vi.fn(async () => okInspection()),
      managementClient,
      processController: new FakeProcessController(),
      isEndpointListening: vi.fn(async () => false),
      tryStopEndpointProcess: vi.fn(async () => false),
      externalOpener: new FakeExternalOpener(),
      releaseFeedClient: new FakeReleaseFeedClient(),
    });

    await coordinator.initialize();

    expect(resolveRecoverySummary(coordinator.snapshot)?.status).toBe("degraded");
    expect(presentationState(coordinator.snapshot).state).toBe("running");
  });

  test("initialize keeps the launcher in running state when /readyz is ready", async () => {
    const settingsStore = new FakeSettingsStore();
    const endpointResolver = new FakeEndpointResolver();
    const managementClient = new FakeManagementClient();
    managementClient.readiness = {
      status: "ready",
      checks: {
        config: "ok",
        database: "ok",
        runtime: "ok",
        render: "ok",
      },
    };

    const coordinator = createLauncherCoordinator({
      settingsStore,
      endpointResolver,
      inspectEnvironment: vi.fn(async () => okInspection()),
      managementClient,
      processController: new FakeProcessController(),
      isEndpointListening: vi.fn(async () => false),
      tryStopEndpointProcess: vi.fn(async () => false),
      externalOpener: new FakeExternalOpener(),
      releaseFeedClient: new FakeReleaseFeedClient(),
    });

    await coordinator.initialize();

    expect(presentationState(coordinator.snapshot).state).toBe("running");
    expect(coordinator.snapshot.server.readiness?.checks?.runtime).toBe("ok");
    expect(coordinator.snapshot.server.readiness?.checks).not.toHaveProperty("adapter");
  });

  test("initialize falls back to the first readiness issue when degraded has no reason", async () => {
    const settingsStore = new FakeSettingsStore();
    const endpointResolver = new FakeEndpointResolver();
    const managementClient = new FakeManagementClient();
    managementClient.readiness = {
      status: "degraded",
      checks: {
        runtime: "resource_missing",
        render: "ok",
      },
      issues: [
        {
          code: "deps.python_runtime_metadata_incomplete",
          severity: "warning",
          summary: "Python 运行环境元数据不完整。",
          remediation: "请在 .deps/manifest.json 中补齐当前平台 Python 运行环境的 archive_format、entrypoints、来源列表与 sha256。",
        },
      ],
    };

    const coordinator = createLauncherCoordinator({
      settingsStore,
      endpointResolver,
      inspectEnvironment: vi.fn(async () => okInspection()),
      managementClient,
      processController: new FakeProcessController(),
      isEndpointListening: vi.fn(async () => false),
      tryStopEndpointProcess: vi.fn(async () => false),
      externalOpener: new FakeExternalOpener(),
      releaseFeedClient: new FakeReleaseFeedClient(),
    });

    await coordinator.initialize();

    expect(presentationState(coordinator.snapshot).state).toBe("degraded");
    expect(presentationState(coordinator.snapshot).detail).toBe("Python 运行环境元数据不完整。");
    expect(coordinator.snapshot.server.readiness?.issues?.[0]?.code).toBe("deps.python_runtime_metadata_incomplete");
  });

  test("initialize auto-refreshes degraded readiness after runtime recovery", async () => {
    const settingsStore = new FakeSettingsStore();
    const endpointResolver = new FakeEndpointResolver();
    const managementClient = new FakeManagementClient();
    const processController = new FakeProcessController();
    processController.isRunning = true;
    managementClient.readiness = {
      status: "degraded",
      issues: [
        {
          code: "platform.resource_missing",
          severity: "warning",
          summary: "运行环境尚未准备完成。",
          remediation: "请准备缺失的运行环境资源。",
        },
      ],
    };

    const coordinator = createLauncherCoordinator({
      settingsStore,
      endpointResolver,
      inspectEnvironment: vi.fn(async () => okInspection()),
      managementClient,
      processController,
      isEndpointListening: vi.fn(async () => false),
      tryStopEndpointProcess: vi.fn(async () => false),
      externalOpener: new FakeExternalOpener(),
      releaseFeedClient: new FakeReleaseFeedClient(),
      options: {
        autoRefreshIntervalMs: 5,
      },
    });

    await coordinator.initialize();
    expect(presentationState(coordinator.snapshot).state).toBe("degraded");

    managementClient.readiness = {
      status: "ready",
      reason: "服务稳定。",
    };

    const readyState = await waitForPresentationState(coordinator, "running");
    expect(readyState.detail).toBe("服务稳定。");
  });

  test("initialize reflects system/status shutting_down state", async () => {
    const settingsStore = new FakeSettingsStore();
    const endpointResolver = new FakeEndpointResolver();
    const managementClient = new FakeManagementClient();
    managementClient.systemStatus = {
      status: "shutting_down",
      recovery_summary: null,
    };

    const coordinator = createLauncherCoordinator({
      settingsStore,
      endpointResolver,
      inspectEnvironment: vi.fn(async () => okInspection()),
      managementClient,
      processController: new FakeProcessController(),
      isEndpointListening: vi.fn(async () => false),
      tryStopEndpointProcess: vi.fn(async () => false),
      externalOpener: new FakeExternalOpener(),
      releaseFeedClient: new FakeReleaseFeedClient(),
    });

    await coordinator.initialize();

    expect(presentationState(coordinator.snapshot).state).toBe("stopping");
  });

  test("initialize falls back to local recovery summary when api path is unavailable", async () => {
    const settingsStore = new FakeSettingsStore();
    const endpointResolver = new FakeEndpointResolver();
    const managementClient = new FakeManagementClient();
    const processController = new FakeProcessController();
    const recoverySummaryReader = new FakeRecoverySummaryReader();
    recoverySummaryReader.summary = {
      status: "blocked",
      phase: "pre_restore",
      operation: "rollback",
      created_at: "2026-04-02T08:00:00Z",
      updated_at: "2026-04-02T08:01:00Z",
    };
    managementClient.health = false;

    const coordinator = createLauncherCoordinator({
      settingsStore,
      endpointResolver,
      inspectEnvironment: vi.fn(async () => okInspection()),
      managementClient,
      processController,
      isEndpointListening: vi.fn(async () => false),
      tryStopEndpointProcess: vi.fn(async () => false),
      externalOpener: new FakeExternalOpener(),
      releaseFeedClient: new FakeReleaseFeedClient(),
      recoverySummaryReader,
    });

    await coordinator.initialize();

    expect(resolveRecoverySummary(coordinator.snapshot)?.status).toBe("blocked");
    expect(coordinator.snapshot.server.readiness).toBeNull();
  });

  test("open web ui opens plain management urls", async () => {
    const settingsStore = new FakeSettingsStore();
    const endpointResolver = new FakeEndpointResolver();
    const managementClient = new FakeManagementClient();
    const processController = new FakeProcessController();
    const externalOpener = new FakeExternalOpener();

    const coordinator = createLauncherCoordinator({
      settingsStore,
      endpointResolver,
      inspectEnvironment: vi.fn(async () => okInspection()),
      managementClient,
      processController,
      isEndpointListening: vi.fn(async () => false),
      tryStopEndpointProcess: vi.fn(async () => false),
      externalOpener,
      releaseFeedClient: new FakeReleaseFeedClient(),
    });

    await coordinator.initialize();
    const detailBeforeOpen = presentationState(coordinator.snapshot).detail;
    await coordinator.openWebUi("/logs?source=tasks");

    expect(externalOpener.openedUris.at(-1)).toContain("/logs?source=tasks");
    expect(externalOpener.openedUris.at(-1)).not.toContain("token=");
    expect(presentationState(coordinator.snapshot).detail).toBe(detailBeforeOpen);

    await coordinator.openWebUi();

    const latestUri = externalOpener.openedUris.at(-1) ?? "";
    expect(latestUri.endsWith("/")).toBe(true);
    expect(latestUri.includes("?token=")).toBe(false);
  });

  test("open web ui falls back to the plain url when setup status cannot be read", async () => {
    const settingsStore = new FakeSettingsStore();
    const endpointResolver = new FakeEndpointResolver();
    const managementClient = new FakeManagementClient();
    const processController = new FakeProcessController();
    const externalOpener = new FakeExternalOpener();

    managementClient.getSetupInitialized = vi.fn(async () => {
      throw new Error("setup status unavailable");
    });

    const coordinator = createLauncherCoordinator({
      settingsStore,
      endpointResolver,
      inspectEnvironment: vi.fn(async () => okInspection()),
      managementClient,
      processController,
      isEndpointListening: vi.fn(async () => false),
      tryStopEndpointProcess: vi.fn(async () => false),
      externalOpener,
      releaseFeedClient: new FakeReleaseFeedClient(),
    });

    await coordinator.initialize();
    await coordinator.openWebUi("/plugins/weather-pro");

    expect(externalOpener.openedUris.at(-1)).toBe("http://127.0.0.1:8080/plugins/weather-pro");
  });

  test("open web ui rejects absolute external targets", async () => {
    const settingsStore = new FakeSettingsStore();
    const endpointResolver = new FakeEndpointResolver();
    const managementClient = new FakeManagementClient();
    const processController = new FakeProcessController();
    const externalOpener = new FakeExternalOpener();

    const coordinator = createLauncherCoordinator({
      settingsStore,
      endpointResolver,
      inspectEnvironment: vi.fn(async () => okInspection()),
      managementClient,
      processController,
      isEndpointListening: vi.fn(async () => false),
      tryStopEndpointProcess: vi.fn(async () => false),
      externalOpener,
      releaseFeedClient: new FakeReleaseFeedClient(),
    });

    await coordinator.initialize();

    await expect(coordinator.openWebUi("https://evil.example/pwn")).rejects.toThrow(
      "启动器只允许打开管理界面的相对路径。",
    );
    expect(externalOpener.openedUris).toHaveLength(0);
  });

  test("start does not launch another process when endpoint is already healthy", async () => {
    const settingsStore = new FakeSettingsStore();
    const endpointResolver = new FakeEndpointResolver();
    const managementClient = new FakeManagementClient();
    const processController = new FakeProcessController();

    const coordinator = createLauncherCoordinator({
      settingsStore,
      endpointResolver,
      inspectEnvironment: vi.fn(async () => okInspection()),
      managementClient,
      processController,
      isEndpointListening: vi.fn(async () => false),
      tryStopEndpointProcess: vi.fn(async () => false),
      externalOpener: new FakeExternalOpener(),
      releaseFeedClient: new FakeReleaseFeedClient(),
    });

    await coordinator.initialize();
    await coordinator.start();

    expect(processController.startCalls).toBe(0);
    expect(presentationState(coordinator.snapshot).state).toBe("running");
    expect(coordinator.snapshot.launcher.processOwnership).toBe("external");
  });

  test("start keeps the launcher in starting state while runtime preparation is still in progress", async () => {
    const settingsStore = new FakeSettingsStore();
    const endpointResolver = new FakeEndpointResolver();
    const managementClient = new FakeManagementClient();
    const processController = new FakeProcessController();
    let ready = false;

    managementClient.health = false;
    managementClient.isHealthy = vi.fn(async () => ready);

    const coordinator = createLauncherCoordinator({
      settingsStore,
      endpointResolver,
      inspectEnvironment: vi.fn(async () => okInspection()),
      managementClient,
      processController,
      isEndpointListening: vi.fn(async () => false),
      tryStopEndpointProcess: vi.fn(async () => false),
      externalOpener: new FakeExternalOpener(),
      releaseFeedClient: new FakeReleaseFeedClient(),
      options: {
        pollIntervalMs: 1,
        startupTimeoutMs: 100,
        shutdownTimeoutMs: 1,
      },
    });

    await coordinator.initialize();

    const startPromise = coordinator.start();
    await new Promise((resolve) => setTimeout(resolve, 10));

    expect(presentationState(coordinator.snapshot).state).toBe("starting");
    expect(presentationState(coordinator.snapshot).detail).toContain("正在准备运行环境并等待服务就绪");

    ready = true;
    await startPromise;

    expect(presentationState(coordinator.snapshot).state).toBe("running");
  });

  test("start publishes runtime preparation progress before the server is ready", async () => {
    const settingsStore = new FakeSettingsStore();
    const endpointResolver = new FakeEndpointResolver();
    const managementClient = new FakeManagementClient();
    const processController = new FakeProcessController();
    let ready = false;

    managementClient.health = false;
    managementClient.isHealthy = vi.fn(async () => ready);

    const coordinator = createLauncherCoordinator({
      settingsStore,
      endpointResolver,
      inspectEnvironment: vi.fn(async () => okInspection()),
      managementClient,
      processController,
      isEndpointListening: vi.fn(async () => false),
      tryStopEndpointProcess: vi.fn(async () => false),
      externalOpener: new FakeExternalOpener(),
      options: {
        pollIntervalMs: 1,
        startupTimeoutMs: 200,
        shutdownTimeoutMs: 1,
      },
    });

    await coordinator.initialize();
    const startPromise = coordinator.start();
    await new Promise((resolve) => setTimeout(resolve, 10));

    processController.runtimePrepare = {
      active: true,
      currentKind: "chromium",
      summary: "正在下载 图片渲染 Chromium",
      resources: [
        {
          kind: "chromium",
          label: "图片渲染 Chromium",
          resourceId: "chromium-windows-x64",
          version: "147.0.7727.24",
          sourceLabel: "Chrome for Testing",
          sourceUrl: "https://example.invalid/chrome.zip",
          archivePath: "C:\\RayleaBot\\cache\\downloads\\runtime\\chromium-windows-x64.zip",
          storeRoot: "C:\\RayleaBot\\.deps\\store\\chromium-windows-x64\\147.0.7727.24",
          stage: "download",
          status: "running",
          progress: 42,
          downloadedBytes: 1024,
          totalBytes: 2048,
          extractedEntries: null,
          totalEntries: null,
          summary: "正在下载 图片渲染 Chromium",
          error: "",
          updatedAt: "2026-06-06T00:00:00Z",
        },
      ],
    };
    await new Promise((resolve) => setTimeout(resolve, 20));

    expect(coordinator.snapshot.launcher.runtimePrepare?.summary).toBe("正在下载 图片渲染 Chromium");
    expect(coordinator.snapshot.launcher.runtimePrepare?.resources[0]?.progress).toBe(42);
    expect(presentationState(coordinator.snapshot).detail).toContain("正在下载 图片渲染 Chromium");

    ready = true;
    await startPromise;

    expect(presentationState(coordinator.snapshot).state).toBe("running");
    expect(coordinator.snapshot.launcher.runtimePrepare).toBeNull();
  });

  test("start waits for /readyz before finalizing a successful startup", async () => {
    const settingsStore = new FakeSettingsStore();
    const endpointResolver = new FakeEndpointResolver();
    const managementClient = new FakeManagementClient();
    const processController = new FakeProcessController();
    let healthChecks = 0;
    let readinessChecks = 0;

    managementClient.health = false;
    managementClient.isHealthy = vi.fn(async () => {
      healthChecks += 1;
      return healthChecks >= 3;
    });
    managementClient.getReadiness = vi.fn(async () => {
      readinessChecks += 1;
      if (readinessChecks === 1) {
        throw new Error("readyz warming up");
      }
      return managementClient.readiness;
    });

    const coordinator = createLauncherCoordinator({
      settingsStore,
      endpointResolver,
      inspectEnvironment: vi.fn(async () => okInspection()),
      managementClient,
      processController,
      isEndpointListening: vi.fn(async () => false),
      tryStopEndpointProcess: vi.fn(async () => false),
      externalOpener: new FakeExternalOpener(),
      releaseFeedClient: new FakeReleaseFeedClient(),
      options: {
        pollIntervalMs: 1,
        startupTimeoutMs: 500,
        shutdownTimeoutMs: 1,
      },
    });

    await coordinator.initialize();
    await coordinator.start();

    expect(processController.startCalls).toBe(1);
    expect(readinessChecks).toBeGreaterThanOrEqual(2);
    expect(presentationState(coordinator.snapshot).state).toBe("running");
    expect(coordinator.snapshot.launcher.lastLocalError).toBe("");
  });

  test("start ignores transient failed readiness snapshots until the service settles", async () => {
    const settingsStore = new FakeSettingsStore();
    const endpointResolver = new FakeEndpointResolver();
    const managementClient = new FakeManagementClient();
    const processController = new FakeProcessController();
    let healthChecks = 0;
    let readinessChecks = 0;

    managementClient.health = false;
    managementClient.isHealthy = vi.fn(async () => {
      healthChecks += 1;
      return healthChecks >= 3;
    });
    managementClient.getReadiness = vi.fn(async () => {
      readinessChecks += 1;
      if (readinessChecks === 1) {
        return {
          status: "failed",
          reason: "服务仍在完成启动。",
        };
      }
      return managementClient.readiness;
    });

    const coordinator = createLauncherCoordinator({
      settingsStore,
      endpointResolver,
      inspectEnvironment: vi.fn(async () => okInspection()),
      managementClient,
      processController,
      isEndpointListening: vi.fn(async () => false),
      tryStopEndpointProcess: vi.fn(async () => false),
      externalOpener: new FakeExternalOpener(),
      releaseFeedClient: new FakeReleaseFeedClient(),
      options: {
        pollIntervalMs: 1,
        startupTimeoutMs: 500,
        startupReadinessGraceMs: 25,
        shutdownTimeoutMs: 1,
      },
    });

    await coordinator.initialize();
    await coordinator.start();

    expect(processController.startCalls).toBe(1);
    expect(readinessChecks).toBeGreaterThanOrEqual(2);
    expect(presentationState(coordinator.snapshot).state).toBe("running");
    expect(coordinator.snapshot.launcher.lastLocalError).toBe("");
  });

  test("start preserves setup_required when startup reaches the setup gate", async () => {
    const settingsStore = new FakeSettingsStore();
    const endpointResolver = new FakeEndpointResolver();
    const managementClient = new FakeManagementClient();
    const processController = new FakeProcessController();
    let healthChecks = 0;

    managementClient.health = false;
    managementClient.readiness = {
      status: "setup_required",
      reason: "管理员初始化尚未完成。",
    };
    managementClient.isHealthy = vi.fn(async () => {
      healthChecks += 1;
      return healthChecks >= 3;
    });

    const coordinator = createLauncherCoordinator({
      settingsStore,
      endpointResolver,
      inspectEnvironment: vi.fn(async () => okInspection()),
      managementClient,
      processController,
      isEndpointListening: vi.fn(async () => false),
      tryStopEndpointProcess: vi.fn(async () => false),
      externalOpener: new FakeExternalOpener(),
      releaseFeedClient: new FakeReleaseFeedClient(),
      options: {
        pollIntervalMs: 1,
        startupTimeoutMs: 500,
        shutdownTimeoutMs: 1,
      },
    });

    await coordinator.initialize();
    await coordinator.start();

    expect(processController.startCalls).toBe(1);
    expect(presentationState(coordinator.snapshot).state).toBe("setup_required");
    expect(coordinator.snapshot.launcher.processOwnership).toBe("launcher_managed");
    expect(presentationState(coordinator.snapshot).detail).toContain("管理员初始化");
  });

  test("stop keeps an external service running when the confirmation is declined", async () => {
    const settingsStore = new FakeSettingsStore();
    const endpointResolver = new FakeEndpointResolver();
    const managementClient = new FakeManagementClient();
    const processController = new FakeProcessController();
    const tryStopEndpointProcess = vi.fn(async () => false);
    managementClient.shutdownFromLauncher = vi.fn(async () => undefined);

    const coordinator = createLauncherCoordinator({
      settingsStore,
      endpointResolver,
      inspectEnvironment: vi.fn(async () => okInspection()),
      managementClient,
      processController,
      isEndpointListening: vi.fn(async () => false),
      tryStopEndpointProcess,
      externalOpener: new FakeExternalOpener(),
      releaseFeedClient: new FakeReleaseFeedClient(),
      confirmExternalServiceStop: vi.fn(async () => false),
    } as any);

    await coordinator.initialize();
    await coordinator.stop();

    expect(managementClient.shutdownFromLauncher).not.toHaveBeenCalled();
    expect(processController.forceKillCalls).toBe(0);
    expect(tryStopEndpointProcess).not.toHaveBeenCalled();
    expect(presentationState(coordinator.snapshot).state).toBe("running");
    expect(coordinator.snapshot.launcher.processOwnership).toBe("external");
  });

  test("stop surfaces external launcher shutdown failures without force killing the foreign process", async () => {
    const settingsStore = new FakeSettingsStore();
    const endpointResolver = new FakeEndpointResolver();
    const managementClient = new FakeManagementClient();
    const processController = new FakeProcessController();
    const tryStopEndpointProcess = vi.fn(async () => false);
    managementClient.shutdownFromLauncher = vi.fn(async () => {
      throw new Error("launcher shutdown failed");
    });

    const coordinator = createLauncherCoordinator({
      settingsStore,
      endpointResolver,
      inspectEnvironment: vi.fn(async () => okInspection()),
      managementClient,
      processController,
      isEndpointListening: vi.fn(async () => false),
      tryStopEndpointProcess,
      externalOpener: new FakeExternalOpener(),
      releaseFeedClient: new FakeReleaseFeedClient(),
      confirmExternalServiceStop: vi.fn(async () => true),
      options: {
        pollIntervalMs: 1,
        startupTimeoutMs: 10,
        shutdownTimeoutMs: 1,
      },
    } as any);

    await coordinator.initialize();
    await coordinator.stop();

    expect(processController.forceKillCalls).toBe(0);
    expect(tryStopEndpointProcess).not.toHaveBeenCalled();
    expect(presentationState(coordinator.snapshot).state).toBe("running");
    expect(coordinator.snapshot.launcher.lastLocalError).toContain("launcher shutdown failed");
  });

  test("stop waits for the managed process to exit before reporting final state", async () => {
    const settingsStore = new FakeSettingsStore();
    const endpointResolver = new FakeEndpointResolver();
    const managementClient = new FakeManagementClient();
    const processController = new FakeProcessController();
    processController.isRunning = true;

    managementClient.health = true;
    managementClient.shutdownFromLauncher = vi.fn(async () => {
      managementClient.health = false;
    });

    const coordinator = createLauncherCoordinator({
      settingsStore,
      endpointResolver,
      inspectEnvironment: vi.fn(async () => okInspection()),
      managementClient,
      processController,
      isEndpointListening: vi.fn(async () => false),
      tryStopEndpointProcess: vi.fn(async () => false),
      externalOpener: new FakeExternalOpener(),
      releaseFeedClient: new FakeReleaseFeedClient(),
      options: {
        pollIntervalMs: 1,
        startupTimeoutMs: 10,
        shutdownTimeoutMs: 1,
      },
    });

    await coordinator.initialize();

    managementClient.health = true;
    await coordinator.stop();

    expect(processController.forceKillCalls).toBe(1);
    expect(presentationState(coordinator.snapshot).state).toBe("stopped");
  });

  test("stop falls back to force kill when launcher shutdown fails", async () => {
    const settingsStore = new FakeSettingsStore();
    const endpointResolver = new FakeEndpointResolver();
    const managementClient = new FakeManagementClient();
    const processController = new FakeProcessController();
    processController.isRunning = true;
    processController.forceKill = vi.fn(async () => {
      processController.forceKillCalls += 1;
      processController.isRunning = false;
      managementClient.health = false;
    });
    managementClient.shutdownFromLauncher = vi.fn(async () => {
      throw new Error("launcher shutdown failed");
    });

    const coordinator = createLauncherCoordinator({
      settingsStore,
      endpointResolver,
      inspectEnvironment: vi.fn(async () => okInspection()),
      managementClient,
      processController,
      isEndpointListening: vi.fn(async () => false),
      tryStopEndpointProcess: vi.fn(async () => false),
      externalOpener: new FakeExternalOpener(),
      releaseFeedClient: new FakeReleaseFeedClient(),
      options: {
        pollIntervalMs: 1,
        startupTimeoutMs: 10,
        shutdownTimeoutMs: 1,
      },
    });

    await coordinator.initialize();
    await coordinator.stop();

    expect(processController.forceKillCalls).toBe(1);
    expect(presentationState(coordinator.snapshot).state).toBe("stopped");
  });

  test("start fails early when the managed process exits before health checks recover", async () => {
    const settingsStore = new FakeSettingsStore();
    const endpointResolver = new FakeEndpointResolver();
    const managementClient = new FakeManagementClient();
    const processController = new FakeProcessController();
    processController.recentStderr = ["config validation failed"];

    managementClient.health = false;
    managementClient.isHealthy = vi.fn(async () => {
      processController.isRunning = false;
      return false;
    });

    const coordinator = createLauncherCoordinator({
      settingsStore,
      endpointResolver,
      inspectEnvironment: vi.fn(async () => okInspection()),
      managementClient,
      processController,
      isEndpointListening: vi.fn(async () => false),
      tryStopEndpointProcess: vi.fn(async () => false),
      externalOpener: new FakeExternalOpener(),
      releaseFeedClient: new FakeReleaseFeedClient(),
      options: {
        pollIntervalMs: 1,
        startupTimeoutMs: 50,
        shutdownTimeoutMs: 1,
      },
    });

    await coordinator.initialize();
    await coordinator.start();

    expect(processController.forceKillCalls).toBe(0);
    expect(presentationState(coordinator.snapshot).state).toBe("failed");
    expect(coordinator.snapshot.launcher.lastLocalError).toContain("config validation failed");
  });

  test("start treats a post-exit healthy endpoint as an existing running service", async () => {
    const settingsStore = new FakeSettingsStore();
    const endpointResolver = new FakeEndpointResolver();
    const managementClient = new FakeManagementClient();
    const processController = new FakeProcessController();
    let healthChecks = 0;

    managementClient.health = false;
    managementClient.isHealthy = vi.fn(async () => {
      healthChecks += 1;
      if (healthChecks <= 2) {
        return false;
      }
      if (healthChecks === 3) {
        processController.isRunning = false;
        return false;
      }
      return true;
    });

    const coordinator = createLauncherCoordinator({
      settingsStore,
      endpointResolver,
      inspectEnvironment: vi.fn(async () => okInspection()),
      managementClient,
      processController,
      isEndpointListening: vi.fn(async () => false),
      tryStopEndpointProcess: vi.fn(async () => false),
      externalOpener: new FakeExternalOpener(),
      releaseFeedClient: new FakeReleaseFeedClient(),
      options: {
        pollIntervalMs: 1,
        startupTimeoutMs: 50,
        shutdownTimeoutMs: 1,
      },
    });

    await coordinator.initialize();
    await coordinator.start();

    expect(processController.startCalls).toBe(1);
    expect(presentationState(coordinator.snapshot).state).toBe("running");
    expect(coordinator.snapshot.launcher.processOwnership).toBe("external");
  });

  test("start reports port occupation when the child exits and another process is still listening", async () => {
    const settingsStore = new FakeSettingsStore();
    const endpointResolver = new FakeEndpointResolver();
    const managementClient = new FakeManagementClient();
    const processController = new FakeProcessController();
    processController.recentStderr = ["listen on 127.0.0.1:8080: bind: address already in use"];

    managementClient.health = false;
    managementClient.isHealthy = vi.fn(async () => {
      processController.isRunning = false;
      return false;
    });
    const isEndpointListening = vi.fn()
      .mockResolvedValueOnce(false)
      .mockResolvedValue(true);

    const coordinator = createLauncherCoordinator({
      settingsStore,
      endpointResolver,
      inspectEnvironment: vi.fn(async () => okInspection()),
      managementClient,
      processController,
      isEndpointListening,
      tryStopEndpointProcess: vi.fn(async () => false),
      externalOpener: new FakeExternalOpener(),
      releaseFeedClient: new FakeReleaseFeedClient(),
      options: {
        pollIntervalMs: 1,
        startupTimeoutMs: 50,
        shutdownTimeoutMs: 1,
      },
    });

    await coordinator.initialize();
    await coordinator.start();

    expect(presentationState(coordinator.snapshot).state).toBe("failed");
    expect(presentationState(coordinator.snapshot).detail).toContain("目标端口已被现有进程占用");
    expect(coordinator.snapshot.launcher.lastLocalError).toContain("bind: address already in use");
  });

  test("initialize reports setup_required when /readyz says setup is still required", async () => {
    const settingsStore = new FakeSettingsStore();
    const endpointResolver = new FakeEndpointResolver();
    const managementClient = new FakeManagementClient();
    managementClient.readiness = {
      status: "setup_required",
      reason: "管理员初始化尚未完成。",
    };
    const processController = new FakeProcessController();
    processController.isRunning = true;

    const coordinator = createLauncherCoordinator({
      settingsStore,
      endpointResolver,
      inspectEnvironment: vi.fn(async () => okInspection()),
      managementClient,
      processController,
      isEndpointListening: vi.fn(async () => false),
      tryStopEndpointProcess: vi.fn(async () => false),
      externalOpener: new FakeExternalOpener(),
      releaseFeedClient: new FakeReleaseFeedClient(),
    });

    await coordinator.initialize();

    expect(presentationState(coordinator.snapshot).state).toBe("setup_required");
    expect(presentationState(coordinator.snapshot).detail).toContain("管理员初始化");
  });

  test("initialize auto-refreshes setup_required after administrator setup completes", async () => {
    const settingsStore = new FakeSettingsStore();
    const endpointResolver = new FakeEndpointResolver();
    const managementClient = new FakeManagementClient();
    managementClient.readiness = {
      status: "setup_required",
      reason: "管理员初始化尚未完成。",
    };
    const processController = new FakeProcessController();
    processController.isRunning = true;

    const coordinator = createLauncherCoordinator({
      settingsStore,
      endpointResolver,
      inspectEnvironment: vi.fn(async () => okInspection()),
      managementClient,
      processController,
      isEndpointListening: vi.fn(async () => false),
      tryStopEndpointProcess: vi.fn(async () => false),
      externalOpener: new FakeExternalOpener(),
      releaseFeedClient: new FakeReleaseFeedClient(),
      options: {
        autoRefreshIntervalMs: 5,
      },
    });

    await coordinator.initialize();
    expect(presentationState(coordinator.snapshot).state).toBe("setup_required");

    managementClient.readiness = {
      status: "ready",
      reason: "服务稳定。",
    };

    const readyState = await waitForPresentationState(coordinator, "running");
    expect(readyState.detail).toBe("服务稳定。");
  });

  test("initialize does not block on slow release checks", async () => {
    const settingsStore = new FakeSettingsStore();
    const endpointResolver = new FakeEndpointResolver();
    const managementClient = new FakeManagementClient();
    const readyRelease = releaseSnapshot({
      status: "up_to_date",
      currentVersion: "0.1.0",
      latestVersion: "0.1.0",
      summary: "当前版本 0.1.0 已是最新。",
      detail: "",
      releasePageUrl: "https://example.invalid/releases/v0.1.0",
      updateAvailable: false,
      canCheck: true,
    });
    let resolveRelease: ((value: ReleaseCheckSnapshot) => void) | null = null;
    const slowReleaseClient: ReleaseFeedClient = {
      getSnapshot: vi.fn(
        () =>
          new Promise((resolve) => {
            resolveRelease = resolve;
          }),
      ),
      downloadUpdate: vi.fn(async () => readyRelease),
      installDownloadedUpdate: vi.fn(async () => readyRelease),
    };

    const coordinator = createLauncherCoordinator({
      settingsStore,
      endpointResolver,
      inspectEnvironment: vi.fn(async () => okInspection()),
      managementClient,
      processController: new FakeProcessController(),
      isEndpointListening: vi.fn(async () => false),
      tryStopEndpointProcess: vi.fn(async () => false),
      externalOpener: new FakeExternalOpener(),
      releaseFeedClient: slowReleaseClient,
    });

    const result = await Promise.race([
      coordinator.initialize().then(() => "resolved"),
      new Promise<string>((resolve) => setTimeout(() => resolve("timeout"), 1000)),
    ]);

    expect(result).toBe("resolved");
    expect(coordinator.snapshot.launcher.releaseCheck.status).toBe("checking");

    resolveRelease?.(readyRelease);
  });

  test("runs a background release check on startup and forces manual checks", async () => {
    const getSnapshot = vi.fn(async (options?: { force?: boolean }) =>
      releaseSnapshot({
        status: "up_to_date",
        currentVersion: "1.0.0",
        latestVersion: "1.0.0",
        summary: "当前版本 1.0.0 已是最新。",
        releasePageUrl: "https://example.invalid/releases/v1.0.0",
        canCheck: true,
      }),
    );
    const releaseFeedClient: ReleaseFeedClient = {
      getSnapshot,
      downloadUpdate: vi.fn(async () => releaseSnapshot()),
      installDownloadedUpdate: vi.fn(async () => releaseSnapshot()),
    };

    const coordinator = createLauncherCoordinator({
      settingsStore: new FakeSettingsStore(),
      endpointResolver: new FakeEndpointResolver(),
      inspectEnvironment: vi.fn(async () => okInspection()),
      managementClient: new FakeManagementClient(),
      processController: new FakeProcessController(),
      isEndpointListening: vi.fn(async () => false),
      tryStopEndpointProcess: vi.fn(async () => false),
      externalOpener: new FakeExternalOpener(),
      releaseFeedClient,
    });

    await coordinator.initialize();
    await waitForCondition(() => expect(coordinator.snapshot.launcher.releaseCheck.status).toBe("up_to_date"));

    expect(getSnapshot).toHaveBeenLastCalledWith({ force: false });

    await coordinator.checkForUpdates();

    expect(getSnapshot).toHaveBeenCalledTimes(2);
    expect(getSnapshot).toHaveBeenLastCalledWith({ force: true });
  });

  test("schedules release checks at the configured interval", async () => {
    vi.useFakeTimers();
    const getSnapshot = vi.fn(async () =>
      releaseSnapshot({
        status: "up_to_date",
        currentVersion: "1.0.0",
        latestVersion: "1.0.0",
        summary: "当前版本 1.0.0 已是最新。",
        releasePageUrl: "https://example.invalid/releases/v1.0.0",
        canCheck: true,
      }),
    );
    const releaseFeedClient: ReleaseFeedClient = {
      getSnapshot,
      downloadUpdate: vi.fn(async () => releaseSnapshot()),
      installDownloadedUpdate: vi.fn(async () => releaseSnapshot()),
    };

    const coordinator = createLauncherCoordinator({
      settingsStore: new FakeSettingsStore(),
      endpointResolver: new FakeEndpointResolver(),
      inspectEnvironment: vi.fn(async () => okInspection()),
      managementClient: new FakeManagementClient(),
      processController: new FakeProcessController(),
      isEndpointListening: vi.fn(async () => false),
      tryStopEndpointProcess: vi.fn(async () => false),
      externalOpener: new FakeExternalOpener(),
      releaseFeedClient,
      options: {
        releaseCheckIntervalMs: 10,
      },
    });

    await coordinator.initialize();
    await Promise.resolve();
    await Promise.resolve();

    expect(getSnapshot).toHaveBeenCalledTimes(1);

    await vi.advanceTimersByTimeAsync(10);

    expect(getSnapshot).toHaveBeenCalledTimes(2);
  });

  test("does not start duplicate update work while a release check is active", async () => {
    const readyRelease = releaseSnapshot({
      status: "up_to_date",
      currentVersion: "1.0.0",
      latestVersion: "1.0.0",
      summary: "当前版本 1.0.0 已是最新。",
      releasePageUrl: "https://example.invalid/releases/v1.0.0",
      canCheck: true,
    });
    let resolveRelease: ((value: ReleaseCheckSnapshot) => void) | null = null;
    const releaseFeedClient: ReleaseFeedClient = {
      getSnapshot: vi.fn(
        () =>
          new Promise((resolve) => {
            resolveRelease = resolve;
          }),
      ),
      downloadUpdate: vi.fn(async () => readyRelease),
      installDownloadedUpdate: vi.fn(async () => readyRelease),
    };

    const coordinator = createLauncherCoordinator({
      settingsStore: new FakeSettingsStore(),
      endpointResolver: new FakeEndpointResolver(),
      inspectEnvironment: vi.fn(async () => okInspection()),
      managementClient: new FakeManagementClient(),
      processController: new FakeProcessController(),
      isEndpointListening: vi.fn(async () => false),
      tryStopEndpointProcess: vi.fn(async () => false),
      externalOpener: new FakeExternalOpener(),
      releaseFeedClient,
    });

    await coordinator.initialize();
    await waitForCondition(() => expect(releaseFeedClient.getSnapshot).toHaveBeenCalledTimes(1));

    await coordinator.checkForUpdates();
    await coordinator.downloadUpdate();

    expect(releaseFeedClient.getSnapshot).toHaveBeenCalledTimes(1);
    expect(releaseFeedClient.downloadUpdate).not.toHaveBeenCalled();

    resolveRelease?.(readyRelease);
  });

  test("publishes download progress and waits for explicit install", async () => {
    const availableRelease = releaseSnapshot({
      status: "update_available",
      currentVersion: "1.0.0",
      latestVersion: "1.2.0",
      summary: "发现新版本 1.2.0。",
      releasePageUrl: "https://example.invalid/releases/v1.2.0",
      updateAvailable: true,
      artifactFileName: "RayleaBot-1.2.0-windows-x64-full.zip",
      totalBytes: 100,
      canCheck: true,
      canDownload: true,
    });
    const downloadedRelease = releaseSnapshot({
      ...availableRelease,
      status: "downloaded",
      summary: "新版本 1.2.0 已下载。",
      downloadProgress: 1,
      downloadedBytes: 100,
      canDownload: false,
      canInstall: true,
    });
    const installDownloadedUpdate = vi.fn(async () => releaseSnapshot({ ...downloadedRelease, status: "installing" }));
    const releaseFeedClient: ReleaseFeedClient = {
      getSnapshot: vi.fn(async () => availableRelease),
      downloadUpdate: vi.fn(async (onProgress) => {
        await onProgress?.(releaseSnapshot({
          ...availableRelease,
          status: "downloading",
          downloadProgress: 0.5,
          downloadedBytes: 50,
          canCheck: false,
          canDownload: false,
        }));
        return downloadedRelease;
      }),
      installDownloadedUpdate,
    };

    const coordinator = createLauncherCoordinator({
      settingsStore: new FakeSettingsStore(),
      endpointResolver: new FakeEndpointResolver(),
      inspectEnvironment: vi.fn(async () => okInspection()),
      managementClient: new FakeManagementClient(),
      processController: new FakeProcessController(),
      isEndpointListening: vi.fn(async () => false),
      tryStopEndpointProcess: vi.fn(async () => false),
      externalOpener: new FakeExternalOpener(),
      releaseFeedClient,
    });

    await coordinator.initialize();
    await waitForCondition(() => expect(coordinator.snapshot.launcher.releaseCheck.status).toBe("update_available"));

    await coordinator.downloadUpdate();

    expect(coordinator.snapshot.launcher.releaseCheck.status).toBe("downloaded");
    expect(coordinator.snapshot.launcher.releaseCheck.downloadProgress).toBe(1);
    expect(installDownloadedUpdate).not.toHaveBeenCalled();
  });

  test("stops the managed service before starting the update helper", async () => {
    const processController = new FakeProcessController();
    processController.isRunning = true;
    const installDownloadedUpdate = vi.fn(async (appProcessId: number) =>
      releaseSnapshot({
        status: "installing",
        currentVersion: "1.0.0",
        latestVersion: "1.2.0",
        summary: "正在重启并安装更新。",
        releasePageUrl: "https://example.invalid/releases/v1.2.0",
        updateAvailable: true,
        canCheck: false,
        canDownload: false,
        canInstall: false,
      }),
    );
    const releaseFeedClient: ReleaseFeedClient = {
      getSnapshot: vi.fn(async () =>
        releaseSnapshot({
          status: "downloaded",
          currentVersion: "1.0.0",
          latestVersion: "1.2.0",
          summary: "新版本 1.2.0 已下载。",
          releasePageUrl: "https://example.invalid/releases/v1.2.0",
          updateAvailable: true,
          canCheck: true,
          canInstall: true,
        }),
      ),
      downloadUpdate: vi.fn(async () => releaseSnapshot()),
      installDownloadedUpdate,
    };

    const coordinator = createLauncherCoordinator({
      settingsStore: new FakeSettingsStore(),
      endpointResolver: new FakeEndpointResolver(),
      inspectEnvironment: vi.fn(async () => okInspection()),
      managementClient: new FakeManagementClient(),
      processController,
      isEndpointListening: vi.fn(async () => false),
      tryStopEndpointProcess: vi.fn(async () => false),
      externalOpener: new FakeExternalOpener(),
      releaseFeedClient,
      options: {
        pollIntervalMs: 1,
        shutdownTimeoutMs: 1,
      },
    });

    await coordinator.initialize();
    await waitForCondition(() => expect(coordinator.snapshot.launcher.releaseCheck.status).toBe("downloaded"));

    await coordinator.prepareUpdateInstall(12345);

    expect(processController.forceKillCalls).toBeGreaterThan(0);
    expect(installDownloadedUpdate).toHaveBeenCalledWith(12345);
    expect(coordinator.snapshot.launcher.releaseCheck.status).toBe("installing");
  });

  test("reset admin waits for startup readiness before opening the setup entry", async () => {
    const settingsStore = new FakeSettingsStore();
    const endpointResolver = new FakeEndpointResolver();
    const managementClient = new FakeManagementClient();
    const processController = new FakeProcessController();
    const externalOpener = new FakeExternalOpener();
    const resetAdminRunner = new FakeResetAdminRunner();
    let healthChecks = 0;

    managementClient.health = false;
    managementClient.isHealthy = vi.fn(async () => {
      healthChecks += 1;
      return healthChecks >= 3;
    });
    managementClient.getReadiness = vi.fn(async () => managementClient.readiness);
    managementClient.readiness = {
      status: "setup_required",
      reason: "管理员初始化尚未完成。",
    };

    const coordinator = createLauncherCoordinator({
      settingsStore,
      endpointResolver,
      inspectEnvironment: vi.fn(async () => okInspection()),
      managementClient,
      processController,
      isEndpointListening: vi.fn(async () => false),
      tryStopEndpointProcess: vi.fn(async () => false),
      externalOpener,
      releaseFeedClient: new FakeReleaseFeedClient(),
      resetAdminRunner,
      options: {
        pollIntervalMs: 1,
        startupTimeoutMs: 25,
        shutdownTimeoutMs: 1,
      },
    });

    await coordinator.initialize();

    await coordinator.resetAdmin();

    expect(resetAdminRunner.calls).toBe(1);
    expect(processController.startCalls).toBe(1);
    expect(managementClient.getReadiness).toHaveBeenCalled();
    expect(presentationState(coordinator.snapshot).state).toBe("setup_required");
    expect(externalOpener.openedUris.at(-1)).toBe("http://127.0.0.1:8080/");
  });

  test("reset admin surfaces start failure with contextual error", async () => {
    const settingsStore = new FakeSettingsStore();
    const endpointResolver = new FakeEndpointResolver();
    const managementClient = new FakeManagementClient();
    const processController = new FakeProcessController();
    const externalOpener = new FakeExternalOpener();
    const resetAdminRunner = new FakeResetAdminRunner();

    managementClient.health = false;

    processController.start = vi.fn(async () => {
      throw new Error("spawn ENOENT");
    });

    const coordinator = createLauncherCoordinator({
      settingsStore,
      endpointResolver,
      inspectEnvironment: vi.fn(async () => okInspection()),
      managementClient,
      processController,
      isEndpointListening: vi.fn(async () => false),
      tryStopEndpointProcess: vi.fn(async () => false),
      externalOpener,
      releaseFeedClient: new FakeReleaseFeedClient(),
      resetAdminRunner,
    });

    await coordinator.initialize();

    managementClient.setupInitialized = false;

    await coordinator.resetAdmin();

    expect(resetAdminRunner.calls).toBe(1);
    expect(presentationState(coordinator.snapshot).state).toBe("failed");
    expect(coordinator.snapshot.launcher.lastLocalError).toContain("spawn ENOENT");
    expect(presentationState(coordinator.snapshot).detail).toContain("管理员凭据已重置");
  });
});
