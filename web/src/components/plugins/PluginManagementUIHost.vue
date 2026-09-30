<script setup lang="ts">
import AppLoadingPanel from '@/components/AppLoadingPanel.vue'
import AppDetails from '@/components/AppDetails.vue'
import AppDetailItem from '@/components/AppDetailItem.vue'
import AppButton from '@/components/AppButton.vue'
import { computed, onBeforeUnmount, ref, watch } from 'vue'

import RetryPanel from '@/components/RetryPanel.vue'
import { t } from '@/i18n'
import { getPluginTrustLabel } from '@/lib/display'
import type { PluginDetail, PluginManagementUIPage } from '@/types/api'

const props = defineProps<{
  plugin: PluginDetail
  title: string
  page: PluginManagementUIPage
}>()

const iframeRef = ref<HTMLIFrameElement | null>(null)
const iframeKey = ref(0)
const confirmed = ref(false)
const waitingForLoad = ref(false)
const fatalError = ref<string | null>(null)
let restartFrameWhenRuntimeReady = props.plugin.state === 'starting'
let loadTimer: ReturnType<typeof setTimeout> | null = null

const managementEntry = computed(() => props.plugin.management_ui?.entry?.trim() ?? '')
const requiresConfirmation = computed(() => props.plugin.trust?.level === 'unverified')
const confirmationStorageKey = computed(() => (
  `rayleabot.plugin-management-ui.confirmed:${props.plugin.id}:${props.plugin.version ?? ''}:${props.plugin.source?.package_source_ref ?? ''}`
))
const frameSrc = computed(() => {
  if (!managementEntry.value.startsWith('ui/')) {
    return ''
  }
  const relativeEntry = managementEntry.value.slice('ui/'.length).split('/').map(encodeURIComponent).join('/')
  const search = new URLSearchParams({ page: props.page.id, version: props.plugin.version ?? '', frame: String(iframeKey.value) })
  return `/plugin-ui/${encodeURIComponent(props.plugin.id)}/${relativeEntry}?${search.toString()}`
})
const canRenderIframe = computed(() => Boolean(frameSrc.value) && iframeKey.value > 0 && (!requiresConfirmation.value || confirmed.value))
const busyLabel = computed(() => waitingForLoad.value ? t('plugins.managementUi.loading') : '')
const sourceReference = computed(() => props.plugin.source?.package_source_ref?.trim() || props.plugin.source?.root?.trim() || t('display.empty'))

function clearLoadTimer() {
  if (loadTimer) {
    clearTimeout(loadTimer)
    loadTimer = null
  }
}

function failFrame(message: string) {
  clearLoadTimer()
  waitingForLoad.value = false
  fatalError.value = message
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

function restartFrame() {
  clearLoadTimer()
  fatalError.value = null
  waitingForLoad.value = false
  if (!managementEntry.value.startsWith('ui/')) {
    failFrame(t('plugins.managementUi.frameUnavailable'))
    return
  }
  if (requiresConfirmation.value && !confirmed.value) {
    return
  }
  waitingForLoad.value = true
  iframeKey.value += 1
  loadTimer = setTimeout(() => {
    if (waitingForLoad.value) failFrame(t('plugins.managementUi.loadTimeout'))
  }, 10_000)
}

function acceptUnverifiedSource() {
  try { localStorage.setItem(confirmationStorageKey.value, '1') } catch { /* current session still continues */ }
  confirmed.value = true
  restartFrame()
}

function handleFrameLoad() {
  let documentElement: HTMLElement | null = null
  try {
    const frame = iframeRef.value
    if (frame?.contentWindow?.location.pathname.startsWith('/plugin-ui/')) {
      documentElement = frame.contentDocument?.documentElement ?? null
    }
  } catch {
    documentElement = null
  }
  if (!documentElement) {
    failFrame(t('plugins.managementUi.frameUnavailable'))
    return
  }
  clearLoadTimer()
  waitingForLoad.value = false
}

watch([
  () => props.plugin.id,
  () => props.plugin.version ?? '',
  () => props.plugin.source?.package_source_ref ?? '',
  () => props.page.id,
  () => props.plugin.management_ui?.entry ?? '',
  () => props.plugin.trust?.level ?? '',
], () => {
  readConfirmation()
  restartFrame()
}, { immediate: true })

watch(() => props.plugin.state, (state) => {
  if (state === 'starting') {
    restartFrameWhenRuntimeReady = true
    return
  }
  if (!restartFrameWhenRuntimeReady) return
  restartFrameWhenRuntimeReady = false
  if (state === 'running') restartFrame()
})

onBeforeUnmount(clearLoadTimer)
</script>

<template>
  <section class="plugin-management-ui-host" data-testid="plugin-management-ui-host" :aria-label="title">
    <section v-if="requiresConfirmation && !confirmed" class="plugin-management-ui-confirm" data-testid="plugin-management-ui-confirm">
      <div class="plugin-management-ui-confirm-note"><strong>{{ t('plugins.managementUi.confirmTitle') }}</strong><p>{{ t('plugins.managementUi.confirmBody') }}</p></div>
      <AppDetails>
        <AppDetailItem :label="t('plugins.fields.trust')">{{ getPluginTrustLabel(plugin.trust?.level) }}</AppDetailItem>
        <AppDetailItem :label="t('plugins.managementUi.entryPath')">{{ managementEntry || t('display.empty') }}</AppDetailItem>
        <AppDetailItem :label="t('plugins.fields.sourceRef')">{{ sourceReference }}</AppDetailItem>
      </AppDetails>
      <div class="table-actions"><AppButton variant="default" @click="acceptUnverifiedSource">{{ t('plugins.managementUi.confirmAction') }}</AppButton></div>
    </section>

    <RetryPanel v-else-if="fatalError" :title="t('plugins.managementUi.loadFailed')" :description="fatalError" :loading="false" @retry="restartFrame" />

    <div v-else class="plugin-management-ui-frame-shell">
      <AppLoadingPanel :busy="waitingForLoad" :label="busyLabel">
        <iframe
          v-if="canRenderIframe"
          :key="iframeKey"
          ref="iframeRef"
          class="plugin-management-ui-frame"
          :src="frameSrc"
          data-testid="plugin-management-ui-frame"
          :title="title"
          @load="handleFrameLoad"
        />
      </AppLoadingPanel>
    </div>
  </section>
</template>

<style scoped lang="scss">
.plugin-management-ui-host { display: flex; flex: 1 1 0; flex-direction: column; min-height: 0; }
.plugin-management-ui-confirm { display: grid; gap: 16px; }
/* Confirming an unverified source is a decision for the operator, so the note uses the attention tone. */
.plugin-management-ui-confirm-note { display: grid; gap: 6px; padding: 12px 14px; border: 1px solid transparent; border-radius: var(--radius-md); background: var(--surface-attention); }
.plugin-management-ui-confirm-note strong { color: var(--text-attention); }
.plugin-management-ui-confirm-note p { margin: 0; color: var(--text); }
.plugin-management-ui-frame-shell,
.plugin-management-ui-frame-shell :deep(.app-loading-panel),
.plugin-management-ui-frame-shell :deep(.app-loading-panel__content) { display: flex; flex: 1 1 0; flex-direction: column; min-height: 0; width: 100%; }
.plugin-management-ui-frame { display: block; flex: 1 1 0; width: 100%; min-height: 0; border: 1px solid var(--border); border-radius: var(--radius-lg); background: var(--surface-strong); }
</style>
