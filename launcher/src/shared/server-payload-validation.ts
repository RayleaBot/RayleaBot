import type {
  LauncherDiagnosticIssue,
  LauncherReadinessSnapshot,
  LauncherSystemStatusSnapshot,
  LivenessStatusResponse,
  RecoveryCompatibilitySummary,
} from "./launcher-models";

type JsonObject = Record<string, unknown>;
type RecoveryIssue = NonNullable<RecoveryCompatibilitySummary["issues"]>[number];
type RecoverySkippedPlugin = NonNullable<RecoveryCompatibilitySummary["skipped_plugins"]>[number];
type RecoveryAuditEntry = NonNullable<RecoveryCompatibilitySummary["audit"]>[number];
type RecoveryAuditItem = RecoveryAuditEntry["items"][number];

function invalid(field: string): never {
  throw new Error(`Invalid server payload field: ${field}`);
}

function expectObject(value: unknown, field: string): JsonObject {
  if (value === null || typeof value !== "object" || Array.isArray(value)) {
    return invalid(field);
  }
  return value as JsonObject;
}

function expectAllowedKeys(value: JsonObject, allowed: readonly string[], field: string) {
  const allowedKeys = new Set(allowed);
  if (Object.keys(value).some((key) => !allowedKeys.has(key))) {
    invalid(field);
  }
}

function expectString(value: unknown, field: string): string {
  if (typeof value !== "string") {
    return invalid(field);
  }
  return value;
}

function expectBoolean(value: unknown, field: string): boolean {
  if (typeof value !== "boolean") {
    return invalid(field);
  }
  return value;
}

function expectNonNegativeInteger(value: unknown, field: string): number {
  if (typeof value !== "number" || !Number.isInteger(value) || value < 0) {
    return invalid(field);
  }
  return value;
}

function expectArray<T>(
  value: unknown,
  field: string,
  normalize: (item: unknown, field: string) => T,
): T[] {
  if (!Array.isArray(value)) {
    return invalid(field);
  }
  return value.map((item, index) => normalize(item, `${field}[${index}]`));
}

function expectStringArray(value: unknown, field: string): string[] {
  return expectArray(value, field, expectString);
}

function expectEnum<const Values extends readonly string[]>(
  value: unknown,
  allowed: Values,
  field: string,
): Values[number] {
  if (typeof value === "string" && (allowed as readonly string[]).includes(value)) {
    return value as Values[number];
  }
  return invalid(field);
}

function normalizeDiagnosticIssue(value: unknown, field: string): LauncherDiagnosticIssue {
  const object = expectObject(value, field);
  expectAllowedKeys(object, ["code", "severity", "summary", "user_message", "remediation", "internal_reason"], field);
  const result: LauncherDiagnosticIssue = {
    code: expectString(object.code, `${field}.code`),
    severity: expectEnum(object.severity, ["ok", "warning", "error"] as const, `${field}.severity`),
    summary: expectString(object.summary, `${field}.summary`),
  };
  if (object.user_message !== undefined) result.user_message = expectString(object.user_message, `${field}.user_message`);
  if (object.remediation !== undefined) result.remediation = expectString(object.remediation, `${field}.remediation`);
  if (object.internal_reason !== undefined) result.internal_reason = expectString(object.internal_reason, `${field}.internal_reason`);
  return result;
}

function normalizeRecoveryIssue(value: unknown, field: string): RecoveryIssue {
  const object = expectObject(value, field);
  expectAllowedKeys(object, ["code", "severity", "summary", "remediation"], field);
  const result: RecoveryIssue = {
    code: expectString(object.code, `${field}.code`),
    severity: expectEnum(object.severity, ["warning", "error"] as const, `${field}.severity`),
    summary: expectString(object.summary, `${field}.summary`),
  };
  if (object.remediation !== undefined) result.remediation = expectString(object.remediation, `${field}.remediation`);
  return result;
}

function normalizeRecoverySkippedPlugin(value: unknown, field: string): RecoverySkippedPlugin {
  const object = expectObject(value, field);
  expectAllowedKeys(object, [
    "plugin_id",
    "version",
    "reason_code",
    "summary",
    "review_id",
    "review_status",
    "reviewed_at",
    "reviewed_by",
    "manual_action",
    "manifest_path",
  ], field);
  const result: RecoverySkippedPlugin = {
    plugin_id: expectString(object.plugin_id, `${field}.plugin_id`),
    reason_code: expectString(object.reason_code, `${field}.reason_code`),
    summary: expectString(object.summary, `${field}.summary`),
    review_id: expectString(object.review_id, `${field}.review_id`),
    review_status: expectEnum(object.review_status, ["pending", "confirmed"] as const, `${field}.review_status`),
  };
  for (const key of ["version", "reviewed_at", "reviewed_by", "manual_action", "manifest_path"] as const) {
    if (object[key] !== undefined) result[key] = expectString(object[key], `${field}.${key}`);
  }
  return result;
}

function normalizeRecoveryAuditItem(value: unknown, field: string): RecoveryAuditItem {
  const object = expectObject(value, field);
  expectAllowedKeys(object, ["review_id", "plugin_id", "reason_code", "summary", "version"], field);
  const result: RecoveryAuditItem = {
    review_id: expectString(object.review_id, `${field}.review_id`),
    plugin_id: expectString(object.plugin_id, `${field}.plugin_id`),
    reason_code: expectString(object.reason_code, `${field}.reason_code`),
    summary: expectString(object.summary, `${field}.summary`),
  };
  if (object.version !== undefined) result.version = expectString(object.version, `${field}.version`);
  return result;
}

function normalizeRecoveryAuditEntry(value: unknown, field: string): RecoveryAuditEntry {
  const object = expectObject(value, field);
  expectAllowedKeys(object, ["task_id", "created_at", "operator_id", "note", "items"], field);
  return {
    task_id: expectString(object.task_id, `${field}.task_id`),
    created_at: expectString(object.created_at, `${field}.created_at`),
    operator_id: expectString(object.operator_id, `${field}.operator_id`),
    note: expectString(object.note, `${field}.note`),
    items: expectArray(object.items, `${field}.items`, normalizeRecoveryAuditItem),
  };
}

function normalizeRecoverySummary(value: unknown, field: string): RecoveryCompatibilitySummary {
  const object = expectObject(value, field);
  expectAllowedKeys(object, [
    "status",
    "phase",
    "operation",
    "created_at",
    "updated_at",
    "source_core_version",
    "target_core_version",
    "source_config_schema_version",
    "target_config_schema_version",
    "source_db_schema_version",
    "target_db_schema_version",
    "requires_post_start_checks",
    "issues",
    "skipped_plugins",
    "manual_actions",
    "next_steps",
    "audit",
  ], field);
  const result: RecoveryCompatibilitySummary = {
    status: expectEnum(object.status, ["pending", "compatible", "degraded", "blocked"] as const, `${field}.status`),
    phase: expectEnum(object.phase, ["pre_restore", "post_startup"] as const, `${field}.phase`),
    operation: expectEnum(object.operation, ["restore"] as const, `${field}.operation`),
    created_at: expectString(object.created_at, `${field}.created_at`),
    updated_at: expectString(object.updated_at, `${field}.updated_at`),
  };
  for (const key of [
    "source_core_version",
    "target_core_version",
    "source_config_schema_version",
    "target_config_schema_version",
    "source_db_schema_version",
    "target_db_schema_version",
  ] as const) {
    if (object[key] !== undefined) result[key] = expectString(object[key], `${field}.${key}`);
  }
  if (object.requires_post_start_checks !== undefined) {
    result.requires_post_start_checks = expectBoolean(object.requires_post_start_checks, `${field}.requires_post_start_checks`);
  }
  if (object.issues !== undefined) result.issues = expectArray(object.issues, `${field}.issues`, normalizeRecoveryIssue);
  if (object.skipped_plugins !== undefined) {
    result.skipped_plugins = expectArray(object.skipped_plugins, `${field}.skipped_plugins`, normalizeRecoverySkippedPlugin);
  }
  if (object.manual_actions !== undefined) result.manual_actions = expectStringArray(object.manual_actions, `${field}.manual_actions`);
  if (object.next_steps !== undefined) result.next_steps = expectStringArray(object.next_steps, `${field}.next_steps`);
  if (object.audit !== undefined) result.audit = expectArray(object.audit, `${field}.audit`, normalizeRecoveryAuditEntry);
  return result;
}

export function normalizeHealthPayload(value: unknown): LivenessStatusResponse | null {
  if (value === null || value === undefined) return null;
  const object = expectObject(value, "server.health");
  expectAllowedKeys(object, ["status"], "server.health");
  return { status: expectEnum(object.status, ["ok"] as const, "server.health.status") };
}

export function normalizeReadinessPayload(
  value: unknown,
  field = "server.readiness",
): LauncherReadinessSnapshot | null {
  if (value === null || value === undefined) return null;
  const object = expectObject(value, field);
  expectAllowedKeys(object, ["status", "reason", "reason_codes", "checks", "issues", "recovery_summary"], field);
  const result: LauncherReadinessSnapshot = {
    status: expectEnum(object.status, ["ready", "degraded", "setup_required", "failed"] as const, `${field}.status`),
  };
  if (object.reason !== undefined) result.reason = expectString(object.reason, `${field}.reason`);
  if (object.reason_codes !== undefined) result.reason_codes = expectStringArray(object.reason_codes, `${field}.reason_codes`);
  if (object.checks !== undefined) {
    const checks = expectObject(object.checks, `${field}.checks`);
    expectAllowedKeys(checks, ["config", "database", "runtime", "render"], `${field}.checks`);
    result.checks = {};
    for (const key of ["config", "database", "runtime", "render"] as const) {
      if (checks[key] !== undefined) result.checks[key] = expectString(checks[key], `${field}.checks.${key}`);
    }
  }
  if (object.issues !== undefined) result.issues = expectArray(object.issues, `${field}.issues`, normalizeDiagnosticIssue);
  if (object.recovery_summary !== undefined) {
    result.recovery_summary = normalizeRecoverySummary(object.recovery_summary, `${field}.recovery_summary`);
  }
  return result;
}

export function normalizeSystemStatusPayload(value: unknown): LauncherSystemStatusSnapshot | null {
  if (value === null || value === undefined) return null;
  const field = "server.systemStatus";
  const object = expectObject(value, field);
  expectAllowedKeys(object, [
    "status",
    "adapters",
    "active_plugins",
    "running_plugins",
    "failed_plugins",
    "db_schema_version",
    "uptime_seconds",
    "recovery_summary",
    "health",
  ], field);
  const result: LauncherSystemStatusSnapshot = {
    status: expectEnum(object.status, ["running", "shutting_down"] as const, `${field}.status`),
    adapters: expectArray(object.adapters, `${field}.adapters`, (value, path) => {
      const adapter = expectObject(value, path);
      expectAllowedKeys(adapter, ["id", "protocol", "enabled", "state"], path);
      const id = expectString(adapter.id, `${path}.id`);
      if (!id) invalid(`${path}.id`);
      return {
        id,
        protocol: expectEnum(adapter.protocol, ["onebot11", "qqofficial"] as const, `${path}.protocol`),
        enabled: expectBoolean(adapter.enabled, `${path}.enabled`),
        state: expectEnum(adapter.state, ["idle", "listening", "connecting", "connected", "auth_failed", "reconnecting", "stopped"] as const, `${path}.state`),
      };
    }),
  };
  for (const key of ["active_plugins", "running_plugins", "failed_plugins", "uptime_seconds"] as const) {
    if (object[key] !== undefined) result[key] = expectNonNegativeInteger(object[key], `${field}.${key}`);
  }
  if (object.db_schema_version !== undefined) {
    result.db_schema_version = expectString(object.db_schema_version, `${field}.db_schema_version`);
  }
  if (object.recovery_summary !== undefined) {
    result.recovery_summary = normalizeRecoverySummary(object.recovery_summary, `${field}.recovery_summary`);
  }
  if (object.health !== undefined) {
    const health = normalizeReadinessPayload(object.health, `${field}.health`);
    if (health === null) invalid(`${field}.health`);
    result.health = health;
  }
  return result;
}

export function normalizeRecoverySummaryPayload(value: unknown): RecoveryCompatibilitySummary | null {
  if (value === null || value === undefined) return null;
  return normalizeRecoverySummary(value, "launcher.localRecoverySummary");
}
