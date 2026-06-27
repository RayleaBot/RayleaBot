import http from 'node:http'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { readFile } from 'node:fs/promises'

import YAML from 'yaml'
import { WebSocketServer } from 'ws'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const repoRoot = path.resolve(__dirname, '..', '..', '..')
const exampleConfigPanelRoot = path.join(repoRoot, 'examples', 'plugins', 'example-config-panel')
const externalPreviewImageBytes = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO2W4n8AAAAASUVORK5CYII=',
  'base64',
)
const bilibiliAvatarUrl = 'http://127.0.0.1:4010/external-preview/avatar.png'
const weiboAvatarUrl = 'https://tvax1.sinaimg.cn/crop.0.0.512.512.180/fixture.jpg'
const redactedConfigValue = '********'
const secretConfigPaths = [
  ['onebot', 'forward_ws', 'access_token'],
  ['onebot', 'http_api', 'access_token'],
  ['onebot', 'reverse_ws', 'access_token'],
  ['onebot', 'webhook', 'access_token'],
]
const externalPreviewFontBytes = await readFile(
  path.join(repoRoot, 'templates', 'fortune.card', 'assets', 'fonts', 'lxgwwenkai-medium', 'e8f52c41386b1b7731acfccb8c1a8c52.woff2'),
)

async function readFixture(relativePath) {
  const absolutePath = path.join(repoRoot, relativePath)
  const raw = await readFile(absolutePath, 'utf8')
  if (relativePath.endsWith('.json')) {
    return JSON.parse(raw)
  }
  return YAML.parse(raw)
}

const fixtures = {
  healthz: await readFixture('fixtures/web-api/ok.healthz-response.yaml'),
  readyz: await readFixture('fixtures/web-api/edge.readyz-degraded-response.yaml'),
  setupAdmin: await readFixture('fixtures/web-api/ok.setup-admin.yaml'),
  setupAdminDenied: await readFixture('fixtures/web-api/edge.setup-admin-already-initialized.yaml'),
  setupStatus: await readFixture('fixtures/web-api/ok.setup-status.yaml'),
  sessionLogin: await readFixture('fixtures/web-api/ok.session-login.yaml'),
  sessionDenied: await readFixture('fixtures/web-api/invalid.session-login-bad-credentials.yaml'),
  configGet: await readFixture('fixtures/web-api/ok.config-get-response.yaml'),
  protocolSnapshot: await readFixture('fixtures/web-api/ok.protocol-onebot11-snapshot.yaml'),
  protocolCompatibility: await readFixture('fixtures/web-api/ok.protocol-onebot11-compatibility.yaml'),
  logsList: await readFixture('fixtures/web-api/ok.logs-list-response.yaml'),
  logDetail: await readFixture('fixtures/web-api/ok.log-detail-response.yaml'),
  logDetailNotFound: await readFixture('fixtures/web-api/edge.log-detail-not-found.yaml'),
  systemStatus: await readFixture('fixtures/web-api/ok.system-status.yaml'),
  systemShutdown: await readFixture('fixtures/web-api/ok.system-shutdown.yaml'),
  systemBackupAccepted: await readFixture('fixtures/web-api/ok.system-backup-accepted.yaml'),
  renderTemplatesList: await readFixture('fixtures/web-api/ok.system-render-templates-list-response.yaml'),
  renderTemplateDetail: await readFixture('fixtures/web-api/ok.system-render-template-detail-response.yaml'),
  renderTemplateNotFound: await readFixture('fixtures/web-api/invalid.system-render-template-not-found.yaml'),
  schedulerJobsList: await readFixture('fixtures/web-api/ok.system-scheduler-jobs-list.yaml'),
  schedulerJobTriggered: await readFixture('fixtures/web-api/ok.system-scheduler-job-triggered.yaml'),
  systemDiagnosticsExport: await readFixture('fixtures/web-api/ok.system-diagnostics-export.yaml'),
  pluginEnable: await readFixture('fixtures/web-api/ok.plugins-enable-response.yaml'),
  pluginDisable: await readFixture('fixtures/web-api/edge.plugins-disable-response.yaml'),
  pluginReload: await readFixture('fixtures/web-api/ok.plugins-reload-response.yaml'),
  pluginInstallAccepted: await readFixture('fixtures/web-api/ok.plugins-install-accepted.yaml'),
  pluginInstallAcceptedWithScripts: await readFixture('fixtures/web-api/ok.plugins-install-accepted-with-scripts.yaml'),
  pluginInstallRemoteUrl: await readFixture('fixtures/web-api/ok.plugins-install-remote-url.yaml'),
  pluginList: await readFixture('fixtures/web-api/ok.plugins-list-response.yaml'),
  pluginDetail: await readFixture('fixtures/web-api/ok.plugin-detail-response.yaml'),
  pluginDetailManagementUI: await readFixture('fixtures/web-api/ok.plugin-detail-response.management-ui.yaml'),
  pluginSettings: await readFixture('fixtures/web-api/ok.plugin-settings-response.yaml'),
  pluginSettingsUpdate: await readFixture('fixtures/web-api/ok.plugin-settings-update-response.yaml'),
  pluginUninstallAccepted: await readFixture('fixtures/web-api/ok.plugins-uninstall-accepted.yaml'),
  invalidUninstallNotFound: await readFixture('fixtures/web-api/invalid.plugins-uninstall-not-found.yaml'),
  governanceBlacklist: await readFixture('fixtures/web-api/ok.governance-blacklist-response.yaml'),
  governanceBlacklistEntryUpsert: await readFixture('fixtures/web-api/ok.governance-blacklist-entry-upsert.yaml'),
  governanceWhitelist: await readFixture('fixtures/web-api/ok.governance-whitelist-response.yaml'),
  governanceWhitelistState: await readFixture('fixtures/web-api/ok.governance-whitelist-state-response.yaml'),
  governanceWhitelistEntryUpsert: await readFixture('fixtures/web-api/ok.governance-whitelist-entry-upsert.yaml'),
  governanceCommandPolicy: await readFixture('fixtures/web-api/ok.governance-command-policy-response.yaml'),
  thirdPartyAccounts: await readFixture('fixtures/web-api/ok.third-party-accounts-list.yaml'),
  thirdPartyAccountUpsert: await readFixture('fixtures/web-api/ok.third-party-account-upsert.yaml'),
  thirdPartyQRCodeCreateBilibili: await readFixture('fixtures/web-api/ok.third-party-login-qrcode-create-bilibili.yaml'),
  thirdPartyQRCodePollBilibiliPending: await readFixture('fixtures/web-api/ok.third-party-login-qrcode-poll-bilibili-pending.yaml'),
  thirdPartyQRCodePollBilibiliSucceeded: await readFixture('fixtures/web-api/ok.third-party-login-qrcode-poll-bilibili-succeeded.yaml'),
  thirdPartyQRCodeCreateWeibo: await readFixture('fixtures/web-api/ok.third-party-login-qrcode-create-weibo.yaml'),
  thirdPartyQRCodePollWeiboPending: await readFixture('fixtures/web-api/ok.third-party-login-qrcode-poll-weibo-pending.yaml'),
  thirdPartyQRCodePollWeiboSucceeded: await readFixture('fixtures/web-api/ok.third-party-login-qrcode-poll-weibo-succeeded.yaml'),
  thirdPartyQRCodeCreateDouyin: await readFixture('fixtures/web-api/ok.third-party-login-qrcode-create-douyin.yaml'),
  thirdPartyQRCodePollDouyinPending: await readFixture('fixtures/web-api/ok.third-party-login-qrcode-poll-douyin-pending.yaml'),
  thirdPartyQRCodePollDouyinSucceeded: await readFixture('fixtures/web-api/ok.third-party-login-qrcode-poll-douyin-succeeded.yaml'),
  thirdPartyQRCodeCreateNeteaseMusic: await readFixture('fixtures/web-api/ok.third-party-login-qrcode-create-netease-music.yaml'),
  thirdPartyQRCodePollNeteaseMusicPending: await readFixture('fixtures/web-api/ok.third-party-login-qrcode-poll-netease-music-pending.yaml'),
  thirdPartyQRCodePollNeteaseMusicSucceeded: await readFixture('fixtures/web-api/ok.third-party-login-qrcode-poll-netease-music-succeeded.yaml'),
  wsLogs: await readFixture('fixtures/websocket/ok.logs-appended.protocol-onebot11.json'),
  wsEvents: await readFixture('fixtures/websocket/edge.events-received-degraded.json'),
  wsEventsProtocolSnapshot: await readFixture('fixtures/websocket/ok.events-received-protocol-snapshot.json'),
  wsConsole: await readFixture('fixtures/websocket/ok.plugins-console-stderr.json'),
  wsSessionExpired: await readFixture('fixtures/websocket/edge.session-expired.json'),
}

const sockets = {
  events: new Set(),
  logs: new Set(),
  plugin_console: new Set(),
}

function baseState() {
  const initialLogs = structuredClone(fixtures.logsList.response.body.items)
  const pluginItems = structuredClone(fixtures.pluginList.response.body.items)
  const pluginMap = Object.fromEntries(pluginItems.map((item) => [item.id, item]))
  pluginMap.weather = structuredClone(fixtures.pluginDetail.response.body.plugin)
  pluginMap['example-config-panel'] = createExampleConfigPanelPlugin()
  const thirdPartyAccounts = structuredClone(fixtures.thirdPartyAccounts.response.body.items)
    .map(localizeBilibiliAccountAvatar)
  return {
    initialized: false,
    token: null,
    plugins: pluginMap,
    pluginSettings: {
      'example-config-panel': structuredClone(fixtures.pluginSettings.response.body.values),
    },
    logs: initialLogs,
    currentSessionLogIds: new Set(initialLogs.map((item) => item.log_id)),
    logDetails: createLogDetailMap(),
    config: structuredClone(fixtures.configGet.response.body.config),
    protocolSnapshot: structuredClone(fixtures.protocolSnapshot.response.body),
    governanceBlacklist: structuredClone(fixtures.governanceBlacklist.response.body),
    governanceWhitelist: structuredClone(fixtures.governanceWhitelist.response.body),
    governanceCommandPolicy: structuredClone(fixtures.governanceCommandPolicy.response.body),
    thirdPartyAccounts,
    thirdPartyQRCodePolls: {},
    renderTemplates: createRenderTemplateState(),
    schedulerJobs: structuredClone(fixtures.schedulerJobsList.response.body.items),
    systemStatus: structuredClone(fixtures.systemStatus.response.body),
    failures: {
      failPluginsListOnce: false,
      failPluginDetailOnce: false,
      failLogsOnce: false,
      failSystemStatusOnce: false,
      failUninstallOnce: false,
    },
    networkOffline: false,
  }
}

function computeProtocolSnapshotFromConfig(config, currentSnapshot) {
  const snapshot = structuredClone(currentSnapshot)
  const onebot = config.onebot ?? {}
  const reverseWs = onebot.reverse_ws ?? { enabled: false, url: '' }
  const forwardWs = onebot.forward_ws ?? { enabled: false, url: '' }
  const httpApi = onebot.http_api ?? { enabled: false, url: '' }
  const webhook = onebot.webhook ?? { enabled: false, url: '' }
  const transports = [
    ['reverse_ws', reverseWs],
    ['forward_ws', forwardWs],
    ['http_api', httpApi],
    ['webhook', webhook],
  ]

  snapshot.provider = 'unknown'
  snapshot.transport_status = transports.map(([transport, entry]) => {
    const configured = Boolean(entry.url)
    let state = 'idle'
    let summary = '未启用'

    if (entry.enabled && configured) {
      if (transport === 'forward_ws') {
        state = 'connected'
        summary = '主动连接已建立'
      } else if (transport === 'reverse_ws') {
        state = 'listening'
        summary = '等待 OneBot 回连'
      } else if (transport === 'http_api') {
        state = 'connected'
        summary = 'HTTP API 可用'
      } else if (transport === 'webhook') {
        state = 'listening'
        summary = 'Webhook 入口可接收上报'
      }
    }

    return {
      transport,
      enabled: Boolean(entry.enabled),
      configured,
      endpoint: entry.url ? entry.url.replace(/^(https?:\/\/[^/]+|wss?:\/\/[^/]+).*$/, '$1') : '',
      state,
      summary,
    }
  })
  snapshot.configured_transports = transports
    .filter(([, entry]) => Boolean(entry.url))
    .map(([name]) => name)

  if (forwardWs.enabled && forwardWs.url) {
    snapshot.active_transports = ['forward_ws']
    snapshot.readiness_status = 'ready'
    snapshot.summary = 'OneBot11 主动连接已就绪'
  } else if (reverseWs.enabled && reverseWs.url) {
    snapshot.active_transports = ['reverse_ws']
    snapshot.readiness_status = 'degraded'
    snapshot.summary = 'OneBot11 等待回连'
  } else if (httpApi.enabled && httpApi.url && webhook.enabled && webhook.url) {
    snapshot.active_transports = ['http_api', 'webhook']
    snapshot.readiness_status = 'ready'
    snapshot.summary = 'OneBot11 HTTP API 与 Webhook 已就绪'
  } else if (httpApi.enabled && httpApi.url) {
    snapshot.active_transports = ['http_api']
    snapshot.readiness_status = 'degraded'
    snapshot.summary = 'OneBot11 仅 HTTP API 可用'
  } else if (webhook.enabled && webhook.url) {
    snapshot.active_transports = ['webhook']
    snapshot.readiness_status = 'degraded'
    snapshot.summary = 'OneBot11 仅 Webhook 上报可用'
  } else {
    snapshot.active_transports = []
    snapshot.readiness_status = 'setup_required'
    snapshot.summary = 'OneBot11 尚未配置连接'
  }
  return snapshot
}

function redactConfigSecrets(config) {
  const snapshot = structuredClone(config)
  const redactedFields = []
  for (const secretPath of secretConfigPaths) {
    const value = getPath(snapshot, secretPath)
    if (typeof value !== 'string' || value.trim() === '') {
      continue
    }
    setPath(snapshot, secretPath, redactedConfigValue)
    redactedFields.push(secretPath.join('.'))
  }
  return {
    config: snapshot,
    redacted_fields: redactedFields.sort(),
  }
}

function restoreRedactedConfigSecrets(payload, currentConfig) {
  const nextConfig = structuredClone(payload)
  for (const secretPath of secretConfigPaths) {
    const submitted = getPath(nextConfig, secretPath)
    if (submitted !== undefined && String(submitted).trim() !== redactedConfigValue) {
      continue
    }
    setPath(nextConfig, secretPath, String(getPath(currentConfig, secretPath) ?? ''))
  }
  return nextConfig
}

function getPath(value, segments) {
  let current = value
  for (const segment of segments) {
    if (!current || typeof current !== 'object' || !(segment in current)) {
      return undefined
    }
    current = current[segment]
  }
  return current
}

function setPath(value, segments, nextValue) {
  let current = value
  for (const segment of segments.slice(0, -1)) {
    if (!current[segment] || typeof current[segment] !== 'object') {
      current[segment] = {}
    }
    current = current[segment]
  }
  current[segments.at(-1)] = nextValue
}

function normalizeTransport(entry = {}) {
  return {
    enabled: Boolean(entry.enabled),
    url: String(entry.url ?? ''),
    access_token: String(entry.access_token ?? ''),
  }
}

function pickOneBotHotState(config) {
  const onebot = config.onebot ?? {}
  const adapter = config.adapter ?? {}

  return {
    adapter: {
      connect_timeout_seconds: adapter.connect_timeout_seconds ?? 0,
      reconnect_initial_seconds: adapter.reconnect_initial_seconds ?? 0,
      reconnect_multiplier: adapter.reconnect_multiplier ?? 0,
      reconnect_max_seconds: adapter.reconnect_max_seconds ?? 0,
      reconnect_jitter_ratio: adapter.reconnect_jitter_ratio ?? 0,
    },
    onebot: {
      reverse_ws: normalizeTransport(onebot.reverse_ws),
      forward_ws: normalizeTransport(onebot.forward_ws),
      http_api: normalizeTransport(onebot.http_api),
      webhook: normalizeTransport(onebot.webhook),
    },
  }
}

function computeRestartRequiredForConfig(prevConfig, nextConfig) {
  return computeConfigApplyEffects(prevConfig, nextConfig).restart_required_fields.length > 0
}

function localizeBilibiliAccountAvatar(account) {
  if (account?.platform === 'bilibili' && account.profile) {
    account.profile.avatar_url = bilibiliAvatarUrl
  }
  return account
}

const thirdPartyAccountPlatforms = ['bilibili', 'weibo', 'douyin', 'netease_music']

function thirdPartyQRCodeFixtures(platform) {
  switch (platform) {
    case 'bilibili':
      return {
        create: fixtures.thirdPartyQRCodeCreateBilibili,
        pending: fixtures.thirdPartyQRCodePollBilibiliPending,
        succeeded: fixtures.thirdPartyQRCodePollBilibiliSucceeded,
      }
    case 'weibo':
      return {
        create: fixtures.thirdPartyQRCodeCreateWeibo,
        pending: fixtures.thirdPartyQRCodePollWeiboPending,
        succeeded: fixtures.thirdPartyQRCodePollWeiboSucceeded,
      }
    case 'douyin':
      return {
        create: fixtures.thirdPartyQRCodeCreateDouyin,
        pending: fixtures.thirdPartyQRCodePollDouyinPending,
        succeeded: fixtures.thirdPartyQRCodePollDouyinSucceeded,
      }
    case 'netease_music':
      return {
        create: fixtures.thirdPartyQRCodeCreateNeteaseMusic,
        pending: fixtures.thirdPartyQRCodePollNeteaseMusicPending,
        succeeded: fixtures.thirdPartyQRCodePollNeteaseMusicSucceeded,
      }
    default:
      return null
  }
}

function thirdPartyQRCodePollKey(platform, loginId) {
  return `${platform}:${loginId}`
}

function defaultCredentialStatus(platform) {
  if (platform === 'bilibili') {
    return structuredClone(fixtures.thirdPartyAccountUpsert.response.body.account.credential)
  }
  return {
    state: 'unknown',
    checked_at: new Date().toISOString(),
    last_error: '',
  }
}

function syncGovernanceCommandPolicyFromConfig(config) {
  if (!state?.governanceCommandPolicy) {
    return
  }

  state.governanceCommandPolicy.default_level = config.permission?.default_level ?? 'everyone'
  state.governanceCommandPolicy.cooldown = {
    user_command_rate_limit: config.user?.command_rate_limit ?? '10/60s',
    group_command_rate_limit: config.group?.command_rate_limit ?? '30/60s',
    cooldown_reply: Boolean(config.user?.cooldown_reply),
  }
}

function createRenderTemplateState() {
  const helpDetail = structuredClone(fixtures.renderTemplateDetail.response.body.template)
  const items = structuredClone(fixtures.renderTemplatesList.response.body.items)
  const byId = Object.fromEntries(items.map((item) => [
    item.id,
    {
      detail: {
        ...item,
        input_schema_json: item.id === helpDetail.id ? structuredClone(helpDetail.input_schema_json) : null,
      },
    },
  ]))

  return { byId }
}

function listRenderTemplates() {
  return {
    items: Object.values(state.renderTemplates.byId)
      .map((template) => ({
        id: template.detail.id,
        version: template.detail.version,
        width: template.detail.width,
        height: template.detail.height,
        has_input_schema: template.detail.has_input_schema,
        updated_at: template.detail.updated_at,
        source: structuredClone(template.detail.source),
      }))
      .sort((left, right) => right.updated_at.localeCompare(left.updated_at)),
  }
}

function getRenderTemplate(templateId) {
  return state.renderTemplates.byId[templateId] ?? null
}

function renderTemplateDetailBody(templateId) {
  const template = getRenderTemplate(templateId)
  return template ? { template: structuredClone(template.detail) } : null
}

function renderTemplatePreviewHTMLBody(templateId, payload = {}) {
  const template = getRenderTemplate(templateId)
  if (!template) {
    return null
  }
  const title = typeof payload.data?.title === 'string' && payload.data.title.trim()
    ? payload.data.title.trim()
    : template.detail.id
  const width = Number.isFinite(template.detail.width) && template.detail.width > 0
    ? Math.ceil(template.detail.width)
    : 960
  return {
    template_id: templateId,
    revision_id: `rev_${templateId.replaceAll('.', '_')}_e2e`,
    width: template.detail.width,
    height: template.detail.height,
    html: `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8" /><link rel="stylesheet" href="http://127.0.0.1:4010/external-preview/font.css" /><style>html,body{min-width:${width}px;margin:0}.surface{width:${width}px;min-height:360px;padding:24px;font-family:RayleaExternalPreview,sans-serif;background-image:url("http://127.0.0.1:4010/external-preview/background.png")}.external-preview-image{width:16px;height:16px}</style></head><body><main class="surface"><h1>${escapeHTML(title)}</h1><img class="external-preview-image" src="http://127.0.0.1:4010/external-preview/avatar.png" alt="外部图片"></main></body></html>`,
  }
}

function escapeHTML(value) {
  return String(value)
    .replaceAll('&', '&amp;')
    .replaceAll('<', '&lt;')
    .replaceAll('>', '&gt;')
    .replaceAll('"', '&quot;')
    .replaceAll("'", '&#39;')
}

const configRestartRequiredFields = new Set([
  'admin.max_sessions',
  'admin.session_ttl_days',
  'admin.sliding_renewal',
  'database.engine',
  'database.path',
  'render.browser_args',
  'render.browser_path',
  'render.worker_count',
  'server.host',
  'server.port',
  'web.exposure_mode',
  'web.setup_local_only',
])

function computeConfigApplyEffects(prevConfig, nextConfig) {
  const changedPaths = []
  collectChangedConfigPaths('', prevConfig ?? {}, nextConfig ?? {}, changedPaths)
  changedPaths.sort()

  const effects = {
    applied_now: [],
    reloaded_now: [],
    restart_required_fields: [],
  }

  for (const path of [...new Set(changedPaths)]) {
    if (path.startsWith('onebot.') || path.startsWith('adapter.')) {
      effects.reloaded_now.push(path)
    } else if (configRestartRequiredFields.has(path) || path.startsWith('database.') || path.startsWith('server.') || path.startsWith('web.')) {
      effects.restart_required_fields.push(path)
    } else {
      effects.applied_now.push(path)
    }
  }

  return effects
}

function collectChangedConfigPaths(prefix, prevValue, nextValue, changedPaths) {
  if (isPlainObject(prevValue) && isPlainObject(nextValue)) {
    const keys = [...new Set([...Object.keys(prevValue), ...Object.keys(nextValue)])].sort()
    for (const key of keys) {
      collectChangedConfigPaths(prefix ? `${prefix}.${key}` : key, prevValue[key], nextValue[key], changedPaths)
    }
    return
  }

  if (prefix && JSON.stringify(prevValue) !== JSON.stringify(nextValue)) {
    changedPaths.push(prefix)
  }
}

function isPlainObject(value) {
  return value !== null && typeof value === 'object' && !Array.isArray(value)
}

function createLogDetailMap() {
  return {
    [fixtures.logDetail.response.body.log_id]: structuredClone(fixtures.logDetail.response.body),
    log_runtime_0001: {
      log_id: 'log_runtime_0001',
      timestamp: '2026-03-20T10:00:00Z',
      level: 'error',
      source: 'runtime',
      message: 'plugin runtime stderr truncated',
      plugin_id: 'weather',
      request_id: 'req_plugin_0001',
      details: {
        direction: 'internal',
        reason: 'stderr exceeded preview limit',
        payload_preview: {
          plugin_id: 'weather',
          stream: 'stderr',
          line_preview: 'Traceback (most recent call last): ...',
        },
      },
    },
    log_adapter_0001: {
      log_id: 'log_adapter_0001',
      timestamp: '2026-03-20T10:00:01Z',
      level: 'error',
      source: 'adapter.onebot11',
      protocol: 'onebot11',
      message: 'reverse websocket connection lost',
      details: {
        direction: 'inbound',
        frame_type: 'socket.close',
        reason: 'reverse websocket connection lost',
      },
    },
  }
}

let state = baseState()

function json(response, status, body) {
  response.writeHead(status, { 'Content-Type': 'application/json' })
  response.end(JSON.stringify(body))
}

function noContent(response) {
  response.writeHead(204)
  response.end()
}

function parseBody(request) {
  return new Promise((resolve, reject) => {
    const chunks = []
    request.on('data', (chunk) => chunks.push(chunk))
    request.on('end', () => {
      if (chunks.length === 0) {
        resolve({})
        return
      }

      try {
        resolve(JSON.parse(Buffer.concat(chunks).toString('utf8')))
      } catch (error) {
        reject(error)
      }
    })
    request.on('error', reject)
  })
}

function requestUrl(request) {
  return new URL(request.url ?? '/', 'http://127.0.0.1:4010')
}

function authToken(request) {
  const header = request.headers.authorization ?? ''
  return header.startsWith('Bearer ') ? header.slice('Bearer '.length) : null
}

function requireAuth(request, response) {
  if (authToken(request) && authToken(request) === state.token) {
    return true
  }

  json(response, 401, {
    error: {
      code: 'permission.denied',
      message: '需要有效的管理会话',
      message_key: 'errors.permission.denied',
      request_id: 'req_auth_missing_fixture',
    },
  })
  return false
}

function broadcast(channel, frame) {
  for (const socket of sockets[channel]) {
    if (socket.readyState === 1) {
      socket.send(JSON.stringify(frame))
    }
  }
}

function closeAllSockets() {
  for (const channel of Object.keys(sockets)) {
    for (const socket of sockets[channel]) {
      socket.close()
    }
    sockets[channel].clear()
  }
}

function resetState(payload = {}) {
  closeAllSockets()

  state = baseState()
  state.initialized = Boolean(payload.initialized)
  state.token = null
  state.failures = {
    ...state.failures,
    ...(payload.failures ?? {}),
  }
  state.networkOffline = false
}

function takeFailureFlag(name) {
  if (!state.failures[name]) {
    return false
  }

  state.failures[name] = false
  return true
}

function sessionExpiredFrame(channel = 'events') {
  return {
    ...fixtures.wsSessionExpired.frame,
    channel,
  }
}

function createExampleConfigPanelPlugin() {
  const plugin = structuredClone(fixtures.pluginDetailManagementUI.response.body.plugin)

  plugin.source = {
    ...plugin.source,
    package_source_type: 'local_zip',
    package_source_ref: 'examples/plugins/example-config-panel.zip',
    verified: false,
  }
  plugin.trust = {
    level: 'unverified',
    label: '未验证来源',
  }

  return plugin
}

function toPluginSummary(plugin) {
  const summary = {
    id: plugin.id,
    name: plugin.name,
    role: plugin.role,
    state: plugin.state,
    state_diagnosis: structuredClone(plugin.state_diagnosis),
    source: structuredClone(plugin.source),
    trust: structuredClone(plugin.trust),
    commands: structuredClone(plugin.commands ?? []),
    command_conflicts: structuredClone(plugin.command_conflicts ?? []),
  }
  if (plugin.version) {
    summary.version = plugin.version
  }
  if (plugin.description) {
    summary.description = plugin.description
  }
  if (plugin.author) {
    summary.author = plugin.author
  }
  return summary
}

function pluginListBody() {
  return {
    items: Object.values(state.plugins).map((plugin) => toPluginSummary(plugin)),
  }
}

function pluginDetailBody(pluginId) {
  return {
    plugin: structuredClone(state.plugins[pluginId]),
  }
}

function pluginSettingsBody(pluginId) {
  const plugin = state.plugins[pluginId]
  if (!plugin) {
    return null
  }

  return {
    plugin_id: pluginId,
    values: {
      ...structuredClone(plugin.default_config ?? {}),
      ...structuredClone(state.pluginSettings[pluginId] ?? {}),
    },
  }
}

function updatePluginSettings(pluginId, patchValues) {
  const current = pluginSettingsBody(pluginId)
  if (!current) {
    return null
  }

  const mergedValues = {
    ...current.values,
    ...structuredClone(patchValues),
  }
  const changedKeys = Object.keys(patchValues)
    .filter((key) => JSON.stringify(current.values[key]) !== JSON.stringify(mergedValues[key]))
    .sort((left, right) => left.localeCompare(right))

  state.pluginSettings[pluginId] = mergedValues

  return {
    plugin_id: pluginId,
    changed_keys: changedKeys,
    values: structuredClone(mergedValues),
  }
}

function normalizeGovernanceEntryPayload(payload) {
  if (!payload || typeof payload !== 'object' || Array.isArray(payload)) {
    return null
  }

  const entryType = typeof payload.entry_type === 'string' ? payload.entry_type : ''
  const targetId = typeof payload.target_id === 'string' ? payload.target_id.trim() : ''
  const reason = typeof payload.reason === 'string' ? payload.reason.trim() : ''

  if (!['user', 'group'].includes(entryType) || !targetId || !reason) {
    return null
  }

  return {
    entry_type: entryType,
    target_id: targetId,
    reason,
  }
}

function governanceEntryCollection(snapshot, entryType) {
  return entryType === 'group' ? snapshot.group_entries : snapshot.user_entries
}

function governanceEntryCreatedAt(collectionName) {
  if (collectionName === 'whitelist') {
    return fixtures.governanceWhitelistEntryUpsert.response.body.created_at
  }
  return fixtures.governanceBlacklistEntryUpsert.response.body.created_at
}

function upsertGovernanceEntry(snapshot, collectionName, payload) {
  const collection = governanceEntryCollection(snapshot, payload.entry_type)
  const existing = collection.find((entry) => entry.target_id === payload.target_id)

  if (existing) {
    existing.reason = payload.reason
    return structuredClone(existing)
  }

  const entry = {
    entry_type: payload.entry_type,
    target_id: payload.target_id,
    reason: payload.reason,
    created_at: governanceEntryCreatedAt(collectionName),
  }
  collection.push(entry)
  collection.sort((left, right) => left.target_id.localeCompare(right.target_id))
  return structuredClone(entry)
}

function removeGovernanceEntry(snapshot, entryType, targetId) {
  const collection = governanceEntryCollection(snapshot, entryType)
  const index = collection.findIndex((entry) => entry.target_id === targetId)
  if (index < 0) {
    return false
  }
  collection.splice(index, 1)
  return true
}

function isPathInside(parentPath, candidatePath) {
  const relative = path.relative(parentPath, candidatePath)
  return relative === '' || (!relative.startsWith('..') && !path.isAbsolute(relative))
}

function getPluginManagementUIRoot(pluginId) {
  if (pluginId === 'example-config-panel') {
    return exampleConfigPanelRoot
  }

  return null
}

function resolvePluginManagementUIFile(pluginId, requestedPath) {
  const plugin = state.plugins[pluginId]
  const entry = plugin?.management_ui?.pages?.[0]?.entry
  const pluginRoot = getPluginManagementUIRoot(pluginId)
  if (!plugin || typeof entry !== 'string' || !entry.trim() || !pluginRoot) {
    return null
  }

  const normalizedRequestPath = requestedPath
    .split('/')
    .map((segment) => segment.trim())
    .filter((segment) => segment.length > 0)
    .join('/')
  if (!normalizedRequestPath) {
    return null
  }

  const allowedDirectory = path.resolve(pluginRoot, path.dirname(entry))
  const resolvedFilePath = path.resolve(pluginRoot, normalizedRequestPath)
  if (!isPathInside(allowedDirectory, resolvedFilePath)) {
    return null
  }

  return resolvedFilePath
}

function getContentType(filePath) {
  const extension = path.extname(filePath).toLowerCase()
  switch (extension) {
    case '.html':
      return 'text/html; charset=utf-8'
    case '.js':
      return 'text/javascript; charset=utf-8'
    case '.css':
      return 'text/css; charset=utf-8'
    case '.json':
      return 'application/json; charset=utf-8'
    case '.svg':
      return 'image/svg+xml'
    case '.png':
      return 'image/png'
    default:
      return 'application/octet-stream'
  }
}

function mergePluginState(pluginId, patch) {
  const previous = state.plugins[pluginId] ?? {}
  state.plugins[pluginId] = {
    ...structuredClone(previous),
    ...structuredClone(patch),
    source: structuredClone(patch.source ?? previous.source),
    trust: structuredClone(patch.trust ?? previous.trust),
    commands: structuredClone(patch.commands ?? previous.commands ?? []),
    command_conflicts: structuredClone(patch.command_conflicts ?? previous.command_conflicts ?? []),
  }
  return state.plugins[pluginId]
}

function appendLogSummary(summary, detail, options = {}) {
  state.logs = [
    ...state.logs.filter((item) => item.log_id !== summary.log_id),
    structuredClone(summary),
  ]

  if (options.currentSession !== false) {
    state.currentSessionLogIds.add(summary.log_id)
  }

  if (detail) {
    state.logDetails[summary.log_id] = structuredClone(detail)
  }
}

function appendTaskLog(taskId, taskType, status, summary, options = {}) {
  const timestamp = options.timestamp ?? new Date().toISOString()
  const level = status === 'failed'
    ? 'error'
    : ['cancelled', 'interrupted'].includes(status) ? 'warn' : 'info'
  const logSummary = {
    log_id: `log_${taskId}_${status}`,
    timestamp,
    level,
    source: 'tasks',
    plugin_id: options.plugin_id,
    request_id: taskId,
    message: `任务${taskStatusText(status)} ${taskType}：${summary}`,
  }
  const detail = {
    ...logSummary,
    details: {
      task_id: taskId,
      task_type: taskType,
      task_status: status,
      task_summary: summary,
      ...(options.details ?? {}),
    },
  }
  appendLogSummary(logSummary, detail)
  broadcast('logs', {
    channel: 'logs',
    type: 'logs.appended',
    timestamp,
    data: logSummary,
  })
  return logSummary
}

function taskStatusText(status) {
  switch (status) {
    case 'pending':
      return '已提交'
    case 'running':
      return '运行中'
    case 'succeeded':
      return '已完成'
    case 'failed':
      return '失败'
    case 'cancelled':
      return '已取消'
    case 'interrupted':
      return '已中断'
    default:
      return status
  }
}

function normalizeSortableTimestamp(value) {
  if (value === null || value === undefined) {
    return Number.NEGATIVE_INFINITY
  }

  if (typeof value === 'number' && Number.isFinite(value)) {
    return normalizeUnixTimestamp(value)
  }

  const raw = String(value).trim()
  if (!raw) {
    return Number.NEGATIVE_INFINITY
  }

  const numeric = Number(raw)
  if (Number.isFinite(numeric)) {
    return normalizeUnixTimestamp(numeric)
  }

  const parsed = Date.parse(raw)
  return Number.isFinite(parsed) ? parsed : Number.NEGATIVE_INFINITY
}

function normalizeUnixTimestamp(value) {
  const absolute = Math.abs(value)
  if (absolute >= 1_000_000_000 && absolute < 1_000_000_000_000) {
    return value * 1000
  }
  return value
}

function compareLogsDesc(left, right) {
  const leftTimestamp = normalizeSortableTimestamp(left.timestamp)
  const rightTimestamp = normalizeSortableTimestamp(right.timestamp)

  if (leftTimestamp !== rightTimestamp) {
    return rightTimestamp - leftTimestamp
  }

  return String(right.log_id ?? '').localeCompare(String(left.log_id ?? ''))
}

function encodeLogCursor(item) {
  return Buffer.from(JSON.stringify({
    log_id: item.log_id,
  }), 'utf8').toString('base64url')
}

function decodeLogCursor(raw) {
  if (!raw) {
    return null
  }

  try {
    const decoded = JSON.parse(Buffer.from(raw, 'base64url').toString('utf8'))
    return typeof decoded?.log_id === 'string' ? decoded.log_id : null
  } catch {
    return null
  }
}

function listLogPage(searchParams) {
  const scope = searchParams.get('scope') === 'current_session' ? 'current_session' : 'history'
  const startAt = searchParams.get('start_at')
  const endAt = searchParams.get('end_at')
  const levels = normalizedSearchValues(searchParams, 'level')
  const source = searchParams.get('source')
  const protocol = searchParams.get('protocol')
  const pluginIds = normalizedSearchValues(searchParams, 'plugin_id')
  const requestId = searchParams.get('request_id')
  const limit = Math.max(1, Number(searchParams.get('limit') ?? '50') || 50)
  const direction = searchParams.get('direction') === 'newer' ? 'newer' : 'older'
  const cursorLogId = decodeLogCursor(searchParams.get('cursor'))

  const filtered = state.logs
    .filter((item) => {
      const timestamp = normalizeSortableTimestamp(item.timestamp)
      if (scope === 'current_session' && !state.currentSessionLogIds.has(item.log_id)) return false
      if (scope === 'history' && startAt && timestamp < normalizeSortableTimestamp(startAt)) return false
      if (scope === 'history' && endAt && timestamp > normalizeSortableTimestamp(endAt)) return false
      if (levels.length > 0 && !levels.includes(item.level)) return false
      if (source && item.source !== source) return false
      if (protocol && item.protocol !== protocol) return false
      if (pluginIds.length > 0 && !pluginIds.includes(item.plugin_id ?? '')) return false
      if (requestId && item.request_id !== requestId) return false
      return true
    })
    .slice()
    .sort(compareLogsDesc)

  let startIndex = 0
  let endIndex = Math.min(limit, filtered.length)
  const cursorIndex = cursorLogId
    ? filtered.findIndex((item) => item.log_id === cursorLogId)
    : -1

  if (cursorIndex >= 0) {
    if (direction === 'older') {
      startIndex = cursorIndex + 1
      endIndex = Math.min(filtered.length, startIndex + limit)
    } else {
      endIndex = cursorIndex
      startIndex = Math.max(0, endIndex - limit)
    }
  }

  const items = filtered.slice(startIndex, endIndex)
  const hasNewer = startIndex > 0
  const hasOlder = endIndex < filtered.length

  return {
    items,
    page: {
      limit,
      has_older: hasOlder,
      has_newer: hasNewer,
      older_cursor: hasOlder && items.length > 0 ? encodeLogCursor(items.at(-1)) : null,
      newer_cursor: hasNewer && items.length > 0 ? encodeLogCursor(items[0]) : null,
    },
  }
}

function normalizedSearchValues(searchParams, key) {
  return Array.from(new Set(
    searchParams
      .getAll(key)
      .map((value) => value.trim())
      .filter(Boolean),
  ))
}

function defaultProtocolLiveLog() {
  const summary = {
    log_id: 'log_adapter_live_0001',
    timestamp: '2026-04-08T10:16:00Z',
    level: 'warn',
    source: 'adapter.onebot11',
    protocol: 'onebot11',
    message: 'ignored OneBot API response with unsupported echo',
    request_id: 'req_adapter_ignored_0001',
  }

  return {
    summary,
    detail: {
      ...summary,
      details: {
        direction: 'inbound',
        frame_type: 'api.response.ignored',
        reason: 'api response echo must be a non-empty string',
        echo_value_type: 'number',
        payload_preview: {
          status: 'ok',
          retcode: 0,
          echo: 123,
          wording: 'ignored by adapter',
        },
      },
    },
  }
}

function errorEnvelope(code, message, requestId, details) {
  return {
    error: {
      code,
      message,
      message_key: `errors.${code}`,
      request_id: requestId,
      ...(details ? { details } : {}),
    },
  }
}

const server = http.createServer(async (request, response) => {
  const url = requestUrl(request)
  const { pathname, searchParams } = url

  if (pathname === '/__test/ping') {
    json(response, 200, { ok: true })
    return
  }

  if (pathname === '/__test/reset' && request.method === 'POST') {
    const payload = await parseBody(request)
    resetState(payload)
    json(response, 200, { ok: true, initialized: state.initialized })
    return
  }

  if (pathname === '/__test/session-expire' && request.method === 'POST') {
    state.token = null
    for (const channel of Object.keys(sockets)) {
      for (const socket of sockets[channel]) {
        socket.send(JSON.stringify(sessionExpiredFrame(channel)))
        socket.close()
      }
    }
    json(response, 200, { ok: true })
    return
  }

  if (pathname === '/__test/socket-close' && request.method === 'POST') {
    const payload = await parseBody(request)
    const channel = payload.channel
    if (!channel || !sockets[channel]) {
      json(response, 400, errorEnvelope('platform.invalid_request', 'invalid socket channel', 'req_socket_close_invalid'))
      return
    }

    for (const socket of sockets[channel]) {
      socket.close()
    }

    json(response, 200, { ok: true, channel })
    return
  }

  if (pathname === '/__test/push-log' && request.method === 'POST') {
    const payload = await parseBody(request)
    const seed = defaultProtocolLiveLog()
    const summary = {
      ...seed.summary,
      ...(payload.summary ?? payload),
      log_id: (payload.summary?.log_id ?? payload.log_id ?? seed.summary.log_id),
      timestamp: (payload.summary?.timestamp ?? payload.timestamp ?? new Date().toISOString()),
    }
    const detail = {
      ...seed.detail,
      ...(payload.detail ?? {}),
      ...summary,
      details: structuredClone(payload.detail?.details ?? payload.details ?? seed.detail.details),
    }

    appendLogSummary(summary, detail, {
      currentSession: payload.scope !== 'history',
    })
    broadcast('logs', {
      channel: 'logs',
      type: 'logs.appended',
      timestamp: summary.timestamp,
      data: summary,
    })
    json(response, 200, { ok: true, log_id: summary.log_id })
    return
  }

  if (pathname === '/healthz' && request.method === 'GET') {
    json(response, fixtures.healthz.response.status, fixtures.healthz.response.body)
    return
  }

  if (pathname === '/readyz' && request.method === 'GET') {
    json(response, fixtures.readyz.response.status, fixtures.readyz.response.body)
    return
  }

  if (pathname.startsWith('/plugin-ui/') && (request.method === 'GET' || request.method === 'HEAD')) {
    const pathSegments = pathname.split('/')
    const pluginId = decodeURIComponent(pathSegments[2] ?? '')
    const requestedPath = pathSegments
      .slice(3)
      .map((segment) => decodeURIComponent(segment))
      .join('/')
    const filePath = resolvePluginManagementUIFile(pluginId, requestedPath)

    if (!filePath) {
      json(response, 404, errorEnvelope('platform.not_found', 'plugin management page not found', 'req_plugin_ui_not_found'))
      return
    }

    try {
      const file = await readFile(filePath)
      response.writeHead(200, {
        'Content-Type': getContentType(filePath),
        'Cache-Control': 'no-store',
      })
      if (request.method === 'HEAD') {
        response.end()
        return
      }

      response.end(file)
      return
    } catch {
      json(response, 404, errorEnvelope('platform.not_found', 'plugin management page not found', 'req_plugin_ui_missing'))
      return
    }
  }

  if (pathname === '/api/setup/status' && request.method === 'GET') {
    json(response, fixtures.setupStatus.response.status, {
      initialized: state.initialized,
    })
    return
  }

  if (pathname === '/api/setup/admin' && request.method === 'POST') {
    if (state.initialized) {
      json(response, fixtures.setupAdminDenied.response.status, fixtures.setupAdminDenied.response.body)
      return
    }

    const payload = await parseBody(request)
    if (!payload.identifier || !payload.secret) {
      json(response, 400, {
        error: {
          code: 'platform.invalid_request',
          message: '缺少初始化字段',
          message_key: 'errors.platform.invalid_request',
          request_id: 'req_setup_admin_invalid',
        },
      })
      return
    }

    state.initialized = true
    state.token = fixtures.setupAdmin.response.body.session_token
    json(response, fixtures.setupAdmin.response.status, fixtures.setupAdmin.response.body)
    return
  }

  if (pathname === '/api/session/login' && request.method === 'POST') {
    const payload = await parseBody(request)
    if (!state.initialized || payload.identifier !== 'admin' || payload.secret !== 'fixture-only-secret') {
      json(response, fixtures.sessionDenied.response.status, fixtures.sessionDenied.response.body)
      return
    }

    state.token = fixtures.sessionLogin.response.body.session_token
    json(response, fixtures.sessionLogin.response.status, fixtures.sessionLogin.response.body)
    return
  }

  if (pathname === '/api/session' && request.method === 'DELETE') {
    if (!requireAuth(request, response)) {
      return
    }

    state.token = null
    noContent(response)
    return
  }

  if (pathname === '/api/system/status' && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    if (takeFailureFlag('failSystemStatusOnce')) {
      json(response, 500, errorEnvelope('plugin.internal_error', 'system status failed', 'req_system_status_failed'))
      return
    }

    json(response, fixtures.systemStatus.response.status, state.systemStatus)
    return
  }

  if (pathname === '/api/governance/blacklist' && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    json(response, 200, structuredClone(state.governanceBlacklist))
    return
  }

  if (pathname === '/__test/network-offline' && request.method === 'POST') {
    state.networkOffline = true
    closeAllSockets()
    json(response, 200, { ok: true })
    return
  }

  if (pathname === '/__test/push-task' && request.method === 'POST') {
    const payload = await parseBody(request)
    if (!payload.task_id || !payload.task_type || !payload.status || !payload.summary) {
      json(response, 400, errorEnvelope('platform.invalid_request', 'task payload is invalid', 'req_test_task_invalid'))
      return
    }

    const taskId = String(payload.task_id)
    appendTaskLog(taskId, String(payload.task_type), String(payload.status), String(payload.summary), {
      timestamp: typeof payload.finished_at === 'string' ? payload.finished_at : undefined,
      plugin_id: typeof payload.plugin_id === 'string' ? payload.plugin_id : undefined,
      details: {
        progress: typeof payload.progress === 'number' ? payload.progress : undefined,
        started_at: typeof payload.started_at === 'string' ? payload.started_at : '2026-04-22T10:00:00Z',
        finished_at: typeof payload.finished_at === 'string' ? payload.finished_at : '2026-04-22T10:00:05Z',
      },
    })
    json(response, 200, { ok: true, log_id: `log_${taskId}_${String(payload.status)}` })
    return
  }

  if (pathname === '/__test/network-online' && request.method === 'POST') {
    state.networkOffline = false
    json(response, 200, { ok: true })
    return
  }

  if (pathname === '/external-preview/font.css' && request.method === 'GET') {
    response.writeHead(200, {
      'Content-Type': 'text/css; charset=utf-8',
      'Cache-Control': 'no-store',
      'Access-Control-Allow-Origin': '*',
    })
    response.end('@font-face{font-family:"RayleaExternalPreview";src:url("http://127.0.0.1:4010/external-preview/font.woff2") format("woff2");font-style:normal;font-weight:400;font-display:block;unicode-range:U+20-7E;}')
    return
  }

  if (pathname === '/external-preview/font.woff2' && request.method === 'GET') {
    response.writeHead(200, {
      'Content-Type': 'font/woff2',
      'Cache-Control': 'no-store',
      'Access-Control-Allow-Origin': '*',
    })
    response.end(externalPreviewFontBytes)
    return
  }

  if ((pathname === '/external-preview/avatar.png' || pathname === '/external-preview/background.png') && request.method === 'GET') {
    response.writeHead(200, {
      'Content-Type': 'image/png',
      'Cache-Control': 'no-store',
      'Access-Control-Allow-Origin': '*',
    })
    response.end(externalPreviewImageBytes)
    return
  }

  if (state.networkOffline) {
    request.socket.destroy()
    return
  }

  if (pathname === '/api/governance/blacklist/entries' && request.method === 'POST') {
    if (!requireAuth(request, response)) {
      return
    }

    const payload = normalizeGovernanceEntryPayload(await parseBody(request))
    if (!payload) {
      json(response, 400, errorEnvelope('platform.invalid_request', 'governance entry payload is invalid', 'req_governance_blacklist_invalid'))
      return
    }

    json(response, 200, upsertGovernanceEntry(state.governanceBlacklist, 'blacklist', payload))
    return
  }

  if (pathname.startsWith('/api/governance/blacklist/entries/') && request.method === 'DELETE') {
    if (!requireAuth(request, response)) {
      return
    }

    const entryType = decodeURIComponent(pathname.split('/')[5] ?? '')
    const targetId = decodeURIComponent(pathname.split('/')[6] ?? '')
    if (!['user', 'group'].includes(entryType) || !targetId) {
      json(response, 404, errorEnvelope('platform.not_found', 'governance entry not found', 'req_governance_blacklist_entry_not_found'))
      return
    }

    if (!removeGovernanceEntry(state.governanceBlacklist, entryType, targetId)) {
      json(response, 404, errorEnvelope('platform.not_found', 'governance entry not found', 'req_governance_blacklist_entry_not_found'))
      return
    }

    noContent(response)
    return
  }

  if (pathname === '/api/governance/whitelist' && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    json(response, 200, structuredClone(state.governanceWhitelist))
    return
  }

  if (pathname === '/api/governance/whitelist/state' && request.method === 'PUT') {
    if (!requireAuth(request, response)) {
      return
    }

    const payload = await parseBody(request)
    if (!payload || typeof payload.enabled !== 'boolean') {
      json(response, 400, errorEnvelope('platform.invalid_request', 'governance whitelist state payload is invalid', 'req_governance_whitelist_state_invalid'))
      return
    }

    state.governanceWhitelist.enabled = payload.enabled
    json(response, fixtures.governanceWhitelistState.response.status, { enabled: state.governanceWhitelist.enabled })
    return
  }

  if (pathname === '/api/governance/whitelist/entries' && request.method === 'POST') {
    if (!requireAuth(request, response)) {
      return
    }

    const payload = normalizeGovernanceEntryPayload(await parseBody(request))
    if (!payload) {
      json(response, 400, errorEnvelope('platform.invalid_request', 'governance entry payload is invalid', 'req_governance_whitelist_invalid'))
      return
    }

    json(response, 200, upsertGovernanceEntry(state.governanceWhitelist, 'whitelist', payload))
    return
  }

  if (pathname.startsWith('/api/governance/whitelist/entries/') && request.method === 'DELETE') {
    if (!requireAuth(request, response)) {
      return
    }

    const entryType = decodeURIComponent(pathname.split('/')[5] ?? '')
    const targetId = decodeURIComponent(pathname.split('/')[6] ?? '')
    if (!['user', 'group'].includes(entryType) || !targetId) {
      json(response, 404, errorEnvelope('platform.not_found', 'governance entry not found', 'req_governance_whitelist_entry_not_found'))
      return
    }

    if (!removeGovernanceEntry(state.governanceWhitelist, entryType, targetId)) {
      json(response, 404, errorEnvelope('platform.not_found', 'governance entry not found', 'req_governance_whitelist_entry_not_found'))
      return
    }

    noContent(response)
    return
  }

  if (pathname === '/api/governance/command-policy' && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    json(response, 200, structuredClone(state.governanceCommandPolicy))
    return
  }

  if (pathname === '/api/system/shutdown' && request.method === 'POST') {
    if (!requireAuth(request, response)) {
      return
    }

    state.systemStatus.status = 'shutting_down'
    json(response, fixtures.systemShutdown.response.status, fixtures.systemShutdown.response.body)
    setTimeout(() => {
      state.networkOffline = true
      closeAllSockets()
    }, 50)
    return
  }

  if (pathname === '/api/system/backup' && request.method === 'POST') {
    if (!requireAuth(request, response)) {
      return
    }

    const taskId = fixtures.systemBackupAccepted.response.body.task_id
    appendTaskLog(taskId, 'backup.create', 'pending', 'create online backup')

    json(response, fixtures.systemBackupAccepted.response.status, fixtures.systemBackupAccepted.response.body)
    return
  }

  if (pathname === '/api/system/diagnostics/export' && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    response.writeHead(fixtures.systemDiagnosticsExport.response.status, {
      'Content-Type': fixtures.systemDiagnosticsExport.response.headers['Content-Type'],
      'Content-Disposition': fixtures.systemDiagnosticsExport.response.headers['Content-Disposition'],
    })
    response.end(Buffer.from('PK\x03\x04fixture-diagnostics'))
    return
  }

  if (pathname === '/api/system/render/templates' && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    json(response, 200, listRenderTemplates())
    return
  }

  if (pathname.startsWith('/api/system/render/templates/') && pathname.endsWith('/asset') && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    response.writeHead(200, {
      'Content-Type': 'application/octet-stream',
      'Cache-Control': 'no-store',
    })
    response.end(Buffer.from('template asset'))
    return
  }

  if (pathname.startsWith('/api/system/render/templates/') && pathname.endsWith('/preview-html') && request.method === 'POST') {
    if (!requireAuth(request, response)) {
      return
    }

    const templateId = decodeURIComponent(pathname.split('/')[5] ?? '')
    const payload = await parseBody(request)
    const body = renderTemplatePreviewHTMLBody(templateId, payload)
    if (!body) {
      json(response, fixtures.renderTemplateNotFound.response.status, fixtures.renderTemplateNotFound.response.body)
      return
    }
    json(response, 200, body)
    return
  }

  if (pathname.startsWith('/api/system/render/templates/') && request.method === 'GET' && pathname.split('/').length === 6) {
    if (!requireAuth(request, response)) {
      return
    }

    const templateId = decodeURIComponent(pathname.split('/')[5] ?? '')
    const detailBody = renderTemplateDetailBody(templateId)
    if (!detailBody) {
      json(response, fixtures.renderTemplateNotFound.response.status, fixtures.renderTemplateNotFound.response.body)
      return
    }

    json(response, 200, detailBody)
    return
  }

  if (pathname === '/api/system/scheduler/jobs' && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    json(response, 200, { items: structuredClone(state.schedulerJobs) })
    return
  }

  if (pathname.startsWith('/api/system/scheduler/jobs/') && pathname.endsWith('/trigger') && request.method === 'POST') {
    if (!requireAuth(request, response)) {
      return
    }

    const jobId = decodeURIComponent(pathname.split('/')[5] ?? '')
    const job = state.schedulerJobs.find((item) => item.job_id === jobId)
    if (!job) {
      json(response, 404, errorEnvelope('platform.not_found', 'scheduler job not found', 'req_scheduler_job_not_found'))
      return
    }

    json(response, fixtures.schedulerJobTriggered.response.status, {
      job_id: job.job_id,
      plugin_id: job.plugin_id,
      triggered: true,
    })
    return
  }

  if (pathname === '/api/config' && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    const snapshot = redactConfigSecrets(state.config)
    json(response, 200, {
      config: snapshot.config,
      redacted_fields: snapshot.redacted_fields,
    })
    return
  }

  if (pathname === '/api/config' && request.method === 'PUT') {
    if (!requireAuth(request, response)) {
      return
    }

    const payload = await parseBody(request)
    const previousConfig = structuredClone(state.config)
    state.config = restoreRedactedConfigSecrets(payload, state.config)
    syncGovernanceCommandPolicyFromConfig(state.config)
    const applyEffects = computeConfigApplyEffects(previousConfig, state.config)
    state.protocolSnapshot = computeProtocolSnapshotFromConfig(state.config, state.protocolSnapshot)
    broadcast('events', {
      channel: 'events',
      type: 'events.received',
      timestamp: new Date().toISOString(),
      data: {
        protocol: 'onebot11',
        protocol_snapshot: structuredClone(state.protocolSnapshot),
      },
    })
    const snapshot = redactConfigSecrets(state.config)
    json(response, 200, {
      config: snapshot.config,
      redacted_fields: snapshot.redacted_fields,
      restart_required: computeRestartRequiredForConfig(previousConfig, state.config),
      apply_effects: applyEffects,
    })
    return
  }

  if (pathname === '/api/protocols/onebot11' && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    json(response, 200, structuredClone(state.protocolSnapshot))
    return
  }

  if (pathname === '/api/protocols/onebot11/compatibility' && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    json(response, 200, structuredClone(fixtures.protocolCompatibility.response.body))
    return
  }

  if (pathname === '/api/third-party/accounts' && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    json(response, 200, { items: structuredClone(state.thirdPartyAccounts) })
    return
  }

  if (pathname.startsWith('/api/third-party/accounts/') && request.method === 'PUT') {
    if (!requireAuth(request, response)) {
      return
    }

    const segments = pathname.split('/')
    const platform = decodeURIComponent(segments[4] ?? '')
    const accountId = decodeURIComponent(segments[5] ?? '')
    const payload = await parseBody(request)
    if (!thirdPartyAccountPlatforms.includes(platform) || !accountId || !payload || typeof payload.label !== 'string' || typeof payload.enabled !== 'boolean') {
      json(response, 400, errorEnvelope('platform.invalid_request', 'third-party account payload is invalid', 'req_third_party_account_invalid'))
      return
    }

    const previous = state.thirdPartyAccounts.find((item) => item.platform === platform && item.account_id === accountId)
    const fixtureAccount = platform === 'bilibili'
      ? structuredClone(fixtures.thirdPartyAccountUpsert.response.body.account)
      : null
    const succeededFixture = thirdPartyQRCodeFixtures(platform)?.succeeded?.response.body
    const nextAccount = {
      ...(fixtureAccount ?? {}),
      ...(previous ?? {}),
      platform,
      account_id: accountId,
      label: payload.label,
      enabled: payload.enabled,
      configured: previous?.configured || Boolean(payload.cookie),
      profile: previous?.profile ?? null,
      credential: previous?.credential ?? defaultCredentialStatus(platform),
      updated_at: new Date().toISOString(),
    }
    if (payload.cookie) {
      nextAccount.profile = platform === 'bilibili'
        ? structuredClone(fixtureAccount.profile)
        : structuredClone(succeededFixture?.account?.profile ?? previous?.profile ?? null)
      if (platform === 'weibo' && nextAccount.profile) {
        nextAccount.profile.avatar_url = weiboAvatarUrl
      }
      nextAccount.credential = defaultCredentialStatus(platform)
      nextAccount.configured = true
    }
    localizeBilibiliAccountAvatar(nextAccount)
    state.thirdPartyAccounts = [
      ...state.thirdPartyAccounts.filter((item) => item.platform !== platform || item.account_id !== accountId),
      nextAccount,
    ].sort((left, right) => left.account_id.localeCompare(right.account_id))
    json(response, 200, { account: structuredClone(nextAccount) })
    return
  }

  if (pathname.startsWith('/api/third-party/accounts/') && pathname.endsWith('/login/qrcode') && request.method === 'POST') {
    if (!requireAuth(request, response)) {
      return
    }

    const segments = pathname.split('/')
    const platform = decodeURIComponent(segments[4] ?? '')
    const fixture = thirdPartyQRCodeFixtures(platform)?.create
    if (!fixture) {
      json(response, 400, errorEnvelope('platform.invalid_request', 'third-party qrcode platform is invalid', 'req_third_party_qr_invalid'))
      return
    }
    const body = structuredClone(fixture.response.body)
    state.thirdPartyQRCodePolls[thirdPartyQRCodePollKey(platform, body.login_id)] = 0
    json(response, fixture.response.status, body)
    return
  }

  if (pathname.startsWith('/api/third-party/accounts/') && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    const segments = pathname.split('/')
    const platform = decodeURIComponent(segments[4] ?? '')
    const loginId = decodeURIComponent(segments[7] ?? '')
    const fixturesForPlatform = thirdPartyQRCodeFixtures(platform)
    const pollKey = thirdPartyQRCodePollKey(platform, loginId)
    if (segments[5] !== 'login' || segments[6] !== 'qrcode' || !fixturesForPlatform || !loginId || !(pollKey in state.thirdPartyQRCodePolls)) {
      json(response, 400, errorEnvelope('platform.invalid_request', 'qr login session not found', 'req_third_party_qr_missing'))
      return
    }
    state.thirdPartyQRCodePolls[pollKey] += 1
    const fixture = state.thirdPartyQRCodePolls[pollKey] > 1
      ? fixturesForPlatform.succeeded
      : fixturesForPlatform.pending
    const body = structuredClone(fixture.response.body)
    if (platform === 'weibo' && body.account?.profile) {
      body.account.profile.avatar_url = weiboAvatarUrl
    }
    json(response, fixture.response.status, {
      ...body,
      login_id: loginId,
    })
    return
  }

  if (pathname.startsWith('/api/third-party/accounts/') && request.method === 'DELETE') {
    if (!requireAuth(request, response)) {
      return
    }

    const segments = pathname.split('/')
    const platform = decodeURIComponent(segments[4] ?? '')
    const accountId = decodeURIComponent(segments[5] ?? '')
    state.thirdPartyAccounts = state.thirdPartyAccounts.filter((item) => item.platform !== platform || item.account_id !== accountId)
    noContent(response)
    return
  }

  if (pathname === '/api/logs' && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    if (takeFailureFlag('failLogsOnce')) {
      json(response, 500, errorEnvelope('plugin.internal_error', 'log list failed', 'req_logs_failed'))
      return
    }

    json(response, 200, listLogPage(searchParams))
    return
  }

  if (pathname.startsWith('/api/logs/') && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    const logId = decodeURIComponent(pathname.split('/')[3] ?? '')
    const detail = state.logDetails[logId]
    if (!detail) {
      json(response, fixtures.logDetailNotFound.response.status, fixtures.logDetailNotFound.response.body)
      return
    }

    json(response, 200, structuredClone(detail))
    return
  }

  if (pathname === '/api/plugins' && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    if (takeFailureFlag('failPluginsListOnce')) {
      json(response, 500, errorEnvelope('plugin.internal_error', 'plugin list failed', 'req_plugins_failed'))
      return
    }

    json(response, 200, pluginListBody())
    return
  }

  if (pathname === '/api/plugins/install' && request.method === 'POST') {
    if (!requireAuth(request, response)) {
      return
    }

    const payload = await parseBody(request)
    let taskId = fixtures.pluginInstallAccepted.response.body.task_id

    if (payload.source_type === 'remote_url') {
      taskId = fixtures.pluginInstallRemoteUrl.response.body.task_id
    } else if (payload.source.includes('script-blocked') && payload.allow_install_scripts !== true) {
      taskId = 'task_plugin_install_failed_script_blocked_0001'
    } else if (payload.allow_install_scripts === true) {
      taskId = fixtures.pluginInstallAcceptedWithScripts.response.body.task_id
    }

    appendTaskLog(taskId, 'plugin.install', 'pending', `install ${payload.source}`, {
      plugin_id: payload.source_type === 'remote_url' ? undefined : 'weather',
    })

    json(response, 202, { task_id: taskId })
    return
  }

  if (pathname.startsWith('/api/plugins/') && pathname.endsWith('/enable') && request.method === 'POST') {
    if (!requireAuth(request, response)) {
      return
    }

    const pluginId = pathname.split('/')[3]
    mergePluginState(pluginId, fixtures.pluginEnable.response.body.plugin)
    broadcast('events', {
      channel: 'events',
      type: 'events.received',
      timestamp: new Date().toISOString(),
      data: {
        plugin_id: pluginId,
        state: state.plugins[pluginId].state,
        commands: structuredClone(state.plugins[pluginId].commands ?? []),
        command_conflicts: structuredClone(state.plugins[pluginId].command_conflicts ?? []),
      },
    })
    json(response, 200, pluginDetailBody(pluginId))
    return
  }

  if (pathname.startsWith('/api/plugins/') && pathname.endsWith('/disable') && request.method === 'POST') {
    if (!requireAuth(request, response)) {
      return
    }

    const pluginId = pathname.split('/')[3]
    mergePluginState(pluginId, fixtures.pluginDisable.response.body.plugin)
    broadcast('events', {
      channel: 'events',
      type: 'events.received',
      timestamp: new Date().toISOString(),
      data: {
        plugin_id: pluginId,
        state: state.plugins[pluginId].state,
        commands: structuredClone(state.plugins[pluginId].commands ?? []),
        command_conflicts: structuredClone(state.plugins[pluginId].command_conflicts ?? []),
      },
    })
    json(response, 200, pluginDetailBody(pluginId))
    return
  }

  if (pathname.startsWith('/api/plugins/') && pathname.endsWith('/reload') && request.method === 'POST') {
    if (!requireAuth(request, response)) {
      return
    }

    const pluginId = pathname.split('/')[3]
    mergePluginState(pluginId, fixtures.pluginReload.response.body.plugin)
    json(response, 200, pluginDetailBody(pluginId))
    return
  }

  if (pathname.startsWith('/api/plugins/') && pathname.endsWith('/settings') && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    const pluginId = pathname.split('/')[3]
    const settingsBody = pluginSettingsBody(pluginId)
    if (!settingsBody) {
      json(response, 404, errorEnvelope('platform.not_found', 'plugin settings not found', 'req_plugin_settings_not_found'))
      return
    }

    json(response, 200, settingsBody)
    return
  }

  if (pathname.startsWith('/api/plugins/') && pathname.endsWith('/settings') && request.method === 'PUT') {
    if (!requireAuth(request, response)) {
      return
    }

    const pluginId = pathname.split('/')[3]
    const payload = await parseBody(request)
    if (!payload || !payload.values || typeof payload.values !== 'object' || Array.isArray(payload.values)) {
      json(response, 400, errorEnvelope('platform.invalid_request', 'plugin settings payload is invalid', 'req_plugin_settings_invalid'))
      return
    }

    const updatedBody = updatePluginSettings(pluginId, payload.values)
    if (!updatedBody) {
      json(response, 404, errorEnvelope('platform.not_found', 'plugin settings not found', 'req_plugin_settings_not_found'))
      return
    }

    json(response, 200, updatedBody)
    return
  }

  if (pathname.startsWith('/api/plugins/') && request.method === 'DELETE') {
    if (!requireAuth(request, response)) {
      return
    }

    const pluginId = pathname.split('/')[3]
    if (takeFailureFlag('failUninstallOnce') || !state.plugins[pluginId]) {
      json(response, fixtures.invalidUninstallNotFound.response.status, fixtures.invalidUninstallNotFound.response.body)
      return
    }

    const taskId = fixtures.pluginUninstallAccepted.response.body.task_id
    appendTaskLog(taskId, 'plugin.uninstall', 'pending', `uninstall ${pluginId}`, {
      plugin_id: pluginId,
    })
    json(response, fixtures.pluginUninstallAccepted.response.status, fixtures.pluginUninstallAccepted.response.body)
    return
  }

  if (pathname.startsWith('/api/plugins/') && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    const pluginId = pathname.split('/')[3]
    if (takeFailureFlag('failPluginDetailOnce')) {
      json(response, 500, errorEnvelope('plugin.internal_error', 'plugin detail failed', 'req_plugin_detail_failed'))
      return
    }
    json(response, 200, pluginDetailBody(pluginId))
    return
  }

  json(response, 404, {
    error: {
      code: 'platform.not_found',
      message: 'mock route not found',
      message_key: 'errors.platform.not_found',
      request_id: 'req_mock_not_found',
    },
  })
})

const wsServer = new WebSocketServer({ noServer: true })

wsServer.on('connection', (socket, request) => {
  const url = requestUrl(request)
  const token = url.searchParams.get('session_token')
  const pathname = url.pathname

  const channel = pathname.startsWith('/ws/plugins/') ? 'plugin_console' : pathname.replace('/ws/', '')

  if (!token || token !== state.token || !sockets[channel]) {
    socket.send(JSON.stringify(sessionExpiredFrame(channel === 'plugin_console' ? 'plugin_console' : channel)))
    socket.close()
    return
  }

  sockets[channel].add(socket)
  socket.on('close', () => {
    sockets[channel].delete(socket)
  })

  if (channel === 'events') {
    setTimeout(() => socket.send(JSON.stringify({
      ...fixtures.wsEventsProtocolSnapshot.frame,
      data: {
        ...fixtures.wsEventsProtocolSnapshot.frame.data,
        protocol_snapshot: structuredClone(state.protocolSnapshot),
      },
    })), 80)
    setTimeout(() => socket.send(JSON.stringify(fixtures.wsEvents.frame)), 120)
  } else if (channel === 'logs') {
    setTimeout(() => {
      const liveLog = defaultProtocolLiveLog()
      appendLogSummary(liveLog.summary, liveLog.detail)
      socket.send(JSON.stringify({
        channel: 'logs',
        type: 'logs.appended',
        timestamp: liveLog.summary.timestamp,
        data: liveLog.summary,
      }))
    }, 210)
  } else if (channel === 'plugin_console') {
    const pluginId = pathname.split('/')[3]
    setTimeout(() => {
      socket.send(JSON.stringify({
        ...fixtures.wsConsole.frame,
        data: {
          ...fixtures.wsConsole.frame.data,
          plugin_id: pluginId,
        },
      }))
    }, 120)
  }
})

server.on('upgrade', (request, socket, head) => {
  if (state.networkOffline) {
    socket.destroy()
    return
  }

  wsServer.handleUpgrade(request, socket, head, (connection) => {
    wsServer.emit('connection', connection, request)
  })
})

server.listen(4010, '127.0.0.1', () => {
  process.stdout.write('mock backend ready\n')
})
