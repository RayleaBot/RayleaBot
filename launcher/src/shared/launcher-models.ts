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

export interface RecoveryCompatibilityIssue {
  code: string;
  severity: "warning" | "error";
  summary: string;
  remediation?: string;
}

export interface RecoveryCompatibilitySkippedPlugin {
  plugin_id: string;
  version?: string;
  reason_code: string;
  summary: string;
  manual_action?: string;
  manifest_path?: string;
}

export interface RecoveryCompatibilitySummary {
  status: "pending" | "compatible" | "degraded" | "blocked";
  phase: "pre_restore" | "post_startup";
  operation: "restore" | "upgrade" | "rollback";
  created_at: string;
  updated_at: string;
  source_core_version?: string;
  target_core_version?: string;
  source_config_schema_version?: string;
  target_config_schema_version?: string;
  source_db_schema_version?: string;
  target_db_schema_version?: string;
  requires_post_start_checks?: boolean;
  issues?: RecoveryCompatibilityIssue[];
  skipped_plugins?: RecoveryCompatibilitySkippedPlugin[];
  manual_actions?: string[];
  next_steps?: string[];
}

export interface LauncherAdvancedOverrides {
  serverExecutablePath?: string;
  configPath?: string;
  workdir?: string;
}

export interface LauncherSettings {
  installationRoot: string;
  closeBehavior: LauncherCloseBehavior;
  advancedOverrides?: LauncherAdvancedOverrides;
}

export interface LauncherResolvedSettings {
  installationRoot: string;
  serverExecutablePath: string;
  configPath: string;
  workdir: string;
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
  resolvedSettings: LauncherResolvedSettings;
  endpoint: ServerEndpoint;
  environmentChecks: EnvironmentCheckResult[];
  recentStderr: string[];
  processId: number | null;
  serviceState: LauncherServiceState;
  shutdownRequested: boolean;
  serviceDetail: string;
  lastError: string;
  releaseCheck: ReleaseCheckSnapshot;
  recoverySummary?: RecoveryCompatibilitySummary | null;
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
