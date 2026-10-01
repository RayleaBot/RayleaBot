import type * as desktop from "../renderer/bindings/github.com/RayleaBot/RayleaBot/launcher/internal/desktop/models";

// Renderer objects use the string values of generated Go enums. Exclude Go's
// zero enum value, which is not a usable UI state.
type JsonModel<T> = T extends string ? Exclude<`${T}`, ""> : T extends object
  ? { [K in keyof T]: JsonModel<T[K]> } : T;
type PresentSlices<T> = { [K in keyof T]: NonNullable<T[K]> extends readonly unknown[] ? NonNullable<T[K]> : T[K] };

export type LauncherCloseBehavior = JsonModel<desktop.LauncherCloseBehavior>;
export type LauncherProcessLifecycle = JsonModel<desktop.LauncherProcessLifecycle>;
export type LauncherProcessOwnership = JsonModel<desktop.LauncherProcessOwnership>;
export type CheckSeverity = JsonModel<desktop.CheckSeverity>;
export type EnvironmentCheckScope = JsonModel<desktop.EnvironmentCheckScope>;
export type RuntimePrepareStatus = JsonModel<desktop.RuntimePrepareStatus>;
export type LauncherAdvancedOverrides = JsonModel<desktop.LauncherAdvancedOverrides>;
export type LauncherSettings = JsonModel<desktop.LauncherSettings>;
export type LauncherCloseConfirmResponse = JsonModel<desktop.LauncherCloseConfirmResponse>;
export type LauncherResolvedSettings = desktop.LauncherResolvedSettings;
export type ServerEndpoint = desktop.ServerEndpoint;
export type EnvironmentCheckResult = JsonModel<desktop.EnvironmentCheckResult>;
export type ReleaseCheckSnapshot = JsonModel<desktop.ReleaseCheckSnapshot>;
export type RuntimePrepareResourceProgress = JsonModel<desktop.RuntimePrepareResourceProgress>;
export type RuntimePrepareSnapshot = PresentSlices<JsonModel<desktop.RuntimePrepareSnapshot>>;

// Server responses decoded by the Go desktop layer, which drops unknown fields.
// These types list the values the renderer handles.
export type LivenessStatusResponse = { status: "ok" };
export type LauncherDiagnosticIssue = {
  code: string;
  severity: "ok" | "warning" | "error";
  summary: string;
  user_message?: string;
  remediation?: string;
  internal_reason?: string;
  runtime_resources?: ("chromium" | "ffmpeg")[];
};
export type LauncherReadinessSnapshot = {
  status: "ready" | "degraded" | "setup_required" | "failed";
  reason?: string;
  reason_codes?: string[];
  checks?: { database?: "ok" | "unavailable"; runtime?: "ok" | "preparing" | "resource_missing"; render?: "ok" | "resource_missing" };
  issues?: LauncherDiagnosticIssue[];
};
export type LauncherAdapterStatus = {
  id: string;
  protocol: "onebot11" | "qqofficial";
  enabled: boolean;
  state: "idle" | "listening" | "connecting" | "connected" | "auth_failed" | "reconnecting" | "stopped";
};
export type LauncherSystemStatusSnapshot = {
  status: "running" | "shutting_down";
  adapters: LauncherAdapterStatus[];
  active_plugins?: number;
  running_plugins?: number;
  failed_plugins?: number;
  db_schema_version?: string;
  uptime_seconds?: number;
  health?: LauncherReadinessSnapshot;
};

// Retain these types across Wails' broader pointer/enum representation.
export type LauncherServerSnapshot = Omit<desktop.LauncherServerSnapshot, "health" | "readiness" | "systemStatus"> & {
  health: LivenessStatusResponse | null;
  readiness: LauncherReadinessSnapshot | null;
  systemStatus: LauncherSystemStatusSnapshot | null;
};
export type LauncherLocalSnapshot = Omit<PresentSlices<JsonModel<desktop.LauncherLocalSnapshot>>, "runtimePrepare"> & {
  runtimePrepare: RuntimePrepareSnapshot | null;
};
export type LauncherSnapshot = Omit<desktop.LauncherSnapshot, "server" | "launcher"> & {
  server: LauncherServerSnapshot;
  launcher: LauncherLocalSnapshot;
};
