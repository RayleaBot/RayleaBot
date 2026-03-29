import { describe, expect, test, vi } from "vitest";
import {
  createLauncherCoordinator,
  type EnvironmentCheckResult,
  type EnvironmentInspection,
  type ExternalOpener,
  type LauncherManagementClient,
  type LauncherSettings,
  type LauncherSettingsStore,
  type ReleaseFeedClient,
  type ServerEndpoint,
  type ServerEndpointResolver,
  type ServerProcessController,
} from "@main/services/launcher-coordinator";

class FakeSettingsStore implements LauncherSettingsStore {
  settings: LauncherSettings = {
    serverExecutablePath: "C:\\RayleaBot\\raylea-server.exe",
    configPath: "C:\\RayleaBot\\config\\user.yaml",
    workdir: "C:\\RayleaBot",
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

  async shutdown() {}
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
  test("initialize reports external service without launcher session bootstrap", async () => {
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

    expect(coordinator.snapshot.serviceState).toBe("external_service");
    expect(managementClient.issueLauncherTokenCalls).toBe(0);
    expect(managementClient.admitLauncherTokenCalls).toBe(0);
    expect(coordinator.snapshot.releaseCheck.status).toBe("up_to_date");
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
    await coordinator.openWebUi();

    expect(externalOpener.openedUris.at(-1)).toContain("?token=");

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
    await coordinator.openWebUi();

    expect(externalOpener.openedUris.at(-1)).toBe("http://127.0.0.1:8080/");
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
    expect(coordinator.snapshot.serviceState).toBe("external_service");
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
});
