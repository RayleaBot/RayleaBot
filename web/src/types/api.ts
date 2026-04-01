export type LogLevel = 'debug' | 'info' | 'warn' | 'error'

export type ConnectionStatus =
  | 'disconnected'
  | 'connecting'
  | 'connected'
  | 'authenticated'
  | 'auth_failed'
  | 'reconnecting'

export type TaskStatus = 'pending' | 'running' | 'succeeded' | 'failed' | 'cancelled' | 'interrupted'

export type TaskType =
  | 'plugin.install'
  | 'plugin.uninstall'
  | 'plugin.reload'
  | 'backup.create'
  | 'restore.apply'
  | 'config.migrate'
  | 'db.migrate'
  | 'render.preview'

export type PluginRegistrationState = 'installed' | 'removed'
export type PluginDesiredState = 'enabled' | 'disabled'
export type PluginRuntimeState = 'starting' | 'running' | 'stopping' | 'crashed' | 'backoff' | 'dead_letter' | 'stopped'
export type PluginRole = 'builtin' | 'user' | 'example' | 'dev'
export type PluginTrustLevel = 'official' | 'third_party' | 'unverified' | 'development'

export interface ErrorEnvelope {
  error: {
    code: string
    message: string
    message_key: string
    request_id: string
    details?: Record<string, unknown>
  }
}

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

export interface ReadinessStatusResponse {
  status: 'ready' | 'degraded' | 'setup_required' | 'failed'
  reason?: string
  reason_codes?: string[]
  checks?: Partial<Record<'config' | 'database' | 'runtime' | 'adapter' | 'render', string>>
  issues?: ReadinessIssue[]
}

export interface SystemStatusResponse {
  status: 'running' | 'shutting_down'
  adapter_state?: string
  active_plugins?: number
  uptime_seconds?: number
}

export interface SystemShutdownResponse {
  accepted: boolean
}

export interface LogSummary {
  timestamp: string
  level: LogLevel
  source: string
  message: string
  plugin_id?: string
  request_id?: string
}

export interface LogListResponse {
  items: LogSummary[]
}

export interface TaskResultSummary {
  summary: string
  details?: Record<string, unknown>
}

export interface TaskErrorSummary {
  code: string
  message: string
  details?: Record<string, unknown>
}

export interface TaskSummary {
  task_id: string
  task_type: TaskType
  status: TaskStatus
  progress?: number
  summary: string
  started_at?: string
  finished_at?: string
  result?: TaskResultSummary
  error?: TaskErrorSummary
}

export interface TaskListResponse {
  items: TaskSummary[]
}

export interface TaskDetailResponse {
  task: TaskSummary
}

export interface TaskAcceptedResponse {
  task_id: string
}

export interface RenderPreviewRequest {
  template: string
  theme?: string
  output?: 'png' | 'jpeg'
  data: Record<string, unknown>
}

export interface PluginSummary {
  id: string
  name: string
  role: PluginRole
  registration_state: PluginRegistrationState
  desired_state: PluginDesiredState
  runtime_state: PluginRuntimeState
  display_state?: string
  source?: PluginSourceSummary
  trust?: PluginTrustSummary
  command_conflicts?: string[]
}

export interface PluginListResponse {
  items: PluginSummary[]
}

export interface PluginDetailResponse {
  plugin: PluginSummary
}

export type PluginInstallSourceType = 'local_zip' | 'local_directory' | 'remote_url'

export interface PluginInstallRequest {
  source_type: PluginInstallSourceType
  source: string
  allow_install_scripts?: boolean
}

export interface PluginGrantRequest {
  capability: string
  expires_at?: string
}

export interface PluginGrantSummary {
  plugin_id: string
  capability: string
  granted_at: string
  expires_at?: string | null
}

export interface PluginGrantListResponse {
  items: PluginGrantSummary[]
}

export interface PluginSourceSummary {
  root: string
  package_source_type?: PluginInstallSourceType
  package_source_ref?: string
  verified: boolean
}

export interface PluginTrustSummary {
  level: PluginTrustLevel
  label: string
}

export interface ConfigDocument {
  schema_version: '2'
  server: {
    host: string
    port: number
  }
  onebot: {
    ws_url: string
    access_token: string
  }
  database: {
    engine: 'sqlite'
    path: string
  }
  command: {
    prefixes: string[]
  }
  admin: {
    super_admins: string[]
    session_ttl_days: number
    sliding_renewal: boolean
    max_sessions: number
    login_fail_limit: number
    login_fail_window_seconds: number
  }
  permission: {
    default_level: 'super_admin' | 'group_admin' | 'everyone'
    auto_grant_capabilities: string[]
  }
  render: {
    worker_count: number
    browser_args: string[]
    browser_path: string
    timeout_seconds: number
    queue_wait_timeout_seconds: number
    queue_max_length: number
  }
  scheduler: {
    timezone: string
  }
  runtime: {
    plugin_init_timeout_seconds: number
    plugin_init_max_total_seconds: number
    plugin_event_timeout_seconds: number
    max_pending_events_per_plugin: number
    max_pending_control_events_per_plugin: number
    nodejs_max_old_space_size_mb: number
    dependency_install_timeout_seconds: number
    max_concurrent_dependency_installs: number
    ipc_pending_actions_max: number
    ipc_action_burst_limit: string
    stderr_rate_limit_bytes_per_second: number
    max_concurrent_tasks_per_plugin: number
    crash_backoff_initial_seconds: number
    crash_backoff_max_seconds: number
    shutdown_grace_seconds: number
    ipc_message_max_bytes: number
  }
  storage: {
    kv_value_max_bytes: number
    kv_total_limit_mb: number
    file_max_bytes: number
    plugin_workdir_soft_limit_mb: number
  }
  data: {
    audit_logs_retention_days: number
    event_records_retention_days: number
    download_cache_retention_days: number
  }
  log: {
    level: LogLevel
    retention_days: number
    rate_limit_per_plugin: string
  }
  message: {
    rate_limit_per_plugin: string
    rate_limit_per_target: string
    circuit_breaker_seconds: number
  }
  user: {
    command_rate_limit: string
    cooldown_reply: boolean
  }
  group: {
    command_rate_limit: string
  }
  adapter: {
    connect_timeout_seconds: number
    reconnect_initial_seconds: number
    reconnect_multiplier: number
    reconnect_max_seconds: number
    reconnect_jitter_ratio: number
  }
  http: {
    timeout_seconds: number
    max_retries: number
    allow_private_hosts: string[]
  }
  web: {
    exposure_mode: 'localhost_only' | 'lan_enabled' | 'public_via_reverse_proxy'
    setup_local_only: boolean
  }
  backup: {
    default_consistency: 'offline' | 'online'
  }
}

export interface ConfigSnapshotResponse {
  config: ConfigDocument
  redacted_fields?: string[]
}

export interface ConfigUpdateResponse extends ConfigSnapshotResponse {
  restart_required: boolean
}

export type EventsPayload =
  | {
      service_status: 'setup_required' | 'stopped' | 'starting' | 'running' | 'degraded' | 'stopping' | 'failed'
      summary: string
      reason?: string
      reason_codes?: string[]
    }
  | {
      plugin_id: string
      registration_state: PluginRegistrationState
      desired_state: PluginDesiredState
      runtime_state: PluginRuntimeState
      display_state?: string
    }
  | {
      connection_status: ConnectionStatus
      summary: string
    }
  | {
      event_type: string
      summary: string
    }
  | {
      observability_scope: 'bridge_runtime'
      summary: string
      last_supported_event_kind?: string
      last_delivery_outcome?: 'delivered' | 'error'
      delivered_count: number
      result_count: number
      error_count: number
    }

export interface WebSocketFrame<T = Record<string, unknown>> {
  channel: 'logs' | 'events' | 'tasks' | 'plugin_console'
  type: string
  timestamp: string
  data: T
  request_id?: string
  error?: {
    code: string
    message?: string
    message_key: string
    details?: Record<string, unknown>
  }
}

export interface SessionExpiredFrame {
  type: 'session_expired'
  data: Record<string, never>
}
