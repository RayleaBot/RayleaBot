<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'

import { useToastFeedback } from '@/adapter/feedback'
import RetryPanel from '@/components/RetryPanel.vue'
import { t } from '@/i18n'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { ApiError, apiRequest } from '@/lib/http'
import { buildRenderTemplateLocation } from '@/lib/management-links'
import { useMotionNavigation } from '@/motion/useMotionNavigation'
import { useConfigStore } from '@/stores/config'
import { useGovernanceStore } from '@/stores/governance'
import { usePluginsStore } from '@/stores/plugins'
import { useUiShellStore } from '@/stores/ui-shell'
import type { PluginDetail, PluginSettingsUpdateRequest, SchedulerJobTriggerResponse } from '@/types/api'

interface PluginManagementUIPage {
  id: string
  label: string
  entry: string
}

interface BridgeMessage {
  version: '2'
  source: 'plugin_management_ui' | 'management_host'
  type: string
  request_id?: string
  payload?: unknown
}

interface PluginSecretsResponse {
  plugin_id: string
  configured: Record<string, boolean>
}

interface PluginSecretsUpdateResponse extends PluginSecretsResponse {
  changed_keys: string[]
}

interface OneBot11TargetIssue {
  scope: 'protocol' | 'groups' | 'private_users' | 'identity'
  message: string
}

interface OneBot11GroupTarget {
  target_type: 'group'
  target_id: string
  target_name: string
  avatar_url?: string
}

interface OneBot11PrivateTarget {
  target_type: 'private'
  target_id: string
  nickname: string
  avatar_url?: string
}

interface OneBot11ProtocolTargetsResponse {
  protocol: 'onebot11'
  available: boolean
  groups: OneBot11GroupTarget[]
  private_users: OneBot11PrivateTarget[]
  issues: OneBot11TargetIssue[]
}

interface OneBot11IdentityResolveItem {
  target_type: 'group' | 'private'
  target_id: string
  user_id: string
}

interface OneBot11IdentityResolveResponse {
  items: Array<Record<string, unknown>>
  issues: OneBot11TargetIssue[]
}

interface PluginManagementActionResponse {
  plugin_id: string
  action: string
  result: Record<string, unknown>
}

const props = defineProps<{
  plugin: PluginDetail
  title: string
  page: PluginManagementUIPage
}>()

const pluginsStore = usePluginsStore()
const governanceStore = useGovernanceStore()
const configStore = useConfigStore()
const uiShellStore = useUiShellStore()
const navigate = useMotionNavigation()

const iframeRef = ref<HTMLIFrameElement | null>(null)
const iframeKey = ref(0)
const reportedIframeHeight = ref(640)
const iframeHeight = ref(640)
const bridgeNonce = ref('')
const pluginOrigin = ref('')
const confirmed = ref(false)
const waitingForReady = ref(false)
const fatalError = ref<string | null>(null)
const actionError = ref<string | null>(null)
let restartFrameWhenRuntimeReady = props.plugin.state === 'starting'

const managementEntry = computed(() => props.page.entry.trim())
const requiresConfirmation = computed(() => props.plugin.trust?.level === 'unverified')
const confirmationStorageKey = computed(() => (
  `rayleabot.plugin-management-ui.confirmed:${props.plugin.id}:${props.plugin.version ?? ''}:${props.plugin.source?.package_source_ref ?? ''}`
))
const canRenderIframe = computed(() => Boolean(frameSrc.value) && (!requiresConfirmation.value || confirmed.value))
const frameSrc = computed(() => {
  if (!pluginOrigin.value || !bridgeNonce.value || !managementEntry.value.startsWith('ui/')) {
    return ''
  }
  const relativeEntry = managementEntry.value.slice('ui/'.length)
  const url = new URL(relativeEntry.split('/').map(encodeURIComponent).join('/'), `${pluginOrigin.value}/`)
  url.searchParams.set('bridge_nonce', bridgeNonce.value)
  url.searchParams.set('version', props.plugin.version ?? '')
  url.searchParams.set('frame', String(iframeKey.value))
  return url.toString()
})
const frameOrigin = computed(() => pluginOrigin.value)
const busy = computed(() => waitingForReady.value || Boolean(pluginsStore.settingsLoading[props.plugin.id]) || Boolean(pluginsStore.settingsSaving[props.plugin.id]))
const busyLabel = computed(() => waitingForReady.value ? t('plugins.managementUi.loading') : '')
const sourceReference = computed(() => props.plugin.source?.package_source_ref?.trim() || props.plugin.source?.root?.trim() || t('display.empty'))
const actionErrorToast = computed(() => actionError.value ? {
  key: `plugin-management-ui:${props.plugin.id}:${actionError.value}`,
  level: 'error' as const,
  message: actionError.value,
} : null)

useToastFeedback(actionErrorToast)

let bridgePort: MessagePort | null = null
let bridgeSession = 0
let readyTimer: ReturnType<typeof setTimeout> | null = null
let frameMeasureAnimation: number | null = null
let lastSettings: Record<string, unknown> = {}
let lastSecretsConfigured: Record<string, boolean> = {}

const minimumFrameHeight = 320
const maximumFrameHeight = 1600
const frameViewportBottomGap = 24

function randomID(prefix: string) {
  const bytes = new Uint8Array(24)
  crypto.getRandomValues(bytes)
  return `${prefix}-${Array.from(bytes, (byte) => byte.toString(16).padStart(2, '0')).join('')}`
}

function toRecord(value: unknown): Record<string, unknown> | null {
  return value && typeof value === 'object' && !Array.isArray(value) ? value as Record<string, unknown> : null
}

function requestID(message: BridgeMessage) {
  return typeof message.request_id === 'string' && message.request_id.trim() ? message.request_id.trim() : undefined
}

function clearReadyTimer() {
  if (readyTimer) {
    clearTimeout(readyTimer)
    readyTimer = null
  }
}

function closeBridge() {
  bridgeSession += 1
  bridgePort?.close()
  bridgePort = null
  clearReadyTimer()
}

function readConfirmation() {
  if (!requiresConfirmation.value) {
    confirmed.value = true
    return
  }
  try {
    confirmed.value = localStorage.getItem(confirmationStorageKey.value) === '1'
  } catch {
    confirmed.value = false
  }
}

async function resolvePluginOrigin() {
  if (!configStore.document) {
    await configStore.fetchConfig()
  }
  const digest = await crypto.subtle.digest('SHA-256', new TextEncoder().encode(props.plugin.id.trim()))
  const pluginHost = `p-${Array.from(new Uint8Array(digest).slice(0, 8), (byte) => byte.toString(16).padStart(2, '0')).join('')}`
  const templateValue = configStore.document?.web?.plugin_ui_origin_template
  const configuredTemplate = typeof templateValue === 'string' ? templateValue.trim() : ''
  if (configuredTemplate) {
    return configuredTemplate.replaceAll('{plugin_host}', pluginHost).replace(/\/$/, '')
  }
  const backendTarget = typeof import.meta.env.VITE_BACKEND_TARGET === 'string' ? import.meta.env.VITE_BACKEND_TARGET.trim() : ''
  const backend = backendTarget ? new URL(backendTarget) : new URL(window.location.origin)
  const port = backend.port || (backend.protocol === 'https:' ? '443' : '80')
  return `${backend.protocol}//${pluginHost}.plugins.localhost:${port}`
}

async function restartFrame() {
  closeBridge()
  fatalError.value = null
  actionError.value = null
  waitingForReady.value = false
  reportedIframeHeight.value = 640
  iframeHeight.value = 640
  if (!confirmed.value && requiresConfirmation.value) {
    pluginOrigin.value = ''
    return
  }
  const session = bridgeSession
  try {
    pluginOrigin.value = await resolvePluginOrigin()
    if (session !== bridgeSession) return
    if (new URL(pluginOrigin.value).origin === window.location.origin) {
      throw new Error('插件页面域必须与管理域不同。')
    }
    bridgeNonce.value = randomID('nonce')
    iframeKey.value += 1
    waitingForReady.value = true
    readyTimer = setTimeout(() => {
      if (session !== bridgeSession || bridgePort) return
      waitingForReady.value = false
      fatalError.value = t('plugins.managementUi.loadTimeout')
    }, 10_000)
  } catch (error) {
    if (session !== bridgeSession) return
    fatalError.value = getDisplayErrorMessage(error, 'errors.common.loadFailed')
  }
}

function acceptUnverifiedSource() {
  try { localStorage.setItem(confirmationStorageKey.value, '1') } catch { /* current session still continues */ }
  confirmed.value = true
  void restartFrame()
}

function postPort(type: string, payload?: unknown, id?: string) {
  if (!bridgePort) return false
  const message: BridgeMessage = { version: '2', source: 'management_host', type }
  if (payload !== undefined) message.payload = JSON.parse(JSON.stringify(payload))
  if (id) message.request_id = id
  bridgePort.postMessage(message)
  return true
}

function postError(error: unknown, id?: string) {
  const message = getDisplayErrorMessage(error)
  actionError.value = message
  postPort('error', {
    code: error instanceof ApiError ? error.code : 'platform.internal_error',
    message,
  }, id)
}

function themePayload() {
  const styles = getComputedStyle(document.documentElement)
  return {
    mode: uiShellStore.resolvedThemeMode,
    tokens: {
      'color-bg': styles.getPropertyValue('--bg').trim(),
      'color-surface': styles.getPropertyValue('--surface-strong').trim() || styles.getPropertyValue('--surface').trim(),
      'color-text': styles.getPropertyValue('--text').trim(),
      'color-muted': styles.getPropertyValue('--muted').trim(),
      'color-primary': styles.getPropertyValue('--brand-fill').trim(),
      'color-border': styles.getPropertyValue('--border').trim(),
    },
  }
}

function postHostInit() {
  postPort('host.init', {
    plugin: {
      id: props.plugin.id,
      name: props.plugin.name ?? props.plugin.id,
      version: props.plugin.version ?? '0.0.0',
      state: props.plugin.state,
    },
    page: { id: props.page.id, label: props.page.label },
    config: lastSettings,
    secrets_configured: lastSecretsConfigured,
    theme: themePayload(),
    language: document.documentElement.lang || navigator.language || 'zh-CN',
    allowed_capabilities: [...(props.plugin.declared_capabilities ?? [])],
  })
}

async function initializeBridge(session: number) {
  try {
    const [settings, secrets] = await Promise.all([
      pluginsStore.fetchSettings(props.plugin.id),
      apiRequest<PluginSecretsResponse>(`/api/plugins/${encodeURIComponent(props.plugin.id)}/secrets`),
    ])
    if (session !== bridgeSession || !bridgePort) return
    lastSettings = settings.values
    lastSecretsConfigured = secrets.configured
    postHostInit()
    waitingForReady.value = false
    actionError.value = null
    clearReadyTimer()
  } catch (error) {
    if (session !== bridgeSession) return
    waitingForReady.value = false
    fatalError.value = getDisplayErrorMessage(error, 'errors.common.loadFailed')
    postError(error)
  }
}

function parseHandshake(value: unknown) {
  const message = toRecord(value)
  if (!message || message.version !== '2' || message.source !== 'plugin_management_ui' || message.type !== 'page.ready') return null
  return typeof message.nonce === 'string' ? message.nonce : null
}

function handleWindowMessage(event: MessageEvent) {
  if (event.source !== iframeRef.value?.contentWindow || event.origin !== frameOrigin.value) return
  const nonce = parseHandshake(event.data)
  if (nonce !== bridgeNonce.value || bridgePort) {
    waitingForReady.value = false
    fatalError.value = t('plugins.managementUi.invalidBridgeMessage')
    return
  }
  const channel = new MessageChannel()
  bridgePort = channel.port1
  const session = bridgeSession
  bridgePort.addEventListener('message', (portEvent) => handlePortMessage(portEvent, session))
  bridgePort.start()
  iframeRef.value?.contentWindow?.postMessage({
    version: '2', source: 'management_host', type: 'host.connect', nonce: bridgeNonce.value,
  }, frameOrigin.value, [channel.port2])
  void initializeBridge(session)
}

function parsePortMessage(value: unknown): BridgeMessage | null {
  const message = toRecord(value)
  if (!message || message.version !== '2' || message.source !== 'plugin_management_ui' || typeof message.type !== 'string') return null
  return message as unknown as BridgeMessage
}

function handlePortMessage(event: MessageEvent, session: number) {
  if (session !== bridgeSession) return
  const message = parsePortMessage(event.data)
  if (!message) {
    postPort('error', { code: 'plugin.protocol_violation', message: t('plugins.managementUi.invalidBridgeMessage') })
    return
  }
  const id = requestID(message)
  const payload = toRecord(message.payload)
  switch (message.type) {
    case 'settings.reload': void reloadSettings(id); return
    case 'settings.save': void saveSettings(toRecord(payload?.config) ?? {}, id); return
    case 'secrets.status.reload': void reloadSecrets(id); return
    case 'secrets.set': void setSecrets(toStringRecord(payload?.values), id); return
    case 'secrets.delete': void deleteSecrets(toStringArray(payload?.keys), id); return
    case 'ui.resize': resizeFrame(payload?.height); return
    case 'scheduler.trigger': void triggerSchedulerJob(String(payload?.job_id ?? ''), id); return
    case 'render_template.open': void openRenderTemplate(String(payload?.template_id ?? ''), id); return
    case 'protocol.targets.reload': void reloadProtocolTargets(id); return
    case 'protocol.identities.resolve': void resolveProtocolIdentities(payload?.items, id); return
    case 'plugin.action.invoke': void invokePluginManagementAction(String(payload?.action ?? ''), toRecord(payload?.payload) ?? {}, id); return
    default: postPort('error', { code: 'plugin.protocol_violation', message: t('plugins.managementUi.invalidBridgeMessage') }, id)
  }
}

function toStringRecord(value: unknown) {
  const record = toRecord(value)
  const result: Record<string, string> = {}
  if (!record) return result
  for (const [key, item] of Object.entries(record)) {
    if (/^[a-z0-9](?:[a-z0-9_.-]{0,126}[a-z0-9])?$/.test(key) && typeof item === 'string' && item.length > 0) result[key] = item
  }
  return result
}

function toStringArray(value: unknown) {
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string') : []
}

function resizeFrame(value: unknown) {
  if (typeof value !== 'number' || !Number.isFinite(value)) return
  reportedIframeHeight.value = Math.min(maximumFrameHeight, Math.max(minimumFrameHeight, Math.ceil(value)))
  updateFrameHeight()
}

function updateFrameHeight() {
  if (typeof window === 'undefined' || !iframeRef.value) {
    iframeHeight.value = reportedIframeHeight.value
    return
  }
  const visualViewportHeight = window.visualViewport?.height
  const viewportHeight = typeof visualViewportHeight === 'number' && Number.isFinite(visualViewportHeight)
    ? visualViewportHeight
    : window.innerHeight
  const measuredTop = iframeRef.value.getBoundingClientRect().top
  const frameTop = Number.isFinite(measuredTop) ? Math.max(0, measuredTop) : 0
  const availableHeight = Math.floor(viewportHeight - frameTop - frameViewportBottomGap)
  iframeHeight.value = Math.min(reportedIframeHeight.value, Math.max(minimumFrameHeight, availableHeight))
}

function scheduleFrameHeightUpdate() {
  if (typeof window === 'undefined' || frameMeasureAnimation !== null) return
  frameMeasureAnimation = window.requestAnimationFrame(() => {
    frameMeasureAnimation = null
    updateFrameHeight()
  })
}

async function reloadSettings(id?: string) {
  try {
    const response = await pluginsStore.fetchSettings(props.plugin.id)
    lastSettings = response.values
    postPort('settings.changed', { config: response.values }, id)
  } catch (error) { postError(error, id) }
}

async function saveSettings(values: PluginSettingsUpdateRequest['values'], id?: string) {
  try {
    const response = await pluginsStore.updateSettings(props.plugin.id, values)
    lastSettings = response.values
    await pluginsStore.fetchDetail(props.plugin.id)
    await governanceStore.fetchCommandPolicy().catch(() => undefined)
    postPort('settings.changed', { config: response.values }, id)
  } catch (error) { postError(error, id) }
}

async function reloadSecrets(id?: string) {
  try {
    const response = await apiRequest<PluginSecretsResponse>(`/api/plugins/${encodeURIComponent(props.plugin.id)}/secrets`)
    lastSecretsConfigured = response.configured
    postPort('secrets.status.changed', { configured: response.configured }, id)
  } catch (error) { postError(error, id) }
}

async function setSecrets(values: Record<string, string>, id?: string) {
  if (Object.keys(values).length === 0) { postPort('error', { code: 'platform.invalid_request', message: '至少提供一个非空密钥。' }, id); return }
  try {
    const response = await apiRequest<PluginSecretsUpdateResponse>(`/api/plugins/${encodeURIComponent(props.plugin.id)}/secrets`, { method: 'PUT', body: { values } })
    lastSecretsConfigured = response.configured
    postPort('secrets.status.changed', { configured: response.configured }, id)
  } catch (error) { postError(error, id) }
}

async function deleteSecrets(keys: string[], id?: string) {
  if (keys.length === 0) { postPort('error', { code: 'platform.invalid_request', message: '至少提供一个密钥名称。' }, id); return }
  try {
    const response = await apiRequest<PluginSecretsUpdateResponse>(`/api/plugins/${encodeURIComponent(props.plugin.id)}/secrets`, { method: 'DELETE', body: { keys } })
    lastSecretsConfigured = response.configured
    postPort('secrets.status.changed', { configured: response.configured }, id)
  } catch (error) { postError(error, id) }
}

function hasCapabilities(capabilities: string[], id?: string) {
  const missing = capabilities.filter((capability) => !(props.plugin.declared_capabilities ?? []).includes(capability))
  if (missing.length === 0) return true
  postPort('error', { code: 'plugin.capability_violation', message: `插件未声明必要能力：${missing.join('、')}` }, id)
  return false
}

async function triggerSchedulerJob(jobID: string, id?: string) {
  try {
    const response = await apiRequest<SchedulerJobTriggerResponse>(`/api/system/scheduler/jobs/${encodeURIComponent(jobID)}/trigger`, { method: 'POST' })
    postPort('scheduler.triggered', response, id)
  } catch (error) { postError(error, id) }
}

async function openRenderTemplate(templateID: string, id?: string) {
  try { await navigate(buildRenderTemplateLocation(templateID)) } catch (error) { postError(error, id) }
}

async function reloadProtocolTargets(id?: string) {
  if (!hasCapabilities(['group.list', 'friend.list'], id)) return
  try {
    const response = await apiRequest<OneBot11ProtocolTargetsResponse>('/api/protocols/onebot11/targets')
    postPort('protocol.targets.changed', response, id)
  } catch (error) { postError(error, id) }
}

async function resolveProtocolIdentities(value: unknown, id?: string) {
  const items = Array.isArray(value) ? value.filter((item): item is OneBot11IdentityResolveItem => {
    const record = toRecord(item)
    return (record?.target_type === 'group' || record?.target_type === 'private') && typeof record.target_id === 'string' && typeof record.user_id === 'string'
  }) : []
  const capabilities = [...(items.some((item) => item.target_type === 'group') ? ['group.member.get'] : []), ...(items.some((item) => item.target_type === 'private') ? ['user.info.get'] : [])]
  if (!hasCapabilities(capabilities, id)) return
  try {
    const response = await apiRequest<OneBot11IdentityResolveResponse>('/api/protocols/onebot11/identities/resolve', { method: 'POST', body: { items } })
    postPort('protocol.identities.resolved', response, id)
  } catch (error) { postError(error, id) }
}

async function invokePluginManagementAction(action: string, payload: Record<string, unknown>, id?: string) {
  if (!/^[a-z][a-z0-9_.:-]*$/.test(action)) { postPort('error', { code: 'platform.invalid_request', message: '管理动作名称无效。' }, id); return }
  try {
    const response = await apiRequest<PluginManagementActionResponse>(`/api/plugins/${encodeURIComponent(props.plugin.id)}/management/actions`, { method: 'POST', body: { action, payload } })
    postPort('plugin.action.result', { action: response.action, result: response.result }, id)
  } catch (error) { postError(error, id) }
}

function handleFrameLoad() {
  // The iframe initiates the nonce-bound handshake. Loading alone never grants a channel.
  updateFrameHeight()
}

watch([
  () => props.plugin.id,
  () => props.plugin.version ?? '',
  () => props.plugin.source?.package_source_ref ?? '',
  () => props.page.id,
  () => props.page.entry,
  () => props.plugin.trust?.level ?? '',
], () => {
  readConfirmation()
  void restartFrame()
}, { immediate: true })

watch(() => props.plugin.state, (state) => {
  if (state === 'starting') {
    restartFrameWhenRuntimeReady = true
    return
  }
  if (!restartFrameWhenRuntimeReady) return
  restartFrameWhenRuntimeReady = false
  if (state === 'running') void restartFrame()
})

watch(() => uiShellStore.resolvedThemeMode, () => {
  if (bridgePort) postHostInit()
})

if (typeof window !== 'undefined') {
  window.addEventListener('message', handleWindowMessage)
  window.addEventListener('resize', scheduleFrameHeightUpdate)
  document.addEventListener('scroll', scheduleFrameHeightUpdate, true)
  window.visualViewport?.addEventListener('resize', scheduleFrameHeightUpdate)
  window.visualViewport?.addEventListener('scroll', scheduleFrameHeightUpdate)
}

onBeforeUnmount(() => {
  closeBridge()
  if (typeof window !== 'undefined') {
    window.removeEventListener('message', handleWindowMessage)
    window.removeEventListener('resize', scheduleFrameHeightUpdate)
    document.removeEventListener('scroll', scheduleFrameHeightUpdate, true)
    window.visualViewport?.removeEventListener('resize', scheduleFrameHeightUpdate)
    window.visualViewport?.removeEventListener('scroll', scheduleFrameHeightUpdate)
    if (frameMeasureAnimation !== null) window.cancelAnimationFrame(frameMeasureAnimation)
  }
})
</script>

<template>
  <a-card :bordered="false" class="plugin-management-ui-card" data-testid="plugin-management-ui-host">
    <template #title>
      <div class="card-header"><span>{{ title }}</span><a-tag v-if="managementEntry">{{ managementEntry }}</a-tag></div>
    </template>

    <section v-if="requiresConfirmation && !confirmed" class="plugin-management-ui-confirm" data-testid="plugin-management-ui-confirm">
      <div class="plugin-management-ui-confirm-note"><strong>{{ t('plugins.managementUi.confirmTitle') }}</strong><p>{{ t('plugins.managementUi.confirmBody') }}</p></div>
      <a-descriptions :column="1" bordered size="small">
        <a-descriptions-item :label="t('plugins.fields.trust')">{{ plugin.trust?.label ?? t('display.empty') }}</a-descriptions-item>
        <a-descriptions-item :label="t('plugins.managementUi.entryPath')">{{ managementEntry || t('display.empty') }}</a-descriptions-item>
        <a-descriptions-item :label="t('plugins.fields.sourceRef')">{{ sourceReference }}</a-descriptions-item>
      </a-descriptions>
      <div class="table-actions"><a-button type="primary" @click="acceptUnverifiedSource">{{ t('plugins.managementUi.confirmAction') }}</a-button></div>
    </section>

    <RetryPanel v-else-if="fatalError" :title="t('plugins.managementUi.loadFailed')" :description="fatalError" :loading="false" variant="compact" @retry="restartFrame" />

    <div v-else class="plugin-management-ui-frame-shell">
      <a-spin :spinning="busy" :tip="busyLabel">
        <iframe
          v-if="canRenderIframe"
          :key="iframeKey"
          ref="iframeRef"
          class="plugin-management-ui-frame"
          :src="frameSrc"
          :style="{ height: `${iframeHeight}px` }"
          sandbox="allow-forms allow-same-origin allow-scripts"
          data-testid="plugin-management-ui-frame"
          :title="title"
          @load="handleFrameLoad"
        />
      </a-spin>
    </div>
  </a-card>
</template>

<style scoped lang="scss">
.plugin-management-ui-card,
.plugin-management-ui-card :deep(.ant-card-body) { display: flex; flex: 1 1 auto; flex-direction: column; min-height: 0; }
.plugin-management-ui-confirm { display: grid; gap: 16px; }
.plugin-management-ui-confirm-note { display: grid; gap: 6px; padding: 12px 14px; border: 1px solid var(--border); border-radius: var(--radius-md); background: var(--surface-soft); }
.plugin-management-ui-confirm-note p { margin: 0; color: var(--muted); }
.plugin-management-ui-frame-shell,
.plugin-management-ui-frame-shell :deep(.ant-spin-nested-loading),
.plugin-management-ui-frame-shell :deep(.ant-spin-container) { display: flex; flex: 1 1 auto; min-height: 0; width: 100%; }
.plugin-management-ui-frame { width: 100%; min-height: 320px; max-height: 1600px; border: 1px solid var(--border); border-radius: var(--radius-lg); background: var(--surface-strong); transition: height 160ms ease; }
</style>
