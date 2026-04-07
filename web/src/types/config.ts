import type { LogLevel } from './common'

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
