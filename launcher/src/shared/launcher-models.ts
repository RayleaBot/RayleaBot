export type LauncherCloseBehavior = "ask_every_time" | "hide_to_tray" | "exit_application";

export type LauncherServiceState =
  | "stopped"
  | "starting"
  | "external_service"
  | "ready"
  | "degraded"
  | "setup_required"
  | "shutting_down"
  | "failed";

export type CheckSeverity = "ok" | "warning" | "error";

export interface LauncherSettings {
  serverExecutablePath: string;
  configPath: string;
  workdir: string;
  closeBehavior: LauncherCloseBehavior;
}

export interface ServerEndpoint {
  host: string;
  port: number;
  baseUrl: string;
}

export interface EnvironmentCheckResult {
  code: string;
  title: string;
  severity: CheckSeverity;
  summary: string;
  detail: string;
  remediation: string;
}

export interface EnvironmentInspection {
  checks: EnvironmentCheckResult[];
  hasBlockingIssues: boolean;
  canBootstrapUserConfig: boolean;
}

export interface ReleaseCheckSnapshot {
  status: string;
  currentVersion: string;
  latestVersion: string;
  summary: string;
  detail: string;
  releasePageUrl: string;
  updateAvailable: boolean;
}

export interface LauncherSnapshot {
  settings: LauncherSettings;
  endpoint: ServerEndpoint;
  environmentChecks: EnvironmentCheckResult[];
  recentStderr: string[];
  processId: number | null;
  serviceState: LauncherServiceState;
  shutdownRequested: boolean;
  serviceDetail: string;
  lastError: string;
  releaseCheck: ReleaseCheckSnapshot;
}

export interface TrayMenuEntry {
  label: string;
  enabled: boolean;
  action: TrayMenuAction | "separator" | null;
}

export type TrayMenuAction = "restore" | "open_web" | "start" | "stop" | "open_logs" | "exit";

export interface TrayMenuState {
  trayStatusSummary: string;
  canOpenWebUi: boolean;
  trayServiceAction: "start" | "stop";
  trayServiceActionLabel: string;
  canRunTrayServiceAction: boolean;
}
