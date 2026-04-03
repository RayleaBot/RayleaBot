import { describe, expect, test, vi } from "vitest";
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
  launcherToken = "launcher_fixture_token";
  sessionToken = "session_fixture_token";
  issueLauncherTokenCalls = 0;
  admitLauncherTokenCalls = 0;
  systemStatusCalls = 0;
  readiness = {
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

  async issueLauncherToken() {
    this.issueLauncherTokenCalls += 1;
    return this.launcherToken;
  }

  async admitLauncherToken() {
    this.admitLauncherTokenCalls += 1;
    return this.sessionToken;
  }

  async getReadiness() {
    return this.readiness;
  }

  async getSystemStatus() {
    this.systemStatusCalls += 1;
    return {
      ...this.systemStatus,
      recovery_summary: this.recoverySummary ?? this.systemStatus.recovery_summary,
    };
  }

  async createRecoveryRecheck() {
    return { task_id: "task_recovery_recheck_0001" };
  }

  async createRuntimeBootstrap() {
    return { task_id: "task_runtime_bootstrap_0001" };
  }

  async shutdown() {}
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
    return {
      status: "up_to_date",
      currentVersion: "0.1.0",
      latestVersion: "0.1.0",
      summary: "当前版本 0.1.0 已是最新。",
      detail: "",
      releasePageUrl: "https://example.invalid/releases/v0.1.0",
      updateAvailable: false,
    };
  }
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
      code: "server.executable",
      title: "服务端可执行文件",
      severity: "ok",
      summary: "已找到可执行文件。",
      detail: "ok",
      remediation: "",
    },
    {
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
    hasBlockingIssues: false,
    canBootstrapUserConfig: false,
    ...overrides,
  };
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

    expect(coordinator.snapshot.serviceState).toBe("running");
    expect((coordinator.snapshot as { serviceOwnership?: string }).serviceOwnership).toBe("external");
    expect(managementClient.issueLauncherTokenCalls).toBe(1);
    expect(managementClient.admitLauncherTokenCalls).toBe(1);
    expect(coordinator.snapshot.releaseCheck.status).toBe("up_to_date");
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

    expect(coordinator.snapshot.serviceState).toBe("running");
    expect((coordinator.snapshot as { serviceOwnership?: string }).serviceOwnership).toBe("launcher_managed");
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

    expect(coordinator.snapshot.endpoint.baseUrl).toBe("http://127.0.0.1:8080/");
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

    expect(coordinator.snapshot.recoverySummary?.status).toBe("degraded");
    expect(coordinator.snapshot.serviceState).toBe("running");
  });

  test("initialize reflects degraded readiness details from /readyz", async () => {
    const settingsStore = new FakeSettingsStore();
    const endpointResolver = new FakeEndpointResolver();
    const managementClient = new FakeManagementClient();
    managementClient.readiness = {
      status: "degraded",
      reason: "OneBot11 正在重连，管理面仍可使用。",
      reason_codes: ["adapter.reconnecting"],
      checks: {
        adapter: "warning",
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

    expect(coordinator.snapshot.serviceState).toBe("degraded");
    expect(coordinator.snapshot.serviceDetail).toContain("OneBot11 正在重连");
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

    expect(coordinator.snapshot.serviceState).toBe("stopping");
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
    managementClient.issueLauncherToken = vi.fn(async () => {
      throw new Error("service unavailable");
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
      recoverySummaryReader,
    });

    await coordinator.initialize();

    expect(coordinator.snapshot.recoverySummary?.status).toBe("blocked");
  });

  test("open web ui adds token only when setup is initialized", async () => {
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
    await coordinator.openWebUi("/tasks?task_id=task_fixture_0001");

    expect(externalOpener.openedUris.at(-1)).toContain("/tasks?task_id=task_fixture_0001");
    expect(externalOpener.openedUris.at(-1)).toContain("&token=");

    managementClient.setupInitialized = false;
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

  test("submits recovery tasks and opens the tasks page", async () => {
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
    await coordinator.createRecoveryRecheck();
    await coordinator.createRuntimeBootstrap(["chromium"]);

    expect(externalOpener.openedUris.at(-2)).toContain("/tasks?task_id=task_recovery_recheck_0001");
    expect(externalOpener.openedUris.at(-1)).toContain("/tasks?task_id=task_runtime_bootstrap_0001");
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
    expect(coordinator.snapshot.serviceState).toBe("running");
    expect((coordinator.snapshot as { serviceOwnership?: string }).serviceOwnership).toBe("external");
  });

  test("stop keeps an external service running when the confirmation is declined", async () => {
    const settingsStore = new FakeSettingsStore();
    const endpointResolver = new FakeEndpointResolver();
    const managementClient = new FakeManagementClient();
    const processController = new FakeProcessController();
    const tryStopEndpointProcess = vi.fn(async () => false);
    managementClient.shutdown = vi.fn(async () => undefined);

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

    expect(managementClient.shutdown).not.toHaveBeenCalled();
    expect(processController.forceKillCalls).toBe(0);
    expect(tryStopEndpointProcess).not.toHaveBeenCalled();
    expect(coordinator.snapshot.serviceState).toBe("running");
    expect((coordinator.snapshot as { serviceOwnership?: string }).serviceOwnership).toBe("external");
  });

  test("stop surfaces external shutdown admission failures without force killing the foreign process", async () => {
    const settingsStore = new FakeSettingsStore();
    const endpointResolver = new FakeEndpointResolver();
    const managementClient = new FakeManagementClient();
    const processController = new FakeProcessController();
    const tryStopEndpointProcess = vi.fn(async () => false);
    managementClient.issueLauncherToken = vi.fn(async () => {
      throw new Error("token issue failed");
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
    expect(coordinator.snapshot.serviceState).toBe("running");
    expect(coordinator.snapshot.lastError).toContain("token issue failed");
  });

  test("stop waits for the managed process to exit before reporting final state", async () => {
    const settingsStore = new FakeSettingsStore();
    const endpointResolver = new FakeEndpointResolver();
    const managementClient = new FakeManagementClient();
    const processController = new FakeProcessController();
    processController.isRunning = true;

    managementClient.health = true;
    managementClient.shutdown = vi.fn(async () => {
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
    expect(coordinator.snapshot.serviceState).toBe("stopped");
  });

  test("stop falls back to force kill when launcher session bootstrap fails", async () => {
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
    managementClient.issueLauncherToken = vi.fn(async () => {
      throw new Error("token issue failed");
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
    expect(coordinator.snapshot.serviceState).toBe("stopped");
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
    expect(coordinator.snapshot.serviceState).toBe("failed");
    expect(coordinator.snapshot.lastError).toContain("config validation failed");
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

    expect(coordinator.snapshot.serviceState).toBe("setup_required");
    expect(coordinator.snapshot.serviceDetail).toContain("管理员初始化");
  });

  test("initialize does not block on slow release checks", async () => {
    const settingsStore = new FakeSettingsStore();
    const endpointResolver = new FakeEndpointResolver();
    const managementClient = new FakeManagementClient();
    const releaseSnapshot = {
      status: "up_to_date",
      currentVersion: "0.1.0",
      latestVersion: "0.1.0",
      summary: "当前版本 0.1.0 已是最新。",
      detail: "",
      releasePageUrl: "https://example.invalid/releases/v0.1.0",
      updateAvailable: false,
    };
    let resolveRelease: ((value: typeof releaseSnapshot) => void) | null = null;
    const slowReleaseClient: ReleaseFeedClient = {
      getSnapshot: vi.fn(
        () =>
          new Promise((resolve) => {
            resolveRelease = resolve;
          }),
      ),
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
      new Promise<string>((resolve) => setTimeout(() => resolve("timeout"), 25)),
    ]);

    expect(result).toBe("resolved");
    expect(coordinator.snapshot.releaseCheck.status).toBe("unavailable");

    resolveRelease?.(releaseSnapshot);
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
    expect(coordinator.snapshot.serviceState).toBe("setup_required");
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
    expect(coordinator.snapshot.serviceState).toBe("failed");
    expect(coordinator.snapshot.lastError).toContain("spawn ENOENT");
    expect(coordinator.snapshot.serviceDetail).toContain("管理员凭据已重置");
  });
});
