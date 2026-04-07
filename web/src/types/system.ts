import type { PluginInstallSourceType } from './plugins'

export interface SetupStatusResponse {
  initialized: boolean
}

export interface SessionLoginRequest {
  identifier: string
  secret: string
}

export interface SessionLoginResponse {
  session_token: string
}

export interface LauncherTokenResponse {
  launcher_token: string
}

export interface LauncherAdmissionRequest {
  launcher_token: string
}

export interface LivenessStatusResponse {
  status: 'ok'
}

export interface ReadinessIssue {
  code: string
  severity: 'ok' | 'warning' | 'error'
  summary: string
  remediation?: string
}

export interface RecoveryCompatibilityIssue {
  code: string
  severity: 'warning' | 'error'
  summary: string
  remediation?: string
}

export interface RecoveryCompatibilitySkippedPlugin {
  plugin_id: string
  version?: string
  reason_code: string
  summary: string
  review_id: string
  review_status: 'pending' | 'confirmed'
  reviewed_at?: string
  reviewed_by?: string
  manual_action?: string
  manifest_path?: string
}

export interface RecoveryCompatibilityAuditItem {
  review_id: string
  plugin_id: string
  reason_code: string
  summary: string
  version?: string
}

export interface RecoveryCompatibilityAuditEntry {
  task_id: string
  created_at: string
  operator_id: string
  note: string
  items: RecoveryCompatibilityAuditItem[]
}

export interface RecoveryCompatibilitySummary {
  status: 'pending' | 'compatible' | 'degraded' | 'blocked'
  phase: 'pre_restore' | 'post_startup'
  operation: 'restore' | 'upgrade' | 'rollback'
  created_at: string
  updated_at: string
  source_core_version?: string
  target_core_version?: string
  source_config_schema_version?: string
  target_config_schema_version?: string
  source_db_schema_version?: string
  target_db_schema_version?: string
  requires_post_start_checks?: boolean
  issues?: RecoveryCompatibilityIssue[]
  skipped_plugins?: RecoveryCompatibilitySkippedPlugin[]
  manual_actions?: string[]
  next_steps?: string[]
  audit?: RecoveryCompatibilityAuditEntry[]
}

export interface ReadinessStatusResponse {
  status: 'ready' | 'degraded' | 'setup_required' | 'failed'
  reason?: string
  reason_codes?: string[]
  checks?: Partial<Record<'config' | 'database' | 'runtime' | 'adapter' | 'render', string>>
  issues?: ReadinessIssue[]
  recovery_summary?: RecoveryCompatibilitySummary
}

export interface SystemStatusResponse {
  status: 'running' | 'shutting_down'
  adapter_state?: string
  active_plugins?: number
  uptime_seconds?: number
  recovery_summary?: RecoveryCompatibilitySummary
}

export interface SystemShutdownResponse {
  accepted: boolean
}

export interface RecoveryConfirmRequest {
  review_ids: string[]
  note?: string
}

export type RuntimeBootstrapResource = 'chromium' | 'python-runtime' | 'nodejs-runtime'

export interface RuntimeBootstrapRequest {
  resources?: RuntimeBootstrapResource[]
}

export interface RenderPreviewRequest {
  template: string
  theme?: string
  output?: 'png' | 'jpeg'
  data: Record<string, unknown>
}
