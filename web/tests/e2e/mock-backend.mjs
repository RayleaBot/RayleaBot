import { redactConfigSecrets } from './mock-config.mjs'
import { listLogPage } from './mock-logs.mjs'
import http from 'node:http'
import { createHash } from 'node:crypto'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { readFile } from 'node:fs/promises'

import YAML from 'yaml'
import { WebSocketServer } from 'ws'

const __filename = fileURLToPath(import.meta.url)
const __dirname = path.dirname(__filename)
const repoRoot = path.resolve(__dirname, '..', '..', '..')
const webDistRoot = process.env.RAYLEA_E2E_SERVE_WEB_DIST === '1' ? path.join(repoRoot, 'web', 'dist') : null
const exampleConfigPanelRoot = path.join(repoRoot, 'examples', 'plugins', 'example-config-panel', 'ui', 'dist')
const exampleConfigPanelHost = `p-${createHash('sha256').update('example-config-panel').digest('hex').slice(0, 16)}.plugins.localhost:4010`
const configuredWebOrigin = String(process.env.RAYLEA_E2E_WEB_ORIGIN ?? 'http://127.0.0.1:4173').trim()
if (!/^http:\/\/(?:127\.0\.0\.1|localhost):\d{1,5}$/.test(configuredWebOrigin)) {
  throw new Error('RAYLEA_E2E_WEB_ORIGIN must be a loopback HTTP origin')
}
const allowedWebSocketOrigins = new Set([
  configuredWebOrigin,
  configuredWebOrigin.replace('://127.0.0.1:', '://localhost:'),
])
const externalPreviewImageBytes = Buffer.from(
  'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO2W4n8AAAAASUVORK5CYII=',
  'base64',
)
const bilibiliAvatarUrl = 'http://127.0.0.1:4010/external-preview/avatar.png'
const weiboAvatarUrl = 'https://tvax1.sinaimg.cn/crop.0.0.512.512.180/fixture.jpg'

const helpMenuFontAssetRoot = path.join(repoRoot, 'templates', 'help.menu', 'assets', 'fonts', 'noto-sans-sc')

const externalPreviewFontBytes = await readFile(
  path.join(helpMenuFontAssetRoot, 'k3kXo84MPvpLmixcA63oeALRLoKI.woff2'),
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
  systemDiagnostics: await readFixture('fixtures/web-api/ok.system-diagnostics.yaml'),
  updateStatus: await readFixture('fixtures/web-api/ok.update-status.yaml'),
  updateCheck: await readFixture('fixtures/web-api/ok.update-check.yaml'),
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
  pluginInstallLocalArtifact: await readFixture('fixtures/web-api/ok.plugins-install-local-artifact.yaml'),
  pluginInstallRemoteUrl: await readFixture('fixtures/web-api/ok.plugins-install-remote-url.yaml'),
  pluginList: await readFixture('fixtures/web-api/ok.plugins-list-response.yaml'),
  pluginStoreList: await readFixture('fixtures/web-api/ok.plugin-store-list.yaml'),
  pluginStoreSources: await readFixture('fixtures/web-api/ok.plugin-store-sources.yaml'),
  pluginStoreInspection: await readFixture('fixtures/web-api/ok.plugin-store-inspection.yaml'),
  pluginStoreSourceRefresh: await readFixture('fixtures/web-api/ok.plugin-store-source-refresh.yaml'),
  pluginDetail: await readFixture('fixtures/web-api/ok.plugin-detail-response.yaml'),
  pluginDetailManagementUI: await readFixture('fixtures/web-api/ok.plugin-detail-response.management-ui.yaml'),
  pluginSettings: await readFixture('fixtures/web-api/ok.plugin-settings-response.yaml'),
  pluginSettingsUpdate: await readFixture('fixtures/web-api/ok.plugin-settings-update-response.yaml'),
  pluginUninstallAccepted: await readFixture('fixtures/web-api/ok.plugins-uninstall-accepted.yaml'),
  invalidUninstallId: await readFixture('fixtures/web-api/invalid.plugins-uninstall-id.yaml'),
  governanceBlacklist: await readFixture('fixtures/web-api/ok.governance-blacklist-response.yaml'),
  governanceBlacklistEntryUpsert: await readFixture('fixtures/web-api/ok.governance-blacklist-entry-upsert.yaml'),
  governanceWhitelist: await readFixture('fixtures/web-api/ok.governance-whitelist-response.yaml'),
  governanceWhitelistState: await readFixture('fixtures/web-api/ok.governance-whitelist-state-response.yaml'),
  governanceWhitelistEntryUpsert: await readFixture('fixtures/web-api/ok.governance-whitelist-entry-upsert.yaml'),
  governanceCommandPolicy: await readFixture('fixtures/web-api/ok.governance-command-policy-response.yaml'),
  thirdPartyAccounts: await readFixture('fixtures/web-api/ok.third-party-accounts-list.yaml'),
  thirdPartyAccountUpsert: await readFixture('fixtures/web-api/ok.third-party-account-upsert.yaml'),
  thirdPartyAccountValidateInvalid: await readFixture('fixtures/web-api/ok.third-party-account-validate-invalid.yaml'),
  thirdPartyAccountValidateNotFound: await readFixture('fixtures/web-api/invalid.third-party-account-validate-not-found.yaml'),
  thirdPartyQRCodeCreateBilibili: await readFixture('fixtures/web-api/ok.third-party-login-qrcode-create-bilibili.yaml'),
  thirdPartyQRCodePollBilibiliPending: await readFixture('fixtures/web-api/ok.third-party-login-qrcode-poll-bilibili-pending.yaml'),
  thirdPartyQRCodePollBilibiliSucceeded: await readFixture('fixtures/web-api/ok.third-party-login-qrcode-poll-bilibili-succeeded.yaml'),
  thirdPartyQRCodeCreateWeibo: await readFixture('fixtures/web-api/ok.third-party-login-qrcode-create-weibo.yaml'),
  thirdPartyQRCodePollWeiboPending: await readFixture('fixtures/web-api/ok.third-party-login-qrcode-poll-weibo-pending.yaml'),
  thirdPartyQRCodePollWeiboSucceeded: await readFixture('fixtures/web-api/ok.third-party-login-qrcode-poll-weibo-succeeded.yaml'),
  thirdPartyQRCodeCreateDouyin: await readFixture('fixtures/web-api/ok.third-party-login-qrcode-create-douyin.yaml'),
  thirdPartyQRCodePollDouyinPending: await readFixture('fixtures/web-api/ok.third-party-login-qrcode-poll-douyin-pending.yaml'),
  thirdPartyQRCodePollDouyinVerificationRequired: await readFixture('fixtures/web-api/ok.third-party-login-qrcode-poll-douyin-verification-required.yaml'),
  thirdPartyQRCodePollDouyinFailed: await readFixture('fixtures/web-api/edge.third-party-login-qrcode-poll-douyin-failed.yaml'),
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
    adminIdentifier: 'admin',
    adminSecret: 'fixture-only-secret',
    token: null,
    csrfToken: null,
    plugins: pluginMap,
    pluginStoreSources: structuredClone(fixtures.pluginStoreSources.response.body.items),
    pluginStoreInstalled: {},
    taskStatuses: {},
    pluginSettings: {
      'example-config-panel': structuredClone(fixtures.pluginSettings.response.body.values),
    },
    pluginSecrets: {
      'example-config-panel': { api_key: 'stored-secret-must-not-leak' },
    },
    pluginInstallInspections: {},
    logs: initialLogs,
    currentSessionLogIds: new Set(initialLogs.map((item) => item.log_id)),
    logDetails: createLogDetailMap(),
    config: structuredClone(fixtures.configGet.response.body.config),
    configRevision: fixtures.configGet.response.body.revision,
    configApplyEffects: { applied_now: [], reloaded_now: [], restart_required_fields: [] },
    effectiveTimezone: 'Asia/Shanghai',
    loadedAdapterIds: fixtures.configGet.response.body.config.adapters.map((entry) => entry.id),
    governanceBlacklist: structuredClone(fixtures.governanceBlacklist.response.body),
    governanceWhitelist: structuredClone(fixtures.governanceWhitelist.response.body),
    governanceCommandPolicy: structuredClone(fixtures.governanceCommandPolicy.response.body),
    thirdPartyAccounts,
    thirdPartyQRCodePolls: {},
    thirdPartyQRCodeExpiresAt: {},
    renderTemplates: createRenderTemplateState(),
    schedulerJobs: structuredClone(fixtures.schedulerJobsList.response.body.items),
    systemStatus: structuredClone(fixtures.systemStatus.response.body),
    failures: {
      failPluginsListOnce: false,
      failPluginDetailOnce: false,
      failLogsOnce: false,
      failSystemStatusOnce: false,
      failUninstallOnce: false,
      failThirdPartyQRCodePollOnce: false,
      douyinQRCodeVerificationRequired: false,
      douyinQRCodeFailed: false,
    },
    networkOffline: false,
  }
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
        verificationRequired: fixtures.thirdPartyQRCodePollDouyinVerificationRequired,
        failed: fixtures.thirdPartyQRCodePollDouyinFailed,
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
        preview_data_json: item.id === helpDetail.id ? structuredClone(helpDetail.preview_data_json) : { title: item.name },
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
        name: template.detail.name,
        description: template.detail.description,
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
    source_digest: 'a'.repeat(64),
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

function collectionPage(items, params, text = item => JSON.stringify(item), filter = () => true, compare = (a, b) => String(a.id).localeCompare(String(b.id))) {
  const query = (params.get('query') || '').trim().toLowerCase()
  const selected = items.filter(item => filter(item) && (!query || text(item).toLowerCase().includes(query))).sort(compare)
  const offset = Number(params.get('cursor') || 0)
  const limit = Math.min(100, Number(params.get('limit') || 100))
  const page = selected.slice(offset, offset + limit)
  return { items: structuredClone(page), total: selected.length, ...(offset + page.length < selected.length ? { next_cursor: String(offset + page.length) } : {}) }
}

function governancePage(value, params) {
  const all = [...value.user_entries, ...value.group_entries]
  const page = collectionPage(all, params, entry => [entry.target_id, entry.reason].join(' '), entry => !params.get('entry_type') || entry.entry_type === params.get('entry_type'), (a,b) => b.created_at.localeCompare(a.created_at) || b.target_id.localeCompare(a.target_id))
  const { items, ...metadata } = page
  return { ...value, ...metadata, entry_count: all.length, user_entries: items.filter(item => item.entry_type === 'user'), group_entries: items.filter(item => item.entry_type === 'group') }
}

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

function cookieValue(request, name) {
  const header = request.headers.cookie ?? ''
  for (const part of header.split(';')) {
    const [key, ...valueParts] = part.trim().split('=')
    if (key === name) return decodeURIComponent(valueParts.join('='))
  }
  return null
}

function isSafeMethod(method) {
  return ['GET', 'HEAD', 'OPTIONS', 'TRACE'].includes(String(method ?? 'GET').toUpperCase())
}

function requireAuth(request, response) {
  const bearer = authToken(request)
  if (bearer && bearer === state.token) {
    return true
  }

  const cookie = cookieValue(request, 'raylea_session')
  if (cookie && cookie === state.token) {
    response.setHeader('X-Raylea-CSRF', state.csrfToken)
    if (isSafeMethod(request.method) || request.headers['x-raylea-csrf'] === state.csrfToken) {
      return true
    }
    json(response, 403, errorEnvelope('permission.denied', '请求缺少有效的 CSRF 凭据', 'req_csrf_missing_fixture'))
    return false
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
  if (typeof payload.timezone === 'string') {
    state.effectiveTimezone = payload.timezone
    state.config.scheduler.timezone = payload.timezone
    for (const job of state.schedulerJobs) job.timezone = payload.timezone
  }
  state.initialized = Boolean(payload.initialized)
  if (payload.config_apply_effects) state.configApplyEffects = structuredClone(payload.config_apply_effects)
  state.token = null
  state.csrfToken = null
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
    command_groups: structuredClone(plugin.command_groups ?? []),
    help: structuredClone(plugin.help ?? {}),
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
  if (plugin.icon) {
    summary.icon = plugin.icon
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

  if (!['user', 'group'].includes(entryType) || !targetId || !reason || !payload.scope) {
    return null
  }

  return {
    scope: structuredClone(payload.scope),
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
  const existing = collection.find((entry) => entry.target_id === payload.target_id && sameScope(entry.scope, payload.scope))

  if (existing) {
    existing.reason = payload.reason
    return structuredClone(existing)
  }

  const entry = {
    scope: structuredClone(payload.scope),
    entry_type: payload.entry_type,
    target_id: payload.target_id,
    reason: payload.reason,
    created_at: governanceEntryCreatedAt(collectionName),
  }
  collection.push(entry)
  collection.sort((left, right) => left.target_id.localeCompare(right.target_id))
  return structuredClone(entry)
}

function removeGovernanceEntry(snapshot, entryType, targetId, scope) {
  const collection = governanceEntryCollection(snapshot, entryType)
  const index = collection.findIndex((entry) => entry.target_id === targetId && sameScope(entry.scope, scope))
  if (index < 0) {
    return false
  }
  collection.splice(index, 1)
  return true
}

function mergePluginState(pluginId, patch) {
  const previous = state.plugins[pluginId] ?? {}
  state.plugins[pluginId] = {
    ...structuredClone(previous),
    ...structuredClone(patch),
    source: structuredClone(patch.source ?? previous.source),
    trust: structuredClone(patch.trust ?? previous.trust),
    commands: structuredClone(patch.commands ?? previous.commands ?? []),
    command_groups: structuredClone(patch.command_groups ?? previous.command_groups ?? []),
    help: structuredClone(patch.help ?? previous.help ?? {}),
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
  const pluginRoot = getPluginManagementUIRoot(pluginId)
  if (!plugin || !pluginRoot) {
    return null
  }

  const normalizedRequestPath = (requestedPath || 'index.html')
    .split('/')
    .map((segment) => segment.trim())
    .filter((segment) => segment.length > 0)
    .join('/')
  if (!normalizedRequestPath) {
    return null
  }

  const allowedDirectory = path.resolve(pluginRoot)
  const resolvedFilePath = path.resolve(allowedDirectory, normalizedRequestPath)
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
    case '.webp':
      return 'image/webp'
    case '.woff2':
      return 'font/woff2'
    default:
      return 'application/octet-stream'
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

  if (state.networkOffline && !pathname.startsWith('/__test/')) {
    response.writeHead(503, {
      'Content-Type': 'application/json',
      'x-rayleabot-backend-unavailable': '1',
    })
    response.end(JSON.stringify(errorEnvelope('platform.unavailable', 'mock backend unavailable', 'req_mock_backend_unavailable')))
    return
  }

  if (String(request.headers.host ?? '').toLowerCase() === exampleConfigPanelHost) {
    if ((request.method !== 'GET' && request.method !== 'HEAD') || pathname.startsWith('/api/') || pathname.startsWith('/ws/')) {
      json(response, 404, errorEnvelope('platform.resource_missing', 'plugin origin has no API routes', 'req_plugin_ui_isolated'))
      return
    }
    let requestedPath = ''
    try {
      requestedPath = pathname.split('/').filter(Boolean).map((segment) => decodeURIComponent(segment)).join('/')
    } catch {
      json(response, 404, errorEnvelope('platform.resource_missing', 'plugin management page not found', 'req_plugin_ui_invalid_path'))
      return
    }
    const filePath = resolvePluginManagementUIFile('example-config-panel', requestedPath)
    if (!filePath) {
      json(response, 404, errorEnvelope('platform.resource_missing', 'plugin management page not found', 'req_plugin_ui_not_found'))
      return
    }
    try {
      const file = await readFile(filePath)
      response.writeHead(200, {
        'Content-Type': getContentType(filePath),
        'Cache-Control': 'no-store, max-age=0',
        'Content-Security-Policy': `default-src 'none'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; font-src 'self'; connect-src 'none'; object-src 'none'; base-uri 'none'; form-action 'none'; frame-ancestors ${configuredWebOrigin}`,
        'X-Content-Type-Options': 'nosniff',
        'Referrer-Policy': 'no-referrer',
      })
      response.end(request.method === 'HEAD' ? undefined : file)
      return
    } catch {
      json(response, 404, errorEnvelope('platform.resource_missing', 'plugin management page not found', 'req_plugin_ui_missing'))
      return
    }
  }

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
    state.csrfToken = null
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
    const cookieTransport = request.headers['x-raylea-session-transport'] === 'cookie'
    if (cookieTransport) {
      state.csrfToken = 'fixture-csrf-token-setup'
      response.setHeader('Set-Cookie', `raylea_session=${encodeURIComponent(state.token)}; Path=/; HttpOnly; SameSite=Strict`)
      response.setHeader('X-Raylea-Session-Transport', 'cookie')
      response.setHeader('X-Raylea-CSRF', state.csrfToken)
      json(response, fixtures.setupAdmin.response.status, {
        transport: 'cookie',
        csrf_token: state.csrfToken,
        expires_at: fixtures.setupAdmin.response.body.expires_at,
      })
      return
    }
    json(response, fixtures.setupAdmin.response.status, fixtures.setupAdmin.response.body)
    return
  }

  if (pathname === '/api/session/login' && request.method === 'POST') {
    const payload = await parseBody(request)
    if (!state.initialized || payload.identifier !== state.adminIdentifier || payload.secret !== state.adminSecret) {
      json(response, fixtures.sessionDenied.response.status, fixtures.sessionDenied.response.body)
      return
    }

    state.token = fixtures.sessionLogin.response.body.session_token
    const cookieTransport = request.headers['x-raylea-session-transport'] === 'cookie'
    if (cookieTransport) {
      state.csrfToken = 'fixture-csrf-token-login'
      response.setHeader('Set-Cookie', `raylea_session=${encodeURIComponent(state.token)}; Path=/; HttpOnly; SameSite=Strict`)
      response.setHeader('X-Raylea-Session-Transport', 'cookie')
      response.setHeader('X-Raylea-CSRF', state.csrfToken)
      json(response, fixtures.sessionLogin.response.status, {
        transport: 'cookie',
        csrf_token: state.csrfToken,
        expires_at: fixtures.sessionLogin.response.body.expires_at,
      })
      return
    }
    json(response, fixtures.sessionLogin.response.status, fixtures.sessionLogin.response.body)
    return
  }

  if (pathname === '/api/account/credentials' && request.method === 'PUT') {
    if (!requireAuth(request, response)) return
    const payload = await parseBody(request)
    if (payload.current_secret !== state.adminSecret) {
      json(response, 403, errorEnvelope('permission.current_secret_invalid', '当前密码不正确', 'req_account_credentials_fixture'))
      return
    }
    if (typeof payload.new_secret !== 'string' || Array.from(payload.new_secret).length < 8) {
      json(response, 400, errorEnvelope('platform.invalid_request', '请求参数不合法', 'req_account_credentials_fixture'))
      return
    }
    state.adminSecret = payload.new_secret
    state.adminIdentifier = payload.new_identifier?.trim() || state.adminIdentifier
    state.token = null
    state.csrfToken = null
    broadcast('events', fixtures.wsSessionExpired.frame)
    closeAllSockets()
    // Exercise session invalidation arriving before the pending HTTP response.
    await new Promise(resolve => setTimeout(resolve, 50))
    response.setHeader('Set-Cookie', 'raylea_session=; Path=/; HttpOnly; SameSite=Strict; Max-Age=0')
    noContent(response)
    return
  }

  if (pathname === '/api/session' && request.method === 'DELETE') {
    if (!requireAuth(request, response)) {
      return
    }

    state.token = null
    state.csrfToken = null
    response.setHeader('Set-Cookie', 'raylea_session=; Path=/; HttpOnly; SameSite=Strict; Max-Age=0')
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

    json(response, fixtures.systemStatus.response.status, { ...state.systemStatus, adapters: adapterDescriptors().map(({ id, protocol, enabled, state }) => ({ id, protocol, enabled, state })) })
    return
  }

  if (pathname === '/api/system/diagnostics' && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }
    json(response, fixtures.systemDiagnostics.response.status, fixtures.systemDiagnostics.response.body)
    return
  }

  if (pathname === '/api/governance/blacklist' && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    json(response, 200, governancePage(state.governanceBlacklist, searchParams))
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
      json(response, 404, errorEnvelope('platform.resource_missing', 'governance entry not found', 'req_governance_blacklist_entry_not_found'))
      return
    }

    if (!removeGovernanceEntry(state.governanceBlacklist, entryType, targetId, Object.fromEntries(searchParams))) {
      json(response, 404, errorEnvelope('platform.resource_missing', 'governance entry not found', 'req_governance_blacklist_entry_not_found'))
      return
    }

    noContent(response)
    return
  }

  if (pathname === '/api/governance/whitelist' && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    json(response, 200, governancePage(state.governanceWhitelist, searchParams))
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
      json(response, 404, errorEnvelope('platform.resource_missing', 'governance entry not found', 'req_governance_whitelist_entry_not_found'))
      return
    }

    if (!removeGovernanceEntry(state.governanceWhitelist, entryType, targetId, Object.fromEntries(searchParams))) {
      json(response, 404, errorEnvelope('platform.resource_missing', 'governance entry not found', 'req_governance_whitelist_entry_not_found'))
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

    json(response, 200, collectionPage(listRenderTemplates().items, searchParams, item => [item.id, item.name, item.description, item.source?.plugin_id, state.plugins[item.source?.plugin_id]?.name].join(' ')))
    return
  }

  if (pathname.startsWith('/api/system/render/templates/') && pathname.endsWith('/asset') && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    const assetPath = searchParams.get('path') ?? ''
    const fontAssetMatch = assetPath.match(/^assets\/fonts\/noto-sans-sc\/([A-Za-z0-9._-]+\.woff2)$/)
    if (fontAssetMatch) {
      response.writeHead(200, {
        'Content-Type': 'font/woff2',
        'Cache-Control': 'no-store',
      })
      response.end(await readFile(path.join(helpMenuFontAssetRoot, fontAssetMatch[1])))
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

    json(response, 200, collectionPage(state.schedulerJobs, searchParams, item => [item.job_id,item.plugin_id,item.plugin_name,item.task_name,item.log_label,item.payload_summary?.content].join(' '), item => searchParams.get('status') === 'error' ? Boolean(item.last_error) : searchParams.get('status') === 'success' ? !item.last_error : true, (a,b) => {
      const fallback = a.plugin_id.localeCompare(b.plugin_id) || a.job_id.localeCompare(b.job_id)
      if (searchParams.get('sort') === 'last_run') return Date.parse(b.last_run || '1970-01-01') - Date.parse(a.last_run || '1970-01-01') || fallback
      if (searchParams.get('sort') === 'duration') return (b.last_duration_ms || 0) - (a.last_duration_ms || 0) || fallback
      return a.plugin_name.localeCompare(b.plugin_name) || a.task_name.localeCompare(b.task_name) || fallback
    }))
    return
  }

  if (pathname.startsWith('/api/system/scheduler/jobs/') && pathname.endsWith('/trigger') && request.method === 'POST') {
    if (!requireAuth(request, response)) {
      return
    }

    const jobId = decodeURIComponent(pathname.split('/')[5] ?? '')
    const job = state.schedulerJobs.find((item) => item.job_id === jobId)
    if (!job) {
      json(response, 404, errorEnvelope('platform.resource_missing', 'scheduler job not found', 'req_scheduler_job_not_found'))
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
      effective_timezone: state.effectiveTimezone,
      revision: state.configRevision,
      redacted_fields: snapshot.redacted_fields,
    })
    return
  }

  if (pathname === '/api/config' && request.method === 'PUT') {
    if (!requireAuth(request, response)) {
      return
    }

    const payload = await parseBody(request)
    state.config = structuredClone(payload)
    state.configRevision += 1
    syncGovernanceCommandPolicyFromConfig(state.config)
    const applyEffects = structuredClone(state.configApplyEffects)
    broadcast('events', {
      channel: 'events',
      type: 'events.received',
      timestamp: new Date().toISOString(),
      data: {
        adapters: adapterDescriptors(),
      },
    })
    const snapshot = redactConfigSecrets(state.config)
    json(response, 200, {
      config: snapshot.config,
      effective_timezone: state.effectiveTimezone,
      redacted_fields: snapshot.redacted_fields,
      restart_required: applyEffects.restart_required_fields.length > 0,
      revision: state.configRevision,
      apply_effects: applyEffects,
    })
    return
  }

  if (pathname === '/api/adapters' && request.method === 'GET') {
    if (!requireAuth(request, response)) return
    json(response, 200, {
      adapters: adapterDescriptors(),
      available_protocols: [
        { protocol: 'onebot11', display_name: 'OneBot11', description: '连接 NapCat 等实现 OneBot11 的协议端。' },
        { protocol: 'qqofficial', display_name: 'QQ 官方机器人', description: '使用 QQ 开放平台的 AppID 和 AppSecret 接入。' },
      ],
    })
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

    json(response, 200, collectionPage(state.thirdPartyAccounts, searchParams, item => [item.platform,item.account_id,item.label,item.profile?.nickname].join(' '), () => true, (a,b) => a.platform.localeCompare(b.platform) || a.account_id.localeCompare(b.account_id)))
    return
  }

  if (/^\/api\/third-party\/accounts\/[^/]+\/[^/]+\/avatar$/.test(pathname) && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    response.writeHead(200, {
      'Content-Type': 'image/png',
      'Cache-Control': 'private, no-store',
      'X-Content-Type-Options': 'nosniff',
    })
    response.end(externalPreviewImageBytes)
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
    if (payload.create_only && previous) { json(response, 409, errorEnvelope('platform.state_conflict', '账号已存在', 'req_account_exists')); return }
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

  if (/^\/api\/third-party\/accounts\/[^/]+\/[^/]+\/validate$/.test(pathname) && request.method === 'POST') {
    if (!requireAuth(request, response)) {
      return
    }

    const segments = pathname.split('/')
    const platform = decodeURIComponent(segments[4] ?? '')
    const accountId = decodeURIComponent(segments[5] ?? '')
    const account = state.thirdPartyAccounts.find((item) => item.platform === platform && item.account_id === accountId)
    if (!account?.configured) {
      json(response, fixtures.thirdPartyAccountValidateNotFound.response.status, structuredClone(fixtures.thirdPartyAccountValidateNotFound.response.body))
      return
    }
    await new Promise((resolve) => setTimeout(resolve, 120))
    const nextAccount = structuredClone(account)
    if (platform === 'weibo') {
      nextAccount.credential = structuredClone(fixtures.thirdPartyAccountValidateInvalid.response.body.account.credential)
    } else {
      nextAccount.credential = {
        state: 'valid',
        checked_at: new Date().toISOString(),
        last_error: '',
      }
    }
    state.thirdPartyAccounts = state.thirdPartyAccounts.map((item) => (
      item.platform === platform && item.account_id === accountId ? nextAccount : item
    ))
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
    const pollKey = thirdPartyQRCodePollKey(platform, body.login_id)
    const expiresAt = new Date(Date.now() + 3 * 60 * 1000).toISOString()
    body.expires_at = expiresAt
    state.thirdPartyQRCodePolls[pollKey] = 0
    state.thirdPartyQRCodeExpiresAt[pollKey] = expiresAt
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
    if (takeFailureFlag('failThirdPartyQRCodePollOnce')) {
      json(response, 502, errorEnvelope('platform.upstream_request_failed', 'third-party qrcode poll failed', 'req_third_party_qr_upstream'))
      return
    }
    state.thirdPartyQRCodePolls[pollKey] += 1
    const pollCount = state.thirdPartyQRCodePolls[pollKey]
    let fixture = pollCount > 1
      ? fixturesForPlatform.succeeded
      : fixturesForPlatform.pending
    if (platform === 'douyin' && state.failures.douyinQRCodeFailed && pollCount > 1) {
      fixture = fixturesForPlatform.failed
    } else if (platform === 'douyin' && state.failures.douyinQRCodeVerificationRequired && pollCount === 2) {
      fixture = fixturesForPlatform.verificationRequired
    }
    const body = structuredClone(fixture.response.body)
    body.expires_at = state.thirdPartyQRCodeExpiresAt[pollKey] ?? body.expires_at
    if (platform === 'douyin' && body.account && !body.account.profile) {
      body.account.profile = {
        uid: 'fixture-douyin-account',
        nickname: '抖音扫码账号',
        avatar_url: 'https://p3-pc-sign.douyinpic.com/fixture/avatar.jpeg',
      }
    }
    if (platform === 'weibo' && body.account?.profile) {
      body.account.profile.avatar_url = weiboAvatarUrl
    }
    if (body.account) {
      const savedAccount = localizeBilibiliAccountAvatar(structuredClone(body.account))
      state.thirdPartyAccounts = [
        ...state.thirdPartyAccounts.filter((item) => item.platform !== savedAccount.platform || item.account_id !== savedAccount.account_id),
        savedAccount,
      ].sort((left, right) => left.account_id.localeCompare(right.account_id))
    }
    json(response, fixture.response.status, {
      ...body,
      login_id: loginId,
    })
    return
  }

  if (/^\/api\/third-party\/accounts\/[^/]+\/login\/qrcode\/[^/]+$/.test(pathname) && request.method === 'DELETE') {
    if (!requireAuth(request, response)) {
      return
    }

    const segments = pathname.split('/')
    const platform = decodeURIComponent(segments[4] ?? '')
    const loginId = decodeURIComponent(segments[7] ?? '')
    if (segments[5] === 'login' && segments[6] === 'qrcode' && platform && loginId) {
      const pollKey = thirdPartyQRCodePollKey(platform, loginId)
      delete state.thirdPartyQRCodePolls[pollKey]
      delete state.thirdPartyQRCodeExpiresAt[pollKey]
      noContent(response)
      return
    }
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

    json(response, 200, listLogPage(state, searchParams))
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

  const iconMatch = pathname.match(/^\/api\/plugins\/([^/]+)\/icon$/)
  if (iconMatch && request.method === 'GET') {
    if (!requireAuth(request, response)) return
    const plugin = state.plugins[decodeURIComponent(iconMatch[1])]
    if (!plugin?.icon) {
      json(response, 404, errorEnvelope('platform.resource_missing', 'icon unavailable', 'req_icon_missing'))
      return
    }
    const bytes = await readFile(path.join(repoRoot, 'examples/plugins/hello-go/assets/icon.svg'))
    response.writeHead(200, {
      'Content-Type': 'image/svg+xml',
      'Cache-Control': 'private, no-store',
      'X-Content-Type-Options': 'nosniff',
      'Content-Security-Policy': "default-src 'none'; style-src 'unsafe-inline'; sandbox",
    })
    response.end(bytes)
    return
  }

  if (pathname === '/api/plugin-store/plugins' && request.method === 'GET') {
    if (!requireAuth(request, response)) return
    const body = structuredClone(fixtures.pluginStoreList.response.body)
    body.items = body.items.map(item => state.pluginStoreInstalled[item.id] ? { ...item, installed_version: state.pluginStoreInstalled[item.id], install_state: 'installed' } : item)
    json(response, 200, body)
    return
  }

  if (pathname === '/api/plugin-store/sources' && request.method === 'GET') {
    if (!requireAuth(request, response)) return
    json(response, 200, collectionPage(state.pluginStoreSources, searchParams, item => [item.id,item.name,item.url].join(' ')))
    return
  }

  const pluginStoreSourceMatch = pathname.match(/^\/api\/plugin-store\/sources\/([^/]+)$/)
  if ((pathname === '/api/plugin-store/sources' && request.method === 'POST')
    || (pluginStoreSourceMatch && ['PUT', 'DELETE'].includes(request.method))) {
    if (!requireAuth(request, response)) return
    const sourceId = pluginStoreSourceMatch ? decodeURIComponent(pluginStoreSourceMatch[1]) : null
    const existing = sourceId ? state.pluginStoreSources.find(item => item.id === sourceId) : null
    if (sourceId && !existing) {
      json(response, 404, errorEnvelope('platform.not_found', 'source not found', 'req_source_missing'))
      return
    }
    if (existing?.official) {
      json(response, 409, errorEnvelope('platform.invalid_request', 'official source is immutable', 'req_source_immutable'))
      return
    }
    if (request.method === 'DELETE') {
      state.pluginStoreSources = state.pluginStoreSources.filter(item => item.id !== sourceId)
      response.writeHead(204)
      response.end()
      return
    }
    const input = await parseBody(request)
    if (typeof input.name !== 'string' || !input.name.trim() || input.name.length > 120
      || typeof input.url !== 'string' || !input.url.startsWith('https://')) {
      json(response, 400, errorEnvelope('platform.invalid_request', 'invalid source', 'req_source_invalid'))
      return
    }
    const item = {
      id: sourceId || 'custom-' + createHash('sha256').update(input.url).digest('hex').slice(0, 12),
      name: input.name.trim(), url: input.url, official: false, cached: true,
      entry_count: 0, refreshed_at: '2026-09-07T00:00:00Z',
    }
    state.pluginStoreSources = [...state.pluginStoreSources.filter(source => source.id !== item.id), item]
    json(response, request.method === 'POST' ? 201 : 200, item)
    return
  }

  const pluginStoreInspectMatch = pathname.match(/^\/api\/plugin-store\/plugins\/([^/]+)\/inspect$/)
  if (pluginStoreInspectMatch && request.method === 'POST') {
    if (!requireAuth(request, response)) return
    const payload = await parseBody(request)
    if (payload.source_id !== 'official') {
      json(response, 400, errorEnvelope('platform.invalid_request', 'invalid plugin source', 'req_plugin_store_inspect_invalid'))
      return
    }
    json(response, 200, structuredClone(fixtures.pluginStoreInspection.response.body))
    return
  }

  const pluginStoreRefreshMatch = pathname.match(/^\/api\/plugin-store\/sources\/([^/]+)\/refresh$/)
  if (pluginStoreRefreshMatch && request.method === 'POST') {
    if (!requireAuth(request, response)) return
    json(response, 200, structuredClone(fixtures.pluginStoreSourceRefresh.response.body))
    return
  }

  const pluginStoreInstallMatch = pathname.match(/^\/api\/plugin-store\/plugins\/([^/]+)\/install$/)
  if (pluginStoreInstallMatch && request.method === 'POST') {
    if (!requireAuth(request, response)) return
    const payload = await parseBody(request)
    const inspection = fixtures.pluginStoreInspection.response.body.inspection
    if (
      payload.inspection_id !== inspection.inspection_id
      || payload.package_sha256 !== inspection.package_sha256
      || payload.trusted_code_confirmed !== true
    ) {
      json(response, 409, errorEnvelope('plugin.install_inspection_required', 'plugin inspection is required', 'req_plugin_store_install_inspection'))
      return
    }
    const accepted = structuredClone(fixtures.pluginInstallAccepted.response.body)
    const pluginId = decodeURIComponent(pluginStoreInstallMatch[1])
    state.pluginStoreInstalled[pluginId] = fixtures.pluginStoreList.response.body.items.find(item => item.id === pluginId)?.latest_release?.version
    state.taskStatuses[accepted.task_id] = { task_id: accepted.task_id, status: 'succeeded' }
    json(response, fixtures.pluginInstallAccepted.response.status, accepted)
    return
  }

  const taskStatusMatch = pathname.match(/^\/api\/system\/tasks\/([^/]+)$/)
  if (taskStatusMatch && request.method === 'GET') {
    if (!requireAuth(request, response)) return
    const status = state.taskStatuses[decodeURIComponent(taskStatusMatch[1])]
    if (!status) { json(response, 404, errorEnvelope('platform.resource_missing', 'task not found', 'req_task_missing')); return }
    json(response, 200, status)
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

    json(response, 200, collectionPage(pluginListBody().items, searchParams, item => [item.id,item.name,item.description].join(' '), item => {
      const status = searchParams.get('state'); const source = searchParams.get('source')
      if (status === 'alert' && item.state !== 'failed' && item.state !== 'invalid' && !item.command_conflicts?.length) return false
      if ((status === 'running' || status === 'disabled') && item.state !== status) return false
      if (source === 'official' && item.trust?.level !== 'official') return false
      if (source === 'community' && item.trust?.level === 'official') return false
      return true
    }))
    return
  }

  if (pathname === '/api/update/status' && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }
    json(response, fixtures.updateStatus.response.status, structuredClone(fixtures.updateStatus.response.body))
    return
  }

  if (pathname === '/api/update/check' && request.method === 'POST') {
    if (!requireAuth(request, response)) {
      return
    }
    json(response, fixtures.updateCheck.response.status, structuredClone(fixtures.updateCheck.response.body))
    return
  }

  if (pathname === '/api/plugins/install/inspect' && request.method === 'POST') {
    if (!requireAuth(request, response)) {
      return
    }

    const payload = await parseBody(request)
    if (!['local_zip', 'local_directory', 'remote_url'].includes(payload.source_type) || typeof payload.source !== 'string' || !payload.source.trim()) {
      json(response, 400, errorEnvelope('platform.invalid_request', 'invalid plugin source', 'req_plugin_inspect_invalid'))
      return
    }
    const inspectionId = 'inspection_mock_weather_package_0000001'
    const packageSha256 = 'a'.repeat(64)
    state.pluginInstallInspections[inspectionId] = {
      source_type: payload.source_type,
      source: payload.source,
      package_sha256: packageSha256,
    }
    json(response, 200, {
      inspection_id: inspectionId,
      expires_at: '2026-07-10T12:15:00Z',
      package_sha256: packageSha256,
      source: {
        source_type: payload.source_type,
        source: payload.source,
      },
      plugin: {
        id: 'example.weather-package',
        name: 'Weather Package',
        version: '1.0.0',
        author: 'example',
        license: 'MIT',
        source_label: payload.source_type === 'remote_url' ? 'example.com' : '本地插件包',
      },
      permissions: {
        'http.request': {},
        'message.send': {},
      },
      target_platform: 'windows-x64',
      backend: {
        entry: 'bin/weather',
        path: 'bin/weather.exe',
        size: 3145728,
      },
      ui: {
        enabled: true,
        entry: 'ui/index.html',
        file_count: 4,
      },
      artifact: {
        valid: true,
        artifact_version: '2',
        file_count: 8,
      },
    })
    return
  }

  if (pathname === '/api/plugins/install' && request.method === 'POST') {
    if (!requireAuth(request, response)) {
      return
    }

    const payload = await parseBody(request)
    const inspection = state.pluginInstallInspections[payload.inspection_id]
    if (
      !inspection
      || payload.trusted_code_confirmed !== true
      || payload.package_sha256 !== inspection.package_sha256
    ) {
      json(response, 409, errorEnvelope('plugin.install_inspection_required', 'plugin inspection is required', 'req_plugin_install_inspection'))
      return
    }
    delete state.pluginInstallInspections[payload.inspection_id]
    let taskId = fixtures.pluginInstallAccepted.response.body.task_id

    if (inspection.source_type === 'remote_url') {
      taskId = fixtures.pluginInstallRemoteUrl.response.body.task_id
    } else if (inspection.source.includes('local-artifact')) {
      taskId = fixtures.pluginInstallLocalArtifact.response.body.task_id
    }

    appendTaskLog(taskId, 'plugin.install', 'pending', `install ${inspection.source}`, {
      plugin_id: inspection.source_type === 'remote_url' ? undefined : 'weather',
    })

    state.taskStatuses[taskId] = { task_id: taskId, status: 'succeeded' }

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

  if (pathname.match(/^\/api\/plugins\/[^/]+\/management\/actions$/) && request.method === 'POST') {
    if (!requireAuth(request, response)) {
      return
    }
    const pluginId = decodeURIComponent(pathname.split('/')[3])
    const payload = await parseBody(request)
    json(response, 200, {
      plugin_id: pluginId,
      action: String(payload.action ?? ''),
      result: { handled: true },
    })
    return
  }

  if (pathname.startsWith('/api/plugins/') && pathname.endsWith('/settings') && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }

    const pluginId = pathname.split('/')[3]
    const settingsBody = pluginSettingsBody(pluginId)
    if (!settingsBody) {
      json(response, 404, errorEnvelope('platform.resource_missing', 'plugin settings not found', 'req_plugin_settings_not_found'))
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
      json(response, 404, errorEnvelope('platform.resource_missing', 'plugin settings not found', 'req_plugin_settings_not_found'))
      return
    }

    json(response, 200, updatedBody)
    return
  }

  if (pathname.startsWith('/api/plugins/') && pathname.endsWith('/secrets') && request.method === 'GET') {
    if (!requireAuth(request, response)) {
      return
    }
    const pluginId = pathname.split('/')[3]
    if (!state.plugins[pluginId]) {
      json(response, 404, errorEnvelope('platform.resource_missing', 'plugin secrets not found', 'req_plugin_secrets_not_found'))
      return
    }
    const configured = Object.fromEntries(Object.entries(state.pluginSecrets[pluginId] ?? {}).map(([key, value]) => [key, Boolean(value)]))
    json(response, 200, { plugin_id: pluginId, configured })
    return
  }

  if (pathname.startsWith('/api/plugins/') && pathname.endsWith('/secrets') && request.method === 'PUT') {
    if (!requireAuth(request, response)) {
      return
    }
    const pluginId = pathname.split('/')[3]
    const payload = await parseBody(request)
    if (!state.plugins[pluginId] || !payload?.values || typeof payload.values !== 'object' || Array.isArray(payload.values)) {
      json(response, 400, errorEnvelope('platform.invalid_request', 'plugin secrets payload is invalid', 'req_plugin_secrets_invalid'))
      return
    }
    const previous = state.pluginSecrets[pluginId] ?? {}
    const next = { ...previous, ...payload.values }
    state.pluginSecrets[pluginId] = next
    json(response, 200, {
      plugin_id: pluginId,
      changed_keys: Object.keys(payload.values).sort(),
      configured: Object.fromEntries(Object.entries(next).map(([key, value]) => [key, Boolean(value)])),
    })
    return
  }

  if (pathname.startsWith('/api/plugins/') && pathname.endsWith('/secrets') && request.method === 'DELETE') {
    if (!requireAuth(request, response)) {
      return
    }
    const pluginId = pathname.split('/')[3]
    const payload = await parseBody(request)
    if (!state.plugins[pluginId] || !Array.isArray(payload?.keys) || payload.keys.length === 0) {
      json(response, 400, errorEnvelope('platform.invalid_request', 'plugin secret delete payload is invalid', 'req_plugin_secrets_delete_invalid'))
      return
    }
    const next = { ...(state.pluginSecrets[pluginId] ?? {}) }
    for (const key of payload.keys) delete next[key]
    state.pluginSecrets[pluginId] = next
    json(response, 200, {
      plugin_id: pluginId,
      changed_keys: [...new Set(payload.keys)].sort(),
      configured: Object.fromEntries(Object.entries(next).map(([key, value]) => [key, Boolean(value)])),
    })
    return
  }

  if (pathname.startsWith('/api/plugins/') && request.method === 'DELETE') {
    if (!requireAuth(request, response)) {
      return
    }

    const pluginId = pathname.split('/')[3]
    if (!/^[a-z0-9](?:[a-z0-9._-]{0,62}[a-z0-9])?$/.test(pluginId)) {
      json(response, fixtures.invalidUninstallId.response.status, fixtures.invalidUninstallId.response.body)
      return
    }
    if (takeFailureFlag('failUninstallOnce')) {
      json(response, 500, errorEnvelope('platform.internal_error', 'uninstall acceptance failed', 'req_plugin_uninstall_failed'))
      return
    }

    const taskId = fixtures.pluginUninstallAccepted.response.body.task_id
    appendTaskLog(taskId, 'plugin.uninstall', 'pending', `uninstall ${pluginId}`, {
      plugin_id: pluginId,
    })
    delete state.plugins[pluginId]
    state.taskStatuses[taskId] = { task_id: taskId, status: 'succeeded' }
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

  if (webDistRoot && request.method === 'GET' && !pathname.startsWith('/api/') && !pathname.startsWith('/ws/')) {
    const candidates = [path.resolve(webDistRoot, pathname.slice(1)), path.join(webDistRoot, 'index.html')]
    for (const filePath of candidates) {
      if (!isPathInside(webDistRoot, filePath)) continue
      try {
        const file = await readFile(filePath)
        response.writeHead(200, { 'Content-Type': getContentType(filePath), 'Cache-Control': 'no-store' })
        response.end(file)
        return
      } catch { /* Match the Server's SPA fallback, including namespaced plugin IDs. */ }
    }
  }

  json(response, 404, {
    error: {
      code: 'platform.resource_missing',
      message: 'mock route not found',
      message_key: 'errors.platform.resource_missing',
      request_id: 'req_mock_not_found',
    },
  })
})

const wsServer = new WebSocketServer({ noServer: true })

wsServer.on('connection', (socket, request) => {
  const url = requestUrl(request)
  const pathname = url.pathname

  const channel = pathname.startsWith('/ws/plugins/') ? 'plugin_console' : pathname.replace('/ws/', '')

  const token = cookieValue(request, 'raylea_session')
  const origin = String(request.headers.origin ?? '')
  const validOrigin = allowedWebSocketOrigins.has(origin)

  if (url.searchParams.has('session_token') || !validOrigin || !token || token !== state.token || !sockets[channel]) {
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
        adapters: adapterDescriptors(),
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

function sameScope(a, b) {
 return a.kind === b.kind && a.source_protocol === b.source_protocol && a.source_adapter === (b.source_adapter ?? '') && a.bot_id === (b.bot_id ?? '')
}

function adapterDescriptors() {
  return state.config.adapters.map((entry) => ({
    id: entry.id,
    protocol: entry.type,
    display_name: `${entry.type === 'onebot11' ? 'OneBot11' : 'QQ 官方机器人'}（${entry.id}）`,
    enabled: entry.enabled,
    state: !state.loadedAdapterIds.includes(entry.id) || !entry.enabled ? 'stopped' : 'connecting',
    summary: !state.loadedAdapterIds.includes(entry.id) ? '重启后启动此连接。' : entry.enabled ? '正在连接协议端。' : '此连接已配置，当前未启用。',
    ...(entry.type === 'onebot11' && state.loadedAdapterIds.includes(entry.id) ? { onebot11: structuredClone(fixtures.protocolSnapshot.response.body.adapters[0].onebot11) } : {}),
  }))
}
