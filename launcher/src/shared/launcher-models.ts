import type * as desktop from "../renderer/bindings/github.com/RayleaBot/RayleaBot/launcher/internal/desktop/models";
import type { components } from "./web-api.generated";

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

export type LivenessStatusResponse = components["schemas"]["LivenessStatusResponse"];
export type LauncherDiagnosticIssue = components["schemas"]["DiagnosticIssue"];
export type LauncherReadinessSnapshot = components["schemas"]["ReadinessStatusResponse"];
export type LauncherSystemStatusSnapshot = components["schemas"]["SystemStatusResponse"];
export type RecoveryCompatibilitySummary = components["schemas"]["RecoveryCompatibilitySummary"];

// The host validates these server payloads against OpenAPI before publishing.
// Retain the formal HTTP types across Wails' broader pointer/enum representation.
export type LauncherServerSnapshot = Omit<desktop.LauncherServerSnapshot, "health" | "readiness" | "systemStatus"> & {
  health: LivenessStatusResponse | null;
  readiness: LauncherReadinessSnapshot | null;
  systemStatus: LauncherSystemStatusSnapshot | null;
};
export type LauncherLocalSnapshot = Omit<PresentSlices<JsonModel<desktop.LauncherLocalSnapshot>>, "runtimePrepare" | "localRecoverySummary"> & {
  runtimePrepare: RuntimePrepareSnapshot | null;
  localRecoverySummary: RecoveryCompatibilitySummary | null;
};
export type LauncherSnapshot = Omit<desktop.LauncherSnapshot, "server" | "launcher"> & {
  server: LauncherServerSnapshot;
  launcher: LauncherLocalSnapshot;
};
