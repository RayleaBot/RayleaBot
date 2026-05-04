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
} from "../../shared/launcher-models";

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
  getLauncherStatus(endpoint: ServerEndpoint): Promise<LauncherSystemStatusSnapshot>;
  shutdownFromLauncher(endpoint: ServerEndpoint): Promise<void>;
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

export interface LauncherCoordinatorOptions {
  startupTimeoutMs?: number;
  startupReadinessGraceMs?: number;
  pollIntervalMs?: number;
  shutdownTimeoutMs?: number;
  resetAdminTimeoutMs?: number;
  autoRefreshIntervalMs?: number;
}

export interface LauncherCoordinatorDependencies {
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
  openReleasePage(): Promise<void>;
  openLogsDirectory(): Promise<void>;
  saveSettings(settings: LauncherSettings): Promise<void>;
  subscribe(listener: (snapshot: LauncherSnapshot) => void): () => void;
}

export interface LauncherOperationContext {
  settings: LauncherSettings;
  resolvedSettings: LauncherResolvedSettings;
  endpoint: ServerEndpoint;
}

export interface LauncherRuntimeContext {
  getCurrentSettings(): LauncherSettings;
  initialize(): Promise<LauncherOperationContext>;
  createOperationContext(): Promise<LauncherOperationContext>;
  saveSettings(settings: LauncherSettings): Promise<LauncherOperationContext>;
}

export interface LocalSnapshotOverrides {
  processLifecycle?: LauncherProcessLifecycle;
  processOwnership?: LauncherProcessOwnership;
  lastLocalError?: string;
  statusHint?: string;
  localRecoverySummary?: RecoveryCompatibilitySummary | null;
}

export interface LauncherSnapshotStore {
  snapshot: LauncherSnapshot;
  reset(context: LauncherOperationContext): void;
  subscribe(listener: (snapshot: LauncherSnapshot) => void): () => void;
  buildSnapshot(
    context: LauncherOperationContext,
    inspection: EnvironmentInspection,
    server?: Partial<LauncherSnapshot["server"]>,
    launcherOverrides?: LocalSnapshotOverrides,
  ): LauncherSnapshot;
  publish(next: LauncherSnapshot): Promise<void>;
}

export interface LauncherStatusService {
  refresh(forceReauthentication: boolean): Promise<void>;
  buildSnapshotFromReadiness(
    context: LauncherOperationContext,
    inspection: EnvironmentInspection,
    readiness: LauncherReadinessSnapshot,
    forceReauthentication: boolean,
  ): Promise<LauncherSnapshot>;
}

export interface LauncherLifecycleService {
  start(): Promise<void>;
  stop(): Promise<void>;
  resetAdmin(): Promise<void>;
}

export interface LauncherDesktopActions {
  openWebUi(targetPath?: string): Promise<void>;
  openReleasePage(): Promise<void>;
  openLogsDirectory(): Promise<void>;
}
