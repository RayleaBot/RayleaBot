<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'

import RetryPanel from '@/components/RetryPanel.vue'
import { t } from '@/i18n'
import { getDisplayErrorMessage } from '@/lib/error-text'
import { ApiError } from '@/lib/http'
import { usePluginsStore } from '@/stores/plugins'
import type { PluginDetail, PluginSettingsUpdateRequest } from '@/types/api'

interface PluginManagementUIHostInitPayload {
  plugin_id: string
  plugin: {
    name: string
    version?: string
    description?: string
    display_state: string
  }
  trust: {
    level: NonNullable<PluginDetail['trust']>['level']
    label: string
  }
  default_config: Record<string, unknown>
  settings: Record<string, unknown>
  title: string
}

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

const props = defineProps<{
  plugin: PluginDetail
  title: string
}>()

const pluginsStore = usePluginsStore()

const iframeRef = ref<HTMLIFrameElement | null>(null)
const iframeNonce = ref(0)
const confirmed = ref(false)
const waitingForReady = ref(false)
const fatalError = ref<string | null>(null)
const actionError = ref<string | null>(null)

const managementEntry = computed(() => props.plugin.management_ui?.entry?.trim() ?? '')
const requiresConfirmation = computed(() => props.plugin.trust?.level === 'unverified')
const confirmationStorageKey = computed(() => (
  `rayleabot.plugin-management-ui.confirmed:${props.plugin.id}:${props.plugin.version ?? ''}:${props.plugin.source?.package_source_type ?? ''}:${props.plugin.source?.package_source_ref ?? ''}`
))
const frameSrc = computed(() => buildPluginManagementUISrc(props.plugin.id, managementEntry.value))
const canRenderIframe = computed(() => Boolean(frameSrc.value) && (!requiresConfirmation.value || confirmed.value))
const busy = computed(() => (
  waitingForReady.value
  || Boolean(pluginsStore.settingsLoading[props.plugin.id])
  || Boolean(pluginsStore.settingsSaving[props.plugin.id])
))
const busyLabel = computed(() => {
  if (pluginsStore.settingsSaving[props.plugin.id]) {
    return t('plugins.managementUi.saving')
  }
  if (waitingForReady.value || pluginsStore.settingsLoading[props.plugin.id]) {
    return t('plugins.managementUi.loading')
  }
  return ''
})
const sourceReference = computed(() => (
  props.plugin.source?.package_source_ref?.trim()
  || props.plugin.source?.root?.trim()
  || t('display.empty')
))

let bridgeToken = 0
let initStartedForBridgeToken = 0
let readyTimer: ReturnType<typeof setTimeout> | null = null
let loadInitTimer: ReturnType<typeof setTimeout> | null = null
let requestCounter = 0
let acceptedOpaqueOrigin = false

function buildPluginManagementUISrc(pluginId: string, entry: string) {
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

  const routePath = `/plugin-ui/${encodeURIComponent(normalizedPluginId)}/${encodedEntry}`
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

function toBridgeValue<T>(value: T): T {
  if (value === undefined) {
    return value
  }

  return JSON.parse(JSON.stringify(value)) as T
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

function postHostInit(settings: Record<string, unknown>, requestId?: string) {
  const payload: PluginManagementUIHostInitPayload = {
    plugin_id: props.plugin.id,
    plugin: {
      name: props.plugin.name ?? props.plugin.id,
      version: props.plugin.version ?? undefined,
      description: props.plugin.description ?? undefined,
      display_state: props.plugin.display_state,
    },
    trust: {
      level: props.plugin.trust?.level ?? 'third_party',
      label: props.plugin.trust?.label ?? t('display.empty'),
    },
    default_config: toBridgeValue(toRecord(props.plugin.default_config) ?? {}),
    settings: toBridgeValue(settings),
    title: props.title,
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

async function initializeFrame(requestId?: string) {
  const currentToken = bridgeToken
  if (initStartedForBridgeToken === currentToken) {
    return
  }
  initStartedForBridgeToken = currentToken

  try {
    const response = await pluginsStore.fetchSettings(props.plugin.id)
    if (currentToken !== bridgeToken) {
      return
    }

    const posted = postHostInit(response.values, requestId)
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
  }
}

watch(
  [
    () => props.plugin.id,
    () => props.plugin.version ?? '',
    () => props.plugin.source?.package_source_type ?? '',
    () => props.plugin.source?.package_source_ref ?? '',
    () => managementEntry.value,
    () => props.plugin.trust?.level ?? '',
  ],
  () => {
    readConfirmation()
    restartFrame()
  },
  { immediate: true },
)

onMounted(() => {
  window.addEventListener('message', handleBridgeMessage)
})

onBeforeUnmount(() => {
  clearLoadInitTimer()
  clearReadyTimer()
  window.removeEventListener('message', handleBridgeMessage)
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

    <a-alert
      v-if="actionError"
      class="section-gap"
      :message="t('errors.common.actionFailed')"
      type="error"
      :description="actionError"
      show-icon
    />

    <section
      v-if="requiresConfirmation && !confirmed"
      class="plugin-management-ui-confirm"
      data-testid="plugin-management-ui-confirm"
    >
      <a-alert
        :message="t('plugins.managementUi.confirmTitle')"
        type="warning"
        :description="t('plugins.managementUi.confirmBody')"
        show-icon
      />

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
          sandbox="allow-forms allow-scripts"
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
  border-radius: 12px;
  background: #fff;
}

@media (max-width: 768px) {
  .plugin-management-ui-frame {
    min-height: 520px;
  }
}
</style>
