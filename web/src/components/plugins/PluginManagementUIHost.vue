<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useRouter } from 'vue-router'

import { useToastFeedback } from '@/adapter/feedback'
import RetryPanel from '@/components/RetryPanel.vue'
import { t } from '@/i18n'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { ApiError, apiRequest } from '@/lib/http'
import { buildRenderTemplateLocation } from '@/lib/management-links'
import { useGovernanceStore } from '@/stores/governance'
import { usePluginsStore } from '@/stores/plugins'
import type { PluginDetail, PluginSettingsUpdateRequest, SchedulerJobTriggerResponse } from '@/types/api'

interface PluginManagementUIHostInitPayload {
  plugin_id: string
  plugin: {
    name: string
    version?: string
    description?: string
    state: string
  }
  trust: {
    level: NonNullable<PluginDetail['trust']>['level']
    label: string
  }
  default_config: Record<string, unknown>
  settings: Record<string, unknown>
  secrets: Record<string, string>
  title: string
  page?: PluginManagementUIPage
}

interface PluginManagementUIPage {
  id: string
  label: string
  entry: string
}

type ThirdPartyBridgePlatform = 'bilibili' | 'weibo' | 'douyin' | 'netease_music'

type PluginManagementUIInboundMessage =
  | {
    version: '1'
    source: 'plugin_management_ui'
    type: 'page.ready'
    request_id?: string
  }
  | {
    version: '1'
    source: 'plugin_management_ui'
    type: 'settings.reload'
    request_id?: string
  }
  | {
    version: '1'
    source: 'plugin_management_ui'
    type: 'settings.save'
    request_id?: string
    payload: {
      values: PluginSettingsUpdateRequest['values']
    }
  }
  | {
    version: '1'
    source: 'plugin_management_ui'
    type: 'secrets.reload'
    request_id?: string
  }
  | {
    version: '1'
    source: 'plugin_management_ui'
    type: 'secrets.save'
    request_id?: string
    payload: {
      values: Record<string, string>
      deleted_keys?: string[]
    }
  }
  | {
    version: '1'
    source: 'plugin_management_ui'
    type: 'scheduler.trigger'
    request_id?: string
    payload: {
      job_id: string
    }
  }
  | {
    version: '1'
    source: 'plugin_management_ui'
    type: 'render_template.open'
    request_id?: string
    payload: {
      template_id: string
    }
  }
  | {
    version: '1'
    source: 'plugin_management_ui'
    type: 'protocol.targets.reload'
    request_id?: string
  }
  | {
    version: '1'
    source: 'plugin_management_ui'
    type: 'protocol.identities.resolve'
    request_id?: string
    payload: {
      items: OneBot11IdentityResolveItem[]
    }
  }
  | {
    version: '1'
    source: 'plugin_management_ui'
    type: 'bilibili.user.resolve'
    request_id?: string
    payload: {
      query: string
    }
  }
  | {
    version: '1'
    source: 'plugin_management_ui'
    type: 'thirdparty.user.resolve'
    request_id?: string
    payload: {
      platform: ThirdPartyBridgePlatform
      query: string
    }
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

interface OneBot11Identity {
  target_type: 'group' | 'private'
  target_id: string
  user_id: string
  nickname: string
  group_nickname?: string
  title?: string
  role?: string
  role_label?: string
  avatar_url: string
}

interface OneBot11IdentityResolveResponse {
  items: OneBot11Identity[]
  issues: OneBot11TargetIssue[]
}

interface BilibiliResolvedUser {
  uid: string
  name: string
  avatar_url: string
  fans?: number
}

interface BilibiliUserResolveResponse {
  query: string
  exact: boolean
  user?: BilibiliResolvedUser
  candidates: BilibiliResolvedUser[]
  message?: string
}

interface ThirdPartyResolvedUser {
  uid: string
  name: string
  avatar_url: string
}

interface ThirdPartyUserResolveResponse {
  platform: ThirdPartyBridgePlatform
  query: string
  exact: boolean
  user?: ThirdPartyResolvedUser
  candidates: ThirdPartyResolvedUser[]
  message?: string
}

const pluginSecretKeyPattern = /^[a-z0-9](?:[a-z0-9_.-]{0,126}[a-z0-9])?$/
const numericIdPattern = /^[0-9]+$/
const thirdPartyBridgePlatforms = new Set<ThirdPartyBridgePlatform>(['bilibili', 'weibo', 'douyin', 'netease_music'])

const props = defineProps<{
  plugin: PluginDetail
  title: string
  page: PluginManagementUIPage
}>()

const pluginsStore = usePluginsStore()
const governanceStore = useGovernanceStore()
const router = useRouter()

const iframeRef = ref<HTMLIFrameElement | null>(null)
const iframeNonce = ref(0)
const iframeSessionId = `${Date.now()}-${Math.random().toString(36).slice(2)}`
const confirmed = ref(false)
const waitingForReady = ref(false)
const fatalError = ref<string | null>(null)
const actionError = ref<string | null>(null)

const managementEntry = computed(() => props.page.entry.trim())
const requiresConfirmation = computed(() => props.plugin.trust?.level === 'unverified')
const confirmationStorageKey = computed(() => (
  `rayleabot.plugin-management-ui.confirmed:${props.plugin.id}:${props.plugin.version ?? ''}:${props.plugin.source?.package_source_type ?? ''}:${props.plugin.source?.package_source_ref ?? ''}`
))
const frameSrc = computed(() => buildPluginManagementUISrc(props.plugin.id, managementEntry.value, {
  version: props.plugin.version ?? '',
  sourceRef: props.plugin.source?.package_source_ref ?? props.plugin.source?.root ?? '',
  nonce: iframeNonce.value,
  session: iframeSessionId,
}))
const canRenderIframe = computed(() => Boolean(frameSrc.value) && (!requiresConfirmation.value || confirmed.value))
const busy = computed(() => (
  waitingForReady.value
  || Boolean(pluginsStore.settingsLoading[props.plugin.id])
  || Boolean(pluginsStore.settingsSaving[props.plugin.id])
  || Boolean(pluginsStore.secretsLoading[props.plugin.id])
  || Boolean(pluginsStore.secretsSaving[props.plugin.id])
))
const busyLabel = computed(() => {
  if (pluginsStore.settingsSaving[props.plugin.id] || pluginsStore.secretsSaving[props.plugin.id]) {
    return t('plugins.managementUi.saving')
  }
  if (waitingForReady.value || pluginsStore.settingsLoading[props.plugin.id] || pluginsStore.secretsLoading[props.plugin.id]) {
    return t('plugins.managementUi.loading')
  }
  return ''
})
const sourceReference = computed(() => (
  props.plugin.source?.package_source_ref?.trim()
  || props.plugin.source?.root?.trim()
  || t('display.empty')
))
const actionErrorToast = computed(() => (
  actionError.value
    ? {
        key: `plugin-management-ui:${props.plugin.id}:${actionError.value}`,
        level: 'error' as const,
        message: actionError.value,
      }
    : null
))

useToastFeedback(actionErrorToast)

let bridgeToken = 0
let initStartedForBridgeToken = 0
let initPayloadBridgeToken = 0
let lastInitSettings: Record<string, unknown> | null = null
let lastInitSecrets: Record<string, string> | null = null
let readyTimer: ReturnType<typeof setTimeout> | null = null
let loadInitTimer: ReturnType<typeof setTimeout> | null = null
let requestCounter = 0
let acceptedOpaqueOrigin = false

function buildPluginManagementUISrc(
  pluginId: string,
  entry: string,
  cacheKey: {
    version: string
    sourceRef: string
    nonce: number
    session: string
  },
) {
  const normalizedPluginId = pluginId.trim()
  const normalizedEntry = entry.trim()
  if (!normalizedPluginId || !normalizedEntry) {
    return ''
  }

  const encodedEntry = normalizedEntry
    .split('/')
    .filter((segment) => segment.trim().length > 0)
    .map((segment) => encodeURIComponent(segment))
    .join('/')

  const query = new URLSearchParams({
    plugin_id: normalizedPluginId,
    version: cacheKey.version.trim(),
    entry: normalizedEntry,
    source_ref: cacheKey.sourceRef.trim(),
    nonce: String(cacheKey.nonce),
    session: cacheKey.session,
  })
  const routePath = `/plugin-ui/${encodeURIComponent(normalizedPluginId)}/${encodedEntry}?${query.toString()}`
  const backendTarget = typeof import.meta.env.VITE_BACKEND_TARGET === 'string'
    ? import.meta.env.VITE_BACKEND_TARGET.trim()
    : ''

  if (import.meta.env.DEV && backendTarget) {
    return new URL(routePath, backendTarget).toString()
  }

  return routePath
}

function nextBridgeRequestId(prefix: string) {
  requestCounter += 1
  return `${prefix}-${Date.now()}-${requestCounter}`
}

function clearReadyTimer() {
  if (readyTimer) {
    clearTimeout(readyTimer)
    readyTimer = null
  }
}

function clearLoadInitTimer() {
  if (loadInitTimer) {
    clearTimeout(loadInitTimer)
    loadInitTimer = null
  }
}

function readConfirmation() {
  if (!requiresConfirmation.value) {
    confirmed.value = true
    return
  }

  try {
    confirmed.value = window.localStorage.getItem(confirmationStorageKey.value) === '1'
  } catch {
    confirmed.value = false
  }
}

function rememberConfirmation() {
  try {
    window.localStorage.setItem(confirmationStorageKey.value, '1')
  } catch {
    // ignore storage failures and keep the current session usable
  }
}

function restartFrame() {
  bridgeToken += 1
  initStartedForBridgeToken = 0
  initPayloadBridgeToken = 0
  lastInitSettings = null
  lastInitSecrets = null
  acceptedOpaqueOrigin = false
  clearLoadInitTimer()
  clearReadyTimer()
  fatalError.value = null
  actionError.value = null

  if (!canRenderIframe.value) {
    waitingForReady.value = false
    return
  }

  waitingForReady.value = true
  iframeNonce.value += 1
  const currentToken = bridgeToken
  readyTimer = setTimeout(() => {
    if (currentToken !== bridgeToken) {
      return
    }

    waitingForReady.value = false
    fatalError.value = t('plugins.managementUi.loadTimeout')
  }, 8000)
}

function toRecord(value: unknown) {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? value as Record<string, unknown>
    : null
}

function toStringRecord(value: unknown) {
  const record = toRecord(value)
  if (!record) {
    return null
  }

  const result: Record<string, string> = {}
  for (const [key, rawValue] of Object.entries(record)) {
    if (!pluginSecretKeyPattern.test(key) || typeof rawValue !== 'string') {
      return null
    }
    result[key] = rawValue
  }
  return result
}

function toThirdPartyBridgePlatform(value: unknown): ThirdPartyBridgePlatform | '' {
  return typeof value === 'string' && thirdPartyBridgePlatforms.has(value as ThirdPartyBridgePlatform)
    ? value as ThirdPartyBridgePlatform
    : ''
}

function toBridgeValue<T>(value: T): T {
  if (value === undefined) {
    return value
  }

  return JSON.parse(JSON.stringify(value)) as T
}

function toBridgePage(page: PluginManagementUIPage): PluginManagementUIPage {
  return {
    id: page.id,
    label: page.label,
    entry: page.entry,
  }
}

function parseInboundBridgeMessage(value: unknown): PluginManagementUIInboundMessage | null {
  const record = toRecord(value)
  if (!record || record.version !== '1' || record.source !== 'plugin_management_ui' || typeof record.type !== 'string') {
    return null
  }

  const requestId = typeof record.request_id === 'string' && record.request_id.trim().length > 0
    ? record.request_id.trim()
    : undefined

  switch (record.type) {
    case 'page.ready':
      return {
        version: '1',
        source: 'plugin_management_ui',
        type: 'page.ready',
        request_id: requestId,
      }
    case 'settings.reload':
      return {
        version: '1',
        source: 'plugin_management_ui',
        type: 'settings.reload',
        request_id: requestId,
      }
    case 'settings.save': {
      const payload = toRecord(record.payload)
      const values = toRecord(payload?.values)
      if (!values) {
        return null
      }

      return {
        version: '1',
        source: 'plugin_management_ui',
        type: 'settings.save',
        request_id: requestId,
        payload: {
          values,
        },
      }
    }
    case 'secrets.reload':
      return {
        version: '1',
        source: 'plugin_management_ui',
        type: 'secrets.reload',
        request_id: requestId,
      }
    case 'secrets.save': {
      const payload = toRecord(record.payload)
      const values = toStringRecord(payload?.values)
      if (!values) {
        return null
      }
      const deletedKeys = Array.isArray(payload?.deleted_keys)
        ? payload.deleted_keys.filter((item): item is string => typeof item === 'string' && pluginSecretKeyPattern.test(item))
        : undefined

      return {
        version: '1',
        source: 'plugin_management_ui',
        type: 'secrets.save',
        request_id: requestId,
        payload: {
          values,
          deleted_keys: deletedKeys,
        },
      }
    }
    case 'scheduler.trigger': {
      const payload = toRecord(record.payload)
      const jobId = typeof payload?.job_id === 'string' ? payload.job_id.trim() : ''
      if (!jobId) {
        return null
      }
      return {
        version: '1',
        source: 'plugin_management_ui',
        type: 'scheduler.trigger',
        request_id: requestId,
        payload: {
          job_id: jobId,
        },
      }
    }
    case 'render_template.open': {
      const payload = toRecord(record.payload)
      const templateId = typeof payload?.template_id === 'string' ? payload.template_id.trim() : ''
      if (!templateId) {
        return null
      }
      return {
        version: '1',
        source: 'plugin_management_ui',
        type: 'render_template.open',
        request_id: requestId,
        payload: {
          template_id: templateId,
        },
      }
    }
    case 'protocol.targets.reload':
      return {
        version: '1',
        source: 'plugin_management_ui',
        type: 'protocol.targets.reload',
        request_id: requestId,
      }
    case 'protocol.identities.resolve': {
      const payload = toRecord(record.payload)
      const rawItems = Array.isArray(payload?.items) ? payload.items : []
      const items: OneBot11IdentityResolveItem[] = []
      for (const rawItem of rawItems) {
        const item = toRecord(rawItem)
        const targetType = item?.target_type === 'group' || item?.target_type === 'private'
          ? item.target_type
          : ''
        const targetId = typeof item?.target_id === 'string' ? item.target_id.trim() : ''
        const userId = typeof item?.user_id === 'string' ? item.user_id.trim() : ''
        if (!targetType || !numericIdPattern.test(targetId) || !numericIdPattern.test(userId)) {
          return null
        }
        items.push({
          target_type: targetType,
          target_id: targetId,
          user_id: userId,
        })
      }
      if (!items.length || items.length > 100) {
        return null
      }
      return {
        version: '1',
        source: 'plugin_management_ui',
        type: 'protocol.identities.resolve',
        request_id: requestId,
        payload: {
          items,
        },
      }
    }
    case 'bilibili.user.resolve': {
      const payload = toRecord(record.payload)
      const query = typeof payload?.query === 'string' ? payload.query.trim() : ''
      if (!query) {
        return null
      }
      return {
        version: '1',
        source: 'plugin_management_ui',
        type: 'bilibili.user.resolve',
        request_id: requestId,
        payload: {
          query,
        },
      }
    }
    case 'thirdparty.user.resolve': {
      const payload = toRecord(record.payload)
      const platform = toThirdPartyBridgePlatform(payload?.platform)
      const query = typeof payload?.query === 'string' ? payload.query.trim() : ''
      if (!platform || !query) {
        return null
      }
      return {
        version: '1',
        source: 'plugin_management_ui',
        type: 'thirdparty.user.resolve',
        request_id: requestId,
        payload: {
          platform,
          query,
        },
      }
    }
    default:
      return null
  }
}

function postMessageToIframe(message: Record<string, unknown>) {
  const frameWindow = iframeRef.value?.contentWindow
  if (!frameWindow) {
    return false
  }

  try {
    frameWindow.postMessage(message, '*')
    return true
  } catch {
    return false
  }
}

function postBridgeError(message: string, options: { code?: string; requestId?: string } = {}) {
  return postMessageToIframe({
    version: '1',
    source: 'management_host',
    type: 'error',
    request_id: options.requestId ?? nextBridgeRequestId('host-error'),
    payload: {
      ...(options.code ? { code: options.code } : {}),
      message,
    },
  })
}

function postHostInit(settings: Record<string, unknown>, secrets: Record<string, string>, requestId?: string) {
  const page = toBridgePage(props.page)
  const payload: PluginManagementUIHostInitPayload = {
    plugin_id: props.plugin.id,
    plugin: {
      name: props.plugin.name ?? props.plugin.id,
      version: props.plugin.version ?? undefined,
      description: props.plugin.description ?? undefined,
      state: props.plugin.state,
    },
    trust: {
      level: props.plugin.trust?.level ?? 'third_party',
      label: props.plugin.trust?.label ?? t('display.empty'),
    },
    default_config: toBridgeValue(toRecord(props.plugin.default_config) ?? {}),
    settings: toBridgeValue(settings),
    secrets: toBridgeValue(secrets),
    title: props.title,
    ...(page ? { page } : {}),
  }

  return postMessageToIframe({
    version: '1',
    source: 'management_host',
    type: 'host.init',
    request_id: requestId ?? nextBridgeRequestId('host-init'),
    payload,
  })
}

function postSettingsChanged(values: Record<string, unknown>, changedKeys: string[], requestId?: string) {
  return postMessageToIframe({
    version: '1',
    source: 'management_host',
    type: 'settings.changed',
    request_id: requestId ?? nextBridgeRequestId('settings-changed'),
    payload: {
      values: toBridgeValue(values),
      changed_keys: toBridgeValue(changedKeys),
    },
  })
}

function postSecretsChanged(values: Record<string, string>, changedKeys: string[], requestId?: string) {
  return postMessageToIframe({
    version: '1',
    source: 'management_host',
    type: 'secrets.changed',
    request_id: requestId ?? nextBridgeRequestId('secrets-changed'),
    payload: {
      values: toBridgeValue(values),
      changed_keys: toBridgeValue(changedKeys),
    },
  })
}

function postSchedulerTriggered(response: SchedulerJobTriggerResponse, requestId?: string) {
  return postMessageToIframe({
    version: '1',
    source: 'management_host',
    type: 'scheduler.triggered',
    request_id: requestId ?? nextBridgeRequestId('scheduler-triggered'),
    payload: {
      job_id: response.job_id,
      plugin_id: response.plugin_id,
      triggered: response.triggered,
    },
  })
}

function postProtocolTargetsChanged(response: OneBot11ProtocolTargetsResponse, requestId?: string) {
  return postMessageToIframe({
    version: '1',
    source: 'management_host',
    type: 'protocol.targets.changed',
    request_id: requestId ?? nextBridgeRequestId('protocol-targets-changed'),
    payload: toBridgeValue(response),
  })
}

function postProtocolIdentitiesResolved(response: OneBot11IdentityResolveResponse, requestId?: string) {
  return postMessageToIframe({
    version: '1',
    source: 'management_host',
    type: 'protocol.identities.resolved',
    request_id: requestId ?? nextBridgeRequestId('protocol-identities-resolved'),
    payload: toBridgeValue(response),
  })
}

function postBilibiliUserResolved(response: BilibiliUserResolveResponse, requestId?: string) {
  return postMessageToIframe({
    version: '1',
    source: 'management_host',
    type: 'bilibili.user.resolved',
    request_id: requestId ?? nextBridgeRequestId('bilibili-user-resolved'),
    payload: toBridgeValue(response),
  })
}

function postThirdPartyUserResolved(response: ThirdPartyUserResolveResponse, requestId?: string) {
  return postMessageToIframe({
    version: '1',
    source: 'management_host',
    type: 'thirdparty.user.resolved',
    request_id: requestId ?? nextBridgeRequestId('third-party-user-resolved'),
    payload: toBridgeValue(response),
  })
}

function hasBridgeCapability(capability: string) {
  return (props.plugin.declared_capabilities ?? []).includes(capability)
}

function canUseBridgeCapabilities(capabilities: string[], requestId?: string) {
  const missing = capabilities.filter((capability) => !hasBridgeCapability(capability))
  if (!missing.length) {
    return true
  }
  postBridgeError(`插件未声明必要能力：${missing.join('、')}`, {
    code: 'plugin.capability_violation',
    requestId,
  })
  return false
}

async function initializeFrame(requestId?: string) {
  const currentToken = bridgeToken
  if (initStartedForBridgeToken === currentToken) {
    if (initPayloadBridgeToken === currentToken && lastInitSettings && lastInitSecrets) {
      const posted = postHostInit(lastInitSettings, lastInitSecrets, requestId)
      if (posted) {
        waitingForReady.value = false
        actionError.value = null
        fatalError.value = null
        clearLoadInitTimer()
        clearReadyTimer()
      }
    }
    return
  }
  initStartedForBridgeToken = currentToken

  try {
    const [settingsResponse, secretsResponse] = await Promise.all([
      pluginsStore.fetchSettings(props.plugin.id),
      pluginsStore.fetchSecrets(props.plugin.id),
    ])
    if (currentToken !== bridgeToken) {
      return
    }

    lastInitSettings = settingsResponse.values
    lastInitSecrets = secretsResponse.values
    initPayloadBridgeToken = currentToken
    const posted = postHostInit(settingsResponse.values, secretsResponse.values, requestId)
    if (!posted) {
      initStartedForBridgeToken = 0
      waitingForReady.value = true
      clearLoadInitTimer()
      loadInitTimer = setTimeout(() => {
        if (currentToken !== bridgeToken) {
          return
        }

        void initializeFrame(requestId)
      }, 160)
      return
    }

    waitingForReady.value = false
    actionError.value = null
    fatalError.value = null
    clearLoadInitTimer()
    clearReadyTimer()
  } catch (error) {
    if (currentToken !== bridgeToken) {
      return
    }

    waitingForReady.value = false
    clearReadyTimer()
    fatalError.value = getDisplayErrorMessage(error, 'errors.common.loadFailed')
    postBridgeError(fatalError.value, {
      code: error instanceof ApiError ? error.code : undefined,
      requestId,
    })
  }
}

async function triggerSchedulerJob(jobId: string, requestId?: string) {
  const currentToken = bridgeToken

  try {
    const response = await apiRequest<SchedulerJobTriggerResponse>(`/api/system/scheduler/jobs/${encodeURIComponent(jobId)}/trigger`, {
      method: 'POST',
    })
    if (currentToken !== bridgeToken) {
      return
    }

    actionError.value = null
    postSchedulerTriggered(response, requestId)
  } catch (error) {
    if (currentToken !== bridgeToken) {
      return
    }

    actionError.value = getDisplayErrorMessage(error)
    postBridgeError(actionError.value, {
      code: error instanceof ApiError ? error.code : undefined,
      requestId,
    })
  }
}

async function openRenderTemplate(templateId: string, requestId?: string) {
  try {
    await router.push(buildRenderTemplateLocation(templateId))
  } catch (error) {
    actionError.value = getDisplayErrorMessage(error)
    postBridgeError(actionError.value, {
      code: error instanceof ApiError ? error.code : undefined,
      requestId,
    })
  }
}

async function reloadProtocolTargets(requestId?: string) {
  if (!canUseBridgeCapabilities(['group.list', 'friend.list'], requestId)) {
    return
  }
  const currentToken = bridgeToken

  try {
    const response = await apiRequest<OneBot11ProtocolTargetsResponse>('/api/protocols/onebot11/targets')
    if (currentToken !== bridgeToken) {
      return
    }
    actionError.value = null
    postProtocolTargetsChanged(response, requestId)
  } catch (error) {
    if (currentToken !== bridgeToken) {
      return
    }

    actionError.value = getDisplayErrorMessage(error)
    postBridgeError(actionError.value, {
      code: error instanceof ApiError ? error.code : undefined,
      requestId,
    })
  }
}

async function resolveProtocolIdentities(items: OneBot11IdentityResolveItem[], requestId?: string) {
  const needsGroup = items.some((item) => item.target_type === 'group')
  const needsPrivate = items.some((item) => item.target_type === 'private')
  const capabilities = [
    ...(needsGroup ? ['group.member.get'] : []),
    ...(needsPrivate ? ['user.info.get'] : []),
  ]
  if (!canUseBridgeCapabilities(capabilities, requestId)) {
    return
  }
  const currentToken = bridgeToken

  try {
    const response = await apiRequest<OneBot11IdentityResolveResponse>('/api/protocols/onebot11/identities/resolve', {
      method: 'POST',
      body: { items },
    })
    if (currentToken !== bridgeToken) {
      return
    }
    actionError.value = null
    postProtocolIdentitiesResolved(response, requestId)
  } catch (error) {
    if (currentToken !== bridgeToken) {
      return
    }

    actionError.value = getDisplayErrorMessage(error)
    postBridgeError(actionError.value, {
      code: error instanceof ApiError ? error.code : undefined,
      requestId,
    })
  }
}

async function resolveBilibiliUser(query: string, requestId?: string) {
  if (!canUseBridgeCapabilities(['http.request'], requestId)) {
    return
  }
  const currentToken = bridgeToken

  try {
    const response = await fetchBilibiliUserResolve(query)
    if (currentToken !== bridgeToken) {
      return
    }
    actionError.value = null
    postBilibiliUserResolved(response, requestId)
  } catch (error) {
    if (currentToken !== bridgeToken) {
      return
    }

    actionError.value = getDisplayErrorMessage(error)
    postBridgeError(actionError.value, {
      code: error instanceof ApiError ? error.code : undefined,
      requestId,
    })
  }
}

async function resolveThirdPartyUser(platform: ThirdPartyBridgePlatform, query: string, requestId?: string) {
  if (!canUseBridgeCapabilities(['http.request'], requestId)) {
    return
  }
  const currentToken = bridgeToken

  try {
    const response = platform === 'bilibili'
      ? thirdPartyResponseFromBilibili(await fetchBilibiliUserResolve(query))
      : await fetchThirdPartyUserResolve(platform, query)
    if (currentToken !== bridgeToken) {
      return
    }
    actionError.value = null
    postThirdPartyUserResolved(response, requestId)
  } catch (error) {
    if (currentToken !== bridgeToken) {
      return
    }

    actionError.value = getDisplayErrorMessage(error)
    postBridgeError(actionError.value, {
      code: error instanceof ApiError ? error.code : undefined,
      requestId,
    })
  }
}

async function fetchBilibiliUserResolve(query: string) {
  const params = new URLSearchParams({ query })
  return apiRequest<BilibiliUserResolveResponse>(`/api/bilibili/users/resolve?${params.toString()}`)
}

async function fetchThirdPartyUserResolve(platform: Exclude<ThirdPartyBridgePlatform, 'bilibili'>, query: string) {
  const params = new URLSearchParams({ platform, query })
  return apiRequest<ThirdPartyUserResolveResponse>(`/api/third-party/users/resolve?${params.toString()}`)
}

function thirdPartyResponseFromBilibili(response: BilibiliUserResolveResponse): ThirdPartyUserResolveResponse {
  return {
    platform: 'bilibili',
    query: response.query,
    exact: response.exact,
    user: response.user ? thirdPartyUserFromBilibili(response.user) : undefined,
    candidates: response.candidates.map(thirdPartyUserFromBilibili),
    ...(response.message ? { message: response.message } : {}),
  }
}

function thirdPartyUserFromBilibili(user: BilibiliResolvedUser): ThirdPartyResolvedUser {
  return {
    uid: user.uid,
    name: user.name,
    avatar_url: user.avatar_url,
  }
}

function handleFrameLoad() {
  if (!canRenderIframe.value) {
    return
  }

  acceptedOpaqueOrigin = true
  clearLoadInitTimer()
  const currentToken = bridgeToken
  loadInitTimer = setTimeout(() => {
    if (currentToken !== bridgeToken || !waitingForReady.value) {
      return
    }

    void initializeFrame()
  }, 120)
}

async function reloadSettings(requestId?: string) {
  const currentToken = bridgeToken

  try {
    const response = await pluginsStore.fetchSettings(props.plugin.id)
    if (currentToken !== bridgeToken) {
      return
    }

    actionError.value = null
    postSettingsChanged(response.values, [], requestId)
  } catch (error) {
    if (currentToken !== bridgeToken) {
      return
    }

    actionError.value = getDisplayErrorMessage(error, 'errors.common.loadFailed')
    postBridgeError(actionError.value, {
      code: error instanceof ApiError ? error.code : undefined,
      requestId,
    })
  }
}

async function saveSettings(values: PluginSettingsUpdateRequest['values'], requestId?: string) {
  const currentToken = bridgeToken

  try {
    const response = await pluginsStore.updateSettings(props.plugin.id, values)
    if (currentToken !== bridgeToken) {
      return
    }

    await pluginsStore.fetchDetail(props.plugin.id)
    if (currentToken !== bridgeToken) {
      return
    }
    await governanceStore.fetchCommandPolicy().catch(() => undefined)
    if (currentToken !== bridgeToken) {
      return
    }

    actionError.value = null
    postSettingsChanged(response.values, response.changed_keys, requestId)
  } catch (error) {
    if (currentToken !== bridgeToken) {
      return
    }

    actionError.value = getDisplayErrorMessage(error)
    postBridgeError(actionError.value, {
      code: error instanceof ApiError ? error.code : undefined,
      requestId,
    })
  }
}

async function reloadSecrets(requestId?: string) {
  const currentToken = bridgeToken

  try {
    const response = await pluginsStore.fetchSecrets(props.plugin.id)
    if (currentToken !== bridgeToken) {
      return
    }

    actionError.value = null
    postSecretsChanged(response.values, [], requestId)
  } catch (error) {
    if (currentToken !== bridgeToken) {
      return
    }

    actionError.value = getDisplayErrorMessage(error, 'errors.common.loadFailed')
    postBridgeError(actionError.value, {
      code: error instanceof ApiError ? error.code : undefined,
      requestId,
    })
  }
}

async function saveSecrets(values: Record<string, string>, deletedKeys: string[] = [], requestId?: string) {
  const currentToken = bridgeToken

  try {
    const response = await pluginsStore.updateSecrets(props.plugin.id, values, deletedKeys)
    if (currentToken !== bridgeToken) {
      return
    }

    actionError.value = null
    postSecretsChanged(response.values, response.changed_keys, requestId)
  } catch (error) {
    if (currentToken !== bridgeToken) {
      return
    }

    actionError.value = getDisplayErrorMessage(error)
    postBridgeError(actionError.value, {
      code: error instanceof ApiError ? error.code : undefined,
      requestId,
    })
  }
}

function acceptUnverifiedSource() {
  rememberConfirmation()
  confirmed.value = true
  restartFrame()
}

function retryLoad() {
  restartFrame()
}

function handleBridgeMessage(event: MessageEvent) {
  const message = parseInboundBridgeMessage(event.data)
  if (!message) {
    if (event.source !== iframeRef.value?.contentWindow && !(acceptedOpaqueOrigin && event.origin === 'null')) {
      return
    }

    waitingForReady.value = false
    clearReadyTimer()
    fatalError.value = t('plugins.managementUi.invalidBridgeMessage')
    postBridgeError(fatalError.value)
    return
  }

  const matchesFrameWindow = event.source === iframeRef.value?.contentWindow
  const canUseOpaqueOrigin = event.origin === 'null' && canRenderIframe.value
  if (!matchesFrameWindow) {
    if (acceptedOpaqueOrigin && canUseOpaqueOrigin) {
      // continue
    } else if (waitingForReady.value && message.type === 'page.ready' && canUseOpaqueOrigin) {
      acceptedOpaqueOrigin = true
    } else {
      return
    }
  }

  switch (message.type) {
    case 'page.ready':
      void initializeFrame(message.request_id)
      return
    case 'settings.reload':
      void reloadSettings(message.request_id)
      return
    case 'settings.save':
      void saveSettings(message.payload.values, message.request_id)
      return
    case 'secrets.reload':
      void reloadSecrets(message.request_id)
      return
    case 'secrets.save':
      void saveSecrets(message.payload.values, message.payload.deleted_keys ?? [], message.request_id)
      return
    case 'scheduler.trigger':
      void triggerSchedulerJob(message.payload.job_id, message.request_id)
      return
    case 'render_template.open':
      void openRenderTemplate(message.payload.template_id, message.request_id)
      return
    case 'protocol.targets.reload':
      void reloadProtocolTargets(message.request_id)
      return
    case 'protocol.identities.resolve':
      void resolveProtocolIdentities(message.payload.items, message.request_id)
      return
    case 'bilibili.user.resolve':
      void resolveBilibiliUser(message.payload.query, message.request_id)
      return
    case 'thirdparty.user.resolve':
      void resolveThirdPartyUser(message.payload.platform, message.payload.query, message.request_id)
      return
  }
}

if (typeof window !== 'undefined') {
  window.addEventListener('message', handleBridgeMessage)
}

watch(
  [
    () => props.plugin.id,
    () => props.plugin.version ?? '',
    () => props.plugin.source?.package_source_type ?? '',
    () => props.plugin.source?.package_source_ref ?? '',
    () => managementEntry.value,
    () => props.page?.id ?? '',
    () => props.page?.entry ?? '',
    () => props.plugin.trust?.level ?? '',
  ],
  () => {
    readConfirmation()
    restartFrame()
  },
  { immediate: true },
)

onBeforeUnmount(() => {
  clearLoadInitTimer()
  clearReadyTimer()
  if (typeof window !== 'undefined') {
    window.removeEventListener('message', handleBridgeMessage)
  }
})
</script>

<template>
  <a-card :bordered="false" class="plugin-management-ui-card" data-testid="plugin-management-ui-host">
    <template #title>
      <div class="card-header">
        <span>{{ title }}</span>
        <a-tag v-if="managementEntry">{{ managementEntry }}</a-tag>
      </div>
    </template>

    <section
      v-if="requiresConfirmation && !confirmed"
      class="plugin-management-ui-confirm"
      data-testid="plugin-management-ui-confirm"
    >
      <div class="plugin-management-ui-confirm-note">
        <strong>{{ t('plugins.managementUi.confirmTitle') }}</strong>
        <p>{{ t('plugins.managementUi.confirmBody') }}</p>
      </div>

      <a-descriptions :column="1" bordered size="small">
        <a-descriptions-item :label="t('plugins.fields.trust')">
          {{ plugin.trust?.label ?? t('display.empty') }}
        </a-descriptions-item>
        <a-descriptions-item :label="t('plugins.managementUi.entryPath')">
          {{ managementEntry || t('display.empty') }}
        </a-descriptions-item>
        <a-descriptions-item :label="t('plugins.fields.sourceRef')">
          {{ sourceReference }}
        </a-descriptions-item>
      </a-descriptions>

      <div class="table-actions">
        <a-button type="primary" @click="acceptUnverifiedSource">
          {{ t('plugins.managementUi.confirmAction') }}
        </a-button>
      </div>
    </section>

    <RetryPanel
      v-else-if="fatalError"
      :title="t('plugins.managementUi.loadFailed')"
      :description="fatalError"
      :loading="false"
      variant="compact"
      @retry="retryLoad"
    />

    <div v-else class="plugin-management-ui-frame-shell">
      <a-spin :spinning="busy" :tip="busyLabel">
        <iframe
          v-if="canRenderIframe"
          :key="iframeNonce"
          ref="iframeRef"
          class="plugin-management-ui-frame"
          :src="frameSrc"
          sandbox="allow-forms allow-modals allow-scripts"
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
.plugin-management-ui-card :deep(.ant-card-body) {
  display: flex;
  flex: 1 1 auto;
  flex-direction: column;
  min-height: 0;
}

.plugin-management-ui-confirm {
  display: grid;
  gap: 16px;
}

.plugin-management-ui-confirm-note {
  display: grid;
  gap: 6px;
  padding: 12px 14px;
  border: 1px solid var(--border);
  border-radius: var(--radius-md);
  background: var(--surface-soft);

  strong {
    color: var(--text);
    font-size: 0.9rem;
    line-height: 1.4;
  }

  p {
    margin: 0;
    color: var(--muted);
    font-size: 0.86rem;
    line-height: 1.5;
  }
}

.plugin-management-ui-frame-shell,
.plugin-management-ui-frame-shell :deep(.ant-spin-nested-loading),
.plugin-management-ui-frame-shell :deep(.ant-spin-container) {
  display: flex;
  flex: 1 1 auto;
  min-height: 0;
}

.plugin-management-ui-frame-shell :deep(.ant-spin-nested-loading) {
  width: 100%;
}

.plugin-management-ui-frame {
  width: 100%;
  min-height: 640px;
  flex: 1 1 auto;
  border: 1px solid var(--border);
  border-radius: var(--radius-lg);
  background: var(--surface-strong);
}

@media (max-width: 768px) {
  .plugin-management-ui-frame {
    min-height: 520px;
  }
}
</style>
