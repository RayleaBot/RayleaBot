import type { ConfigDocument } from '@/types/api'
import { t } from '@/i18n'

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
  description?: string
  fields: ConfigFieldDefinition[]
}

export function getConfigSections(): ConfigSectionDefinition[] {
  return [
    {
      key: 'server',
      title: t('config.sections.server'),
      fields: [
        { path: 'server.host', label: t('config.fields.serverHost'), type: 'text' },
        { path: 'server.port', label: t('config.fields.serverPort'), type: 'number' },
      ],
    },
    {
      key: 'onebot',
      title: t('config.sections.onebot'),
      fields: [
        {
          path: 'onebot.ws_url',
          label: t('config.fields.onebotWsUrl'),
          type: 'text',
          description: t('config.hints.onebotOptional'),
        },
        {
          path: 'onebot.access_token',
          label: t('config.fields.onebotAccessToken'),
          type: 'text',
          description: t('config.hints.redacted'),
        },
      ],
    },
    {
      key: 'database',
      title: t('config.sections.database'),
      fields: [
        { path: 'database.engine', label: t('config.fields.databaseEngine'), type: 'text' },
        { path: 'database.path', label: t('config.fields.databasePath'), type: 'text' },
      ],
    },
    {
      key: 'command',
      title: t('config.sections.command'),
      fields: [{ path: 'command.prefixes', label: t('config.fields.commandPrefixes'), type: 'list' }],
    },
    {
      key: 'admin',
      title: t('config.sections.admin'),
      fields: [
        { path: 'admin.super_admins', label: t('config.fields.adminSuperAdmins'), type: 'list' },
        { path: 'admin.session_ttl_days', label: t('config.fields.adminSessionTtlDays'), type: 'number' },
        { path: 'admin.sliding_renewal', label: t('config.fields.adminSlidingRenewal'), type: 'boolean' },
        { path: 'admin.max_sessions', label: t('config.fields.adminMaxSessions'), type: 'number' },
        { path: 'admin.login_fail_limit', label: t('config.fields.adminLoginFailLimit'), type: 'number' },
        { path: 'admin.login_fail_window_seconds', label: t('config.fields.adminLoginFailWindowSeconds'), type: 'number' },
      ],
    },
    {
      key: 'permission',
      title: t('config.sections.permission'),
      fields: [
        {
          path: 'permission.default_level',
          label: t('config.fields.permissionDefaultLevel'),
          type: 'select',
          options: [
            { label: t('config.options.permissionEveryone'), value: 'everyone' },
            { label: t('config.options.permissionGroupAdmin'), value: 'group_admin' },
            { label: t('config.options.permissionSuperAdmin'), value: 'super_admin' },
          ],
        },
        { path: 'permission.auto_grant_capabilities', label: t('config.fields.permissionAutoGrantCapabilities'), type: 'list' },
      ],
    },
    {
      key: 'render',
      title: t('config.sections.render'),
      fields: [
        { path: 'render.worker_count', label: t('config.fields.renderWorkerCount'), type: 'number' },
        { path: 'render.browser_args', label: t('config.fields.renderBrowserArgs'), type: 'list' },
        { path: 'render.browser_path', label: t('config.fields.renderBrowserPath'), type: 'text' },
        { path: 'render.timeout_seconds', label: t('config.fields.renderTimeoutSeconds'), type: 'number' },
        { path: 'render.queue_wait_timeout_seconds', label: t('config.fields.renderQueueWaitTimeoutSeconds'), type: 'number' },
        { path: 'render.queue_max_length', label: t('config.fields.renderQueueMaxLength'), type: 'number' },
      ],
    },
    {
      key: 'scheduler',
      title: t('config.sections.scheduler'),
      fields: [{ path: 'scheduler.timezone', label: t('config.fields.schedulerTimezone'), type: 'text' }],
    },
    {
      key: 'runtime',
      title: t('config.sections.runtime'),
      fields: [
        { path: 'runtime.plugin_init_timeout_seconds', label: t('config.fields.runtimePluginInitTimeoutSeconds'), type: 'number' },
        { path: 'runtime.plugin_init_max_total_seconds', label: t('config.fields.runtimePluginInitMaxTotalSeconds'), type: 'number' },
        { path: 'runtime.plugin_event_timeout_seconds', label: t('config.fields.runtimePluginEventTimeoutSeconds'), type: 'number' },
        { path: 'runtime.max_pending_events_per_plugin', label: t('config.fields.runtimeMaxPendingEventsPerPlugin'), type: 'number' },
        { path: 'runtime.max_pending_control_events_per_plugin', label: t('config.fields.runtimeMaxPendingControlEventsPerPlugin'), type: 'number' },
        { path: 'runtime.nodejs_max_old_space_size_mb', label: t('config.fields.runtimeNodejsMaxOldSpaceSizeMb'), type: 'number' },
        { path: 'runtime.dependency_install_timeout_seconds', label: t('config.fields.runtimeDependencyInstallTimeoutSeconds'), type: 'number' },
        { path: 'runtime.max_concurrent_dependency_installs', label: t('config.fields.runtimeMaxConcurrentDependencyInstalls'), type: 'number' },
        { path: 'runtime.ipc_pending_actions_max', label: t('config.fields.runtimeIpcPendingActionsMax'), type: 'number' },
        { path: 'runtime.ipc_action_burst_limit', label: t('config.fields.runtimeIpcActionBurstLimit'), type: 'text' },
        { path: 'runtime.stderr_rate_limit_bytes_per_second', label: t('config.fields.runtimeStderrRateLimitBytesPerSecond'), type: 'number' },
        { path: 'runtime.max_concurrent_tasks_per_plugin', label: t('config.fields.runtimeMaxConcurrentTasksPerPlugin'), type: 'number' },
        { path: 'runtime.crash_backoff_initial_seconds', label: t('config.fields.runtimeCrashBackoffInitialSeconds'), type: 'number' },
        { path: 'runtime.crash_backoff_max_seconds', label: t('config.fields.runtimeCrashBackoffMaxSeconds'), type: 'number' },
        { path: 'runtime.shutdown_grace_seconds', label: t('config.fields.runtimeShutdownGraceSeconds'), type: 'number' },
        { path: 'runtime.ipc_message_max_bytes', label: t('config.fields.runtimeIpcMessageMaxBytes'), type: 'number' },
      ],
    },
    {
      key: 'storage',
      title: t('config.sections.storage'),
      fields: [
        { path: 'storage.kv_value_max_bytes', label: t('config.fields.storageKvValueMaxBytes'), type: 'number' },
        { path: 'storage.kv_total_limit_mb', label: t('config.fields.storageKvTotalLimitMb'), type: 'number' },
        { path: 'storage.file_max_bytes', label: t('config.fields.storageFileMaxBytes'), type: 'number' },
        { path: 'storage.plugin_workdir_soft_limit_mb', label: t('config.fields.storagePluginWorkdirSoftLimitMb'), type: 'number' },
      ],
    },
    {
      key: 'data',
      title: t('config.sections.data'),
      fields: [
        { path: 'data.audit_logs_retention_days', label: t('config.fields.dataAuditLogsRetentionDays'), type: 'number' },
        { path: 'data.event_records_retention_days', label: t('config.fields.dataEventRecordsRetentionDays'), type: 'number' },
        { path: 'data.download_cache_retention_days', label: t('config.fields.dataDownloadCacheRetentionDays'), type: 'number' },
      ],
    },
    {
      key: 'log',
      title: t('config.sections.log'),
      fields: [
        {
          path: 'log.level',
          label: t('config.fields.logLevel'),
          type: 'select',
          options: [
            { label: t('config.options.logDebug'), value: 'debug' },
            { label: t('config.options.logInfo'), value: 'info' },
            { label: t('config.options.logWarn'), value: 'warn' },
            { label: t('config.options.logError'), value: 'error' },
          ],
        },
        { path: 'log.retention_days', label: t('config.fields.logRetentionDays'), type: 'number' },
        { path: 'log.rate_limit_per_plugin', label: t('config.fields.logRateLimitPerPlugin'), type: 'text' },
      ],
    },
    {
      key: 'message',
      title: t('config.sections.message'),
      fields: [
        { path: 'message.rate_limit_per_plugin', label: t('config.fields.messageRateLimitPerPlugin'), type: 'text' },
        { path: 'message.rate_limit_per_target', label: t('config.fields.messageRateLimitPerTarget'), type: 'text' },
        { path: 'message.circuit_breaker_seconds', label: t('config.fields.messageCircuitBreakerSeconds'), type: 'number' },
      ],
    },
    {
      key: 'user',
      title: t('config.sections.user'),
      fields: [
        { path: 'user.command_rate_limit', label: t('config.fields.userCommandRateLimit'), type: 'text' },
        { path: 'user.cooldown_reply', label: t('config.fields.userCooldownReply'), type: 'boolean' },
      ],
    },
    {
      key: 'group',
      title: t('config.sections.group'),
      fields: [{ path: 'group.command_rate_limit', label: t('config.fields.groupCommandRateLimit'), type: 'text' }],
    },
    {
      key: 'adapter',
      title: t('config.sections.adapter'),
      fields: [
        { path: 'adapter.connect_timeout_seconds', label: t('config.fields.adapterConnectTimeoutSeconds'), type: 'number' },
        { path: 'adapter.reconnect_initial_seconds', label: t('config.fields.adapterReconnectInitialSeconds'), type: 'number' },
        { path: 'adapter.reconnect_multiplier', label: t('config.fields.adapterReconnectMultiplier'), type: 'number' },
        { path: 'adapter.reconnect_max_seconds', label: t('config.fields.adapterReconnectMaxSeconds'), type: 'number' },
        { path: 'adapter.reconnect_jitter_ratio', label: t('config.fields.adapterReconnectJitterRatio'), type: 'number' },
      ],
    },
    {
      key: 'http',
      title: t('config.sections.http'),
      fields: [
        { path: 'http.timeout_seconds', label: t('config.fields.httpTimeoutSeconds'), type: 'number' },
        { path: 'http.max_retries', label: t('config.fields.httpMaxRetries'), type: 'number' },
        { path: 'http.allow_private_hosts', label: t('config.fields.httpAllowPrivateHosts'), type: 'list' },
      ],
    },
    {
      key: 'web',
      title: t('config.sections.web'),
      fields: [
        {
          path: 'web.exposure_mode',
          label: t('config.fields.webExposureMode'),
          type: 'select',
          options: [
            { label: t('config.options.webExposureLocalhostOnly'), value: 'localhost_only' },
            { label: t('config.options.webExposureLanEnabled'), value: 'lan_enabled' },
            { label: t('config.options.webExposureReverseProxy'), value: 'public_via_reverse_proxy' },
          ],
        },
        { path: 'web.setup_local_only', label: t('config.fields.webSetupLocalOnly'), type: 'boolean' },
      ],
    },
    {
      key: 'backup',
      title: t('config.sections.backup'),
      fields: [
        {
          path: 'backup.default_consistency',
          label: t('config.fields.backupDefaultConsistency'),
          type: 'select',
          options: [
            { label: t('config.options.backupOffline'), value: 'offline' },
            { label: t('config.options.backupOnline'), value: 'online' },
          ],
        },
      ],
    },
  ]
}

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
