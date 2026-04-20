import type {
  LauncherReadinessSnapshot,
  RecoveryCompatibilitySummary,
  TaskSummary,
} from "@shared/launcher-models";
import type {
  EnvironmentCheckResult,
  EnvironmentInspection,
  ExternalOpener,
  LauncherManagementClient,
  LauncherOperationContext,
  LauncherResetAdminRunner,
  LauncherSettings,
  LauncherSettingsStore,
  RecoverySummaryReader,
  ReleaseFeedClient,
  ServerEndpoint,
  ServerEndpointResolver,
  ServerProcessController,
} from "@main/services/launcher-coordinator";

export class FakeSettingsStore implements LauncherSettingsStore {
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

export class FakeEndpointResolver implements ServerEndpointResolver {
  resolve(): ServerEndpoint {
    return { host: "127.0.0.1", port: 8080, baseUrl: "http://127.0.0.1:8080/" };
  }
}

export class FakeManagementClient implements LauncherManagementClient {
  health = true;
  setupInitialized = true;
  launcherToken = "launcher_fixture_token";
  sessionToken = "session_fixture_token";
  issueLauncherTokenCalls = 0;
  admitLauncherTokenCalls = 0;
  systemStatusCalls = 0;
  readiness: LauncherReadinessSnapshot = {
    status: "ready",
  };
  systemStatus = {
    status: "running",
    recovery_summary: null as RecoveryCompatibilitySummary | null,
  };
  recoverySummary: RecoveryCompatibilitySummary | null = null;
  inProgressTask: TaskSummary | null = null;

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

  async findInProgressTask() {
    return this.inProgressTask;
  }

  async createRecoveryRecheck() {
    return { task_id: "task_recovery_recheck_0001" };
  }

  async createRuntimeBootstrap() {
    return { task_id: "task_runtime_bootstrap_0001" };
  }

  async shutdown() {}
}

export class FakeRecoverySummaryReader implements RecoverySummaryReader {
  summary: RecoveryCompatibilitySummary | null = null;

  async read() {
    return this.summary;
  }
}

export class FakeProcessController implements ServerProcessController {
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

export class FakeExternalOpener implements ExternalOpener {
  openedUris: string[] = [];
  openedDirectories: string[] = [];

  async openUri(uri: string) {
    this.openedUris.push(uri);
  }

  async openDirectory(directoryPath: string) {
    this.openedDirectories.push(directoryPath);
  }
}

export class FakeReleaseFeedClient implements ReleaseFeedClient {
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

export class FakeResetAdminRunner implements LauncherResetAdminRunner {
  calls = 0;

  async run() {
    this.calls += 1;
  }
}

export function okInspection(overrides: Partial<EnvironmentInspection> = {}): EnvironmentInspection {
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

export const defaultOperationContext: LauncherOperationContext = {
  settings: {
    installationRoot: "C:\\RayleaBot",
    closeBehavior: "ask_every_time",
  },
  resolvedSettings: {
    installationRoot: "C:\\RayleaBot",
    serverExecutablePath: "C:\\RayleaBot\\raylea-server.exe",
    configPath: "C:\\RayleaBot\\config\\user.yaml",
    workdir: "C:\\RayleaBot",
  },
  endpoint: { host: "127.0.0.1", port: 8080, baseUrl: "http://127.0.0.1:8080/" },
};
