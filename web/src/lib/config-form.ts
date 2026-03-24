import type { ConfigDocument } from '@/types/api'

export interface ConfigFieldOption {
  label: string
  value: string | boolean
}

export interface ConfigFieldDefinition {
  path: string
  label: string
  type: 'text' | 'number' | 'boolean' | 'select' | 'list'
  description?: string
  options?: ConfigFieldOption[]
}

export interface ConfigSectionDefinition {
  key: keyof ConfigDocument
  title: string
  description: string
  fields: ConfigFieldDefinition[]
}

export const configSections: ConfigSectionDefinition[] = [
  {
    key: 'server',
    title: 'Server',
    description: 'HTTP 监听地址与端口。',
    fields: [
      { path: 'server.host', label: 'Host', type: 'text' },
      { path: 'server.port', label: 'Port', type: 'number' },
    ],
  },
  {
    key: 'onebot',
    title: 'OneBot',
    description: '反向 WebSocket 连接与鉴权。',
    fields: [
      { path: 'onebot.ws_url', label: 'WS URL', type: 'text' },
      { path: 'onebot.access_token', label: 'Access Token', type: 'text', description: '未改动时保留 __REDACTED__。' },
    ],
  },
  {
    key: 'database',
    title: 'Database',
    description: 'SQLite 主状态库。',
    fields: [
      { path: 'database.engine', label: 'Engine', type: 'text' },
      { path: 'database.path', label: 'Path', type: 'text' },
    ],
  },
  {
    key: 'command',
    title: 'Command',
    description: '聊天侧命令前缀。',
    fields: [{ path: 'command.prefixes', label: 'Prefixes', type: 'list' }],
  },
  {
    key: 'admin',
    title: 'Admin',
    description: '管理会话与登录限流。',
    fields: [
      { path: 'admin.super_admins', label: 'Super Admins', type: 'list' },
      { path: 'admin.session_ttl_days', label: 'Session TTL Days', type: 'number' },
      { path: 'admin.sliding_renewal', label: 'Sliding Renewal', type: 'boolean' },
      { path: 'admin.max_sessions', label: 'Max Sessions', type: 'number' },
      { path: 'admin.login_fail_limit', label: 'Login Fail Limit', type: 'number' },
      { path: 'admin.login_fail_window_seconds', label: 'Login Fail Window Seconds', type: 'number' },
    ],
  },
  {
    key: 'permission',
    title: 'Permission',
    description: '管理会话、自动授权和聊天侧默认权限。',
    fields: [
      {
        path: 'permission.default_level',
        label: 'Default Level',
        type: 'select',
        options: [
          { label: 'Everyone', value: 'everyone' },
          { label: 'Group Admin', value: 'group_admin' },
          { label: 'Super Admin', value: 'super_admin' },
        ],
      },
      { path: 'permission.auto_grant_capabilities', label: 'Auto Grant Capabilities', type: 'list' },
    ],
  },
  {
    key: 'render',
    title: 'Render',
    description: 'Render service 保留配置。',
    fields: [
      { path: 'render.worker_count', label: 'Worker Count', type: 'number' },
      { path: 'render.browser_args', label: 'Browser Args', type: 'list' },
      { path: 'render.browser_path', label: 'Browser Path', type: 'text' },
      { path: 'render.timeout_seconds', label: 'Timeout Seconds', type: 'number' },
      { path: 'render.queue_wait_timeout_seconds', label: 'Queue Wait Timeout', type: 'number' },
      { path: 'render.queue_max_length', label: 'Queue Max Length', type: 'number' },
    ],
  },
  {
    key: 'scheduler',
    title: 'Scheduler',
    description: '平台调度器时区。',
    fields: [{ path: 'scheduler.timezone', label: 'Timezone', type: 'text' }],
  },
  {
    key: 'runtime',
    title: 'Runtime',
    description: '插件运行时、队列和 IPC 约束。',
    fields: [
      { path: 'runtime.plugin_init_timeout_seconds', label: 'Init Timeout', type: 'number' },
      { path: 'runtime.plugin_init_max_total_seconds', label: 'Init Max Total', type: 'number' },
      { path: 'runtime.plugin_event_timeout_seconds', label: 'Event Timeout', type: 'number' },
      { path: 'runtime.max_pending_events_per_plugin', label: 'Pending Events', type: 'number' },
      { path: 'runtime.max_pending_control_events_per_plugin', label: 'Pending Control Events', type: 'number' },
      { path: 'runtime.nodejs_max_old_space_size_mb', label: 'Node.js Old Space MB', type: 'number' },
      { path: 'runtime.dependency_install_timeout_seconds', label: 'Dependency Install Timeout', type: 'number' },
      { path: 'runtime.max_concurrent_dependency_installs', label: 'Concurrent Dependency Installs', type: 'number' },
      { path: 'runtime.ipc_pending_actions_max', label: 'IPC Pending Actions Max', type: 'number' },
      { path: 'runtime.ipc_action_burst_limit', label: 'IPC Action Burst Limit', type: 'text' },
      { path: 'runtime.stderr_rate_limit_bytes_per_second', label: 'Stderr Rate Limit', type: 'number' },
      { path: 'runtime.max_concurrent_tasks_per_plugin', label: 'Concurrent Tasks Per Plugin', type: 'number' },
      { path: 'runtime.crash_backoff_initial_seconds', label: 'Crash Backoff Initial', type: 'number' },
      { path: 'runtime.crash_backoff_max_seconds', label: 'Crash Backoff Max', type: 'number' },
      { path: 'runtime.shutdown_grace_seconds', label: 'Shutdown Grace Seconds', type: 'number' },
      { path: 'runtime.ipc_message_max_bytes', label: 'IPC Message Max Bytes', type: 'number' },
    ],
  },
  {
    key: 'storage',
    title: 'Storage',
    description: '插件本地 KV 与文件区限制。',
    fields: [
      { path: 'storage.kv_value_max_bytes', label: 'KV Value Max Bytes', type: 'number' },
      { path: 'storage.kv_total_limit_mb', label: 'KV Total Limit MB', type: 'number' },
      { path: 'storage.file_max_bytes', label: 'File Max Bytes', type: 'number' },
      { path: 'storage.plugin_workdir_soft_limit_mb', label: 'Plugin Workdir Soft Limit MB', type: 'number' },
    ],
  },
  {
    key: 'data',
    title: 'Data',
    description: '短期运营数据保留窗口。',
    fields: [
      { path: 'data.audit_logs_retention_days', label: 'Audit Logs Days', type: 'number' },
      { path: 'data.event_records_retention_days', label: 'Event Records Days', type: 'number' },
      { path: 'data.download_cache_retention_days', label: 'Download Cache Days', type: 'number' },
    ],
  },
  {
    key: 'log',
    title: 'Log',
    description: '管理日志等级、保留与插件日志预算。',
    fields: [
      {
        path: 'log.level',
        label: 'Level',
        type: 'select',
        options: [
          { label: 'Debug', value: 'debug' },
          { label: 'Info', value: 'info' },
          { label: 'Warn', value: 'warn' },
          { label: 'Error', value: 'error' },
        ],
      },
      { path: 'log.retention_days', label: 'Retention Days', type: 'number' },
      { path: 'log.rate_limit_per_plugin', label: 'Rate Limit', type: 'text' },
    ],
  },
  {
    key: 'message',
    title: 'Message',
    description: '消息发送限流与熔断。',
    fields: [
      { path: 'message.rate_limit_per_plugin', label: 'Plugin Rate Limit', type: 'text' },
      { path: 'message.rate_limit_per_target', label: 'Target Rate Limit', type: 'text' },
      { path: 'message.circuit_breaker_seconds', label: 'Circuit Breaker Seconds', type: 'number' },
    ],
  },
  {
    key: 'user',
    title: 'User',
    description: '用户级命令冷却反馈。',
    fields: [
      { path: 'user.command_rate_limit', label: 'Command Rate Limit', type: 'text' },
      { path: 'user.cooldown_reply', label: 'Cooldown Reply', type: 'boolean' },
    ],
  },
  {
    key: 'group',
    title: 'Group',
    description: '群级命令冷却反馈。',
    fields: [{ path: 'group.command_rate_limit', label: 'Command Rate Limit', type: 'text' }],
  },
  {
    key: 'adapter',
    title: 'Adapter',
    description: '适配器连接与重连策略。',
    fields: [
      { path: 'adapter.connect_timeout_seconds', label: 'Connect Timeout', type: 'number' },
      { path: 'adapter.reconnect_initial_seconds', label: 'Reconnect Initial', type: 'number' },
      { path: 'adapter.reconnect_multiplier', label: 'Reconnect Multiplier', type: 'number' },
      { path: 'adapter.reconnect_max_seconds', label: 'Reconnect Max', type: 'number' },
      { path: 'adapter.reconnect_jitter_ratio', label: 'Reconnect Jitter', type: 'number' },
    ],
  },
  {
    key: 'http',
    title: 'HTTP',
    description: '插件本地 http.request 限制。',
    fields: [
      { path: 'http.timeout_seconds', label: 'Timeout Seconds', type: 'number' },
      { path: 'http.max_retries', label: 'Max Retries', type: 'number' },
      { path: 'http.allow_private_hosts', label: 'Allow Private Hosts', type: 'list' },
    ],
  },
  {
    key: 'web',
    title: 'Web',
    description: '管理面暴露模式。',
    fields: [
      {
        path: 'web.exposure_mode',
        label: 'Exposure Mode',
        type: 'select',
        options: [
          { label: 'Localhost Only', value: 'localhost_only' },
          { label: 'LAN Enabled', value: 'lan_enabled' },
          { label: 'Reverse Proxy', value: 'public_via_reverse_proxy' },
        ],
      },
      { path: 'web.setup_local_only', label: 'Setup Local Only', type: 'boolean' },
    ],
  },
  {
    key: 'backup',
    title: 'Backup',
    description: '备份默认一致性。',
    fields: [
      {
        path: 'backup.default_consistency',
        label: 'Default Consistency',
        type: 'select',
        options: [
          { label: 'Offline', value: 'offline' },
          { label: 'Online', value: 'online' },
        ],
      },
    ],
  },
]

export function cloneConfig(config: ConfigDocument) {
  return JSON.parse(JSON.stringify(config)) as ConfigDocument
}

export function getValueByPath(target: Record<string, unknown>, path: string): unknown {
  return path.split('.').reduce<unknown>((current, segment) => {
    if (current && typeof current === 'object') {
      return (current as Record<string, unknown>)[segment]
    }

    return undefined
  }, target)
}

export function setValueByPath(target: Record<string, unknown>, path: string, value: unknown) {
  const segments = path.split('.')
  const last = segments.pop()
  if (!last) {
    return
  }

  let cursor: Record<string, unknown> = target
  for (const segment of segments) {
    const next = cursor[segment]
    if (!next || typeof next !== 'object') {
      cursor[segment] = {}
    }
    cursor = cursor[segment] as Record<string, unknown>
  }

  cursor[last] = value
}
