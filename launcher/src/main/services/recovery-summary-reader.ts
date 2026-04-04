import fs from "node:fs/promises";
import path from "node:path";
import type {
  RecoveryCompatibilityAuditEntry,
  RecoveryCompatibilityAuditItem,
  RecoveryCompatibilityIssue,
  RecoveryCompatibilitySkippedPlugin,
  RecoveryCompatibilitySummary,
} from "../../shared/launcher-models";

const RECOVERY_SUMMARY_FILE = "recovery-summary.json";
const RECOVERY_STATUSES = new Set<RecoveryCompatibilitySummary["status"]>([
  "pending",
  "compatible",
  "degraded",
  "blocked",
]);
const RECOVERY_PHASES = new Set<RecoveryCompatibilitySummary["phase"]>([
  "pre_restore",
  "post_startup",
]);
const RECOVERY_OPERATIONS = new Set<RecoveryCompatibilitySummary["operation"]>([
  "restore",
  "upgrade",
  "rollback",
]);

function asObjectRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return null;
  }
  return value as Record<string, unknown>;
}

function readNonEmptyString(value: unknown) {
  if (typeof value !== "string") {
    return "";
  }
  return value.trim();
}

function readStringArray(value: unknown) {
  if (!Array.isArray(value)) {
    return undefined;
  }
  const items = value
    .map((item) => readNonEmptyString(item))
    .filter((item) => item.length > 0);
  return items.length > 0 ? items : undefined;
}

function parseIssues(value: unknown): RecoveryCompatibilitySummary["issues"] | undefined {
  if (!Array.isArray(value)) {
    return undefined;
  }
  const issues = value
    .map((item) => {
      const payload = asObjectRecord(item);
      if (!payload) {
        return null;
      }
      const code = readNonEmptyString(payload.code);
      const severity = readNonEmptyString(payload.severity);
      const summary = readNonEmptyString(payload.summary);
      if (!code || !summary || (severity !== "warning" && severity !== "error")) {
        return null;
      }
      const remediation = readNonEmptyString(payload.remediation);
      const issue: RecoveryCompatibilityIssue = remediation
        ? { code, severity, summary, remediation }
        : { code, severity, summary };
      return issue;
    })
    .filter((item): item is NonNullable<typeof item> => item !== null);
  return issues.length > 0 ? issues : undefined;
}

function parseSkippedPlugins(value: unknown): RecoveryCompatibilitySummary["skipped_plugins"] | undefined {
  if (!Array.isArray(value)) {
    return undefined;
  }
  const plugins = value
    .map((item) => {
      const payload = asObjectRecord(item);
      if (!payload) {
        return null;
      }
      const pluginId = readNonEmptyString(payload.plugin_id);
      const reasonCode = readNonEmptyString(payload.reason_code);
      const summary = readNonEmptyString(payload.summary);
      const reviewId = readNonEmptyString(payload.review_id);
      const reviewStatus = readNonEmptyString(payload.review_status);
      if (!pluginId || !reasonCode || !summary || !reviewId || (reviewStatus !== "pending" && reviewStatus !== "confirmed")) {
        return null;
      }
      const version = readNonEmptyString(payload.version);
      const reviewedAt = readNonEmptyString(payload.reviewed_at);
      const reviewedBy = readNonEmptyString(payload.reviewed_by);
      const manualAction = readNonEmptyString(payload.manual_action);
      const manifestPath = readNonEmptyString(payload.manifest_path);
      const plugin: RecoveryCompatibilitySkippedPlugin = {
        plugin_id: pluginId,
        ...(version ? { version } : {}),
        reason_code: reasonCode,
        summary,
        review_id: reviewId,
        review_status: reviewStatus as RecoveryCompatibilitySkippedPlugin["review_status"],
        ...(reviewedAt ? { reviewed_at: reviewedAt } : {}),
        ...(reviewedBy ? { reviewed_by: reviewedBy } : {}),
        ...(manualAction ? { manual_action: manualAction } : {}),
        ...(manifestPath ? { manifest_path: manifestPath } : {}),
      };
      return plugin;
    })
    .filter((item): item is NonNullable<typeof item> => item !== null);
  return plugins.length > 0 ? plugins : undefined;
}

function parseAuditItems(value: unknown): RecoveryCompatibilityAuditItem[] | undefined {
  if (!Array.isArray(value)) {
    return undefined;
  }
  const items = value
    .map((item) => {
      const payload = asObjectRecord(item);
      if (!payload) {
        return null;
      }
      const reviewId = readNonEmptyString(payload.review_id);
      const pluginId = readNonEmptyString(payload.plugin_id);
      const reasonCode = readNonEmptyString(payload.reason_code);
      const summary = readNonEmptyString(payload.summary);
      if (!reviewId || !pluginId || !reasonCode || !summary) {
        return null;
      }
      const version = readNonEmptyString(payload.version);
      const auditItem: RecoveryCompatibilityAuditItem = {
        review_id: reviewId,
        plugin_id: pluginId,
        reason_code: reasonCode,
        summary,
        ...(version ? { version } : {}),
      };
      return auditItem;
    })
    .filter((item): item is NonNullable<typeof item> => item !== null);
  return items.length > 0 ? items : undefined;
}

function parseAudit(value: unknown): RecoveryCompatibilitySummary["audit"] | undefined {
  if (!Array.isArray(value)) {
    return undefined;
  }
  const entries = value
    .map((item) => {
      const payload = asObjectRecord(item);
      if (!payload) {
        return null;
      }
      const taskId = readNonEmptyString(payload.task_id);
      const createdAt = readNonEmptyString(payload.created_at);
      const operatorId = readNonEmptyString(payload.operator_id);
      const rawNote = typeof payload.note === "string" ? payload.note.trim() : null;
      const items = parseAuditItems(payload.items);
      if (!taskId || !createdAt || !operatorId || rawNote === null || !items) {
        return null;
      }
      const entry: RecoveryCompatibilityAuditEntry = {
        task_id: taskId,
        created_at: createdAt,
        operator_id: operatorId,
        note: rawNote,
        items,
      };
      return entry;
    })
    .filter((item): item is NonNullable<typeof item> => item !== null);
  return entries.length > 0 ? entries : undefined;
}

function parseRecoverySummary(value: unknown): RecoveryCompatibilitySummary | null {
  const payload = asObjectRecord(value);
  if (!payload) {
    return null;
  }

  const status = readNonEmptyString(payload.status);
  const phase = readNonEmptyString(payload.phase);
  const operation = readNonEmptyString(payload.operation);
  const createdAt = readNonEmptyString(payload.created_at);
  const updatedAt = readNonEmptyString(payload.updated_at);

  if (
    !RECOVERY_STATUSES.has(status as RecoveryCompatibilitySummary["status"])
    || !RECOVERY_PHASES.has(phase as RecoveryCompatibilitySummary["phase"])
    || !RECOVERY_OPERATIONS.has(operation as RecoveryCompatibilitySummary["operation"])
    || !createdAt
    || !updatedAt
  ) {
    return null;
  }

  const summary: RecoveryCompatibilitySummary = {
    status: status as RecoveryCompatibilitySummary["status"],
    phase: phase as RecoveryCompatibilitySummary["phase"],
    operation: operation as RecoveryCompatibilitySummary["operation"],
    created_at: createdAt,
    updated_at: updatedAt,
  };

  if (typeof payload.requires_post_start_checks === "boolean") {
    summary.requires_post_start_checks = payload.requires_post_start_checks;
  }

  for (const key of [
    "source_core_version",
    "target_core_version",
    "source_config_schema_version",
    "target_config_schema_version",
    "source_db_schema_version",
    "target_db_schema_version",
  ] as const) {
    const value = readNonEmptyString(payload[key]);
    if (value) {
      summary[key] = value;
    }
  }

  const manualActions = readStringArray(payload.manual_actions);
  if (manualActions) {
    summary.manual_actions = manualActions;
  }

  const nextSteps = readStringArray(payload.next_steps);
  if (nextSteps) {
    summary.next_steps = nextSteps;
  }

  const issues = parseIssues(payload.issues);
  if (issues) {
    summary.issues = issues;
  }

  const skippedPlugins = parseSkippedPlugins(payload.skipped_plugins);
  if (skippedPlugins) {
    summary.skipped_plugins = skippedPlugins;
  }

  const audit = parseAudit(payload.audit);
  if (audit) {
    summary.audit = audit;
  }

  return summary;
}

export class NodeRecoverySummaryReader {
  async read(logDirectory: string): Promise<RecoveryCompatibilitySummary | null> {
    try {
      const payload = JSON.parse(await fs.readFile(path.join(logDirectory, RECOVERY_SUMMARY_FILE), "utf8"));
      return parseRecoverySummary(payload);
    } catch {
      return null;
    }
  }
}
