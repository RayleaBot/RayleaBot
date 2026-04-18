import type { components } from "./web-api.generated";

export type LauncherCloseBehavior = "ask_every_time" | "hide_to_tray" | "exit_application";
export type LauncherProcessLifecycle = "stopped" | "starting" | "running" | "stopping";
export type LauncherProcessOwnership = "none" | "launcher_managed" | "external";

export type ErrorEnvelope = components["schemas"]["ErrorEnvelope"];
export type LivenessStatusResponse = components["schemas"]["LivenessStatusResponse"];
export type LauncherAdmissionRequest = components["schemas"]["LauncherAdmissionRequest"];
export type LauncherDiagnosticIssue = components["schemas"]["DiagnosticIssue"];
export type LauncherReadinessSnapshot = components["schemas"]["ReadinessStatusResponse"];
export type LauncherSystemStatusSnapshot = components["schemas"]["SystemStatusResponse"];
export type LauncherTokenResponse = components["schemas"]["LauncherTokenResponse"];
export type RecoveryCompatibilityAuditEntry = components["schemas"]["RecoveryCompatibilityAuditEntry"];
export type RecoveryCompatibilityAuditItem = components["schemas"]["RecoveryCompatibilityAuditItem"];
export type RecoveryCompatibilityIssue = components["schemas"]["RecoveryCompatibilityIssue"];
export type RecoveryCompatibilitySkippedPlugin = components["schemas"]["RecoveryCompatibilitySkippedPlugin"];
export type RecoveryCompatibilitySummary = components["schemas"]["RecoveryCompatibilitySummary"];
export type TaskAcceptedResponse = components["schemas"]["TaskAcceptedResponse"];
export type TaskListResponse = components["schemas"]["TaskListResponse"];
export type TaskSummary = components["schemas"]["TaskSummary"];

export type CheckSeverity = LauncherDiagnosticIssue["severity"];
export type EnvironmentCheckScope = "preflight" | "advisory";

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
  scope: EnvironmentCheckScope;
  code: string;
  title: string;
  severity: CheckSeverity;
  summary: string;
  detail: string;
  remediation: string;
}

export interface EnvironmentInspection {
  checks: EnvironmentCheckResult[];
  preflightChecks: EnvironmentCheckResult[];
  advisoryChecks: EnvironmentCheckResult[];
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

export interface LauncherServerSnapshot {
  health: LivenessStatusResponse | null;
  readiness: LauncherReadinessSnapshot | null;
  systemStatus: LauncherSystemStatusSnapshot | null;
}

export interface LauncherLocalSnapshot {
  processId: number | null;
  processLifecycle: LauncherProcessLifecycle;
  processOwnership: LauncherProcessOwnership;
  environmentChecks: EnvironmentCheckResult[];
  preflightChecks: EnvironmentCheckResult[];
  advisoryChecks: EnvironmentCheckResult[];
  recentStderr: string[];
  releaseCheck: ReleaseCheckSnapshot;
  lastLocalError: string;
  statusHint: string;
  settings: LauncherSettings;
  resolvedSettings: LauncherResolvedSettings;
  endpoint: ServerEndpoint;
  localRecoverySummary: RecoveryCompatibilitySummary | null;
}

export interface LauncherSnapshot {
  server: LauncherServerSnapshot;
  launcher: LauncherLocalSnapshot;
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
