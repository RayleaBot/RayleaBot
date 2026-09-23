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
const reportedIframeHeight = ref(640)
const iframeHeight = ref(640)
const confirmed = ref(false)
const waitingForLoad = ref(false)
const fatalError = ref<string | null>(null)
let restartFrameWhenRuntimeReady = props.plugin.state === 'starting'
let loadTimer: ReturnType<typeof setTimeout> | null = null
let contentObserver: ResizeObserver | null = null
let frameMeasureAnimation: number | null = null

const minimumFrameHeight = 320
const maximumFrameHeight = 1600
const frameViewportBottomGap = 24

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

function stopContentObserver() {
  contentObserver?.disconnect()
  contentObserver = null
}

function failFrame(message: string) {
  clearLoadTimer()
  stopContentObserver()
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
  stopContentObserver()
  fatalError.value = null
  waitingForLoad.value = false
  reportedIframeHeight.value = 640
  iframeHeight.value = 640
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
  const content = documentElement
  clearLoadTimer()
  stopContentObserver()
  waitingForLoad.value = false
  const report = () => {
    reportedIframeHeight.value = Math.min(maximumFrameHeight, Math.max(minimumFrameHeight, Math.ceil(content.scrollHeight)))
    updateFrameHeight()
  }
  if (typeof ResizeObserver !== 'undefined') {
    contentObserver = new ResizeObserver(report)
    contentObserver.observe(content)
  }
  report()
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

if (typeof window !== 'undefined') {
  window.addEventListener('resize', scheduleFrameHeightUpdate)
  document.addEventListener('scroll', scheduleFrameHeightUpdate, true)
  window.visualViewport?.addEventListener('resize', scheduleFrameHeightUpdate)
  window.visualViewport?.addEventListener('scroll', scheduleFrameHeightUpdate)
}

onBeforeUnmount(() => {
  clearLoadTimer()
  stopContentObserver()
  if (typeof window !== 'undefined') {
    window.removeEventListener('resize', scheduleFrameHeightUpdate)
    document.removeEventListener('scroll', scheduleFrameHeightUpdate, true)
    window.visualViewport?.removeEventListener('resize', scheduleFrameHeightUpdate)
    window.visualViewport?.removeEventListener('scroll', scheduleFrameHeightUpdate)
    if (frameMeasureAnimation !== null) window.cancelAnimationFrame(frameMeasureAnimation)
  }
})
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
          :style="{ height: `${iframeHeight}px` }"
          data-testid="plugin-management-ui-frame"
          :title="title"
          @load="handleFrameLoad"
        />
      </AppLoadingPanel>
    </div>
  </section>
</template>

<style scoped lang="scss">
.plugin-management-ui-host { display: flex; flex: 0 0 auto; flex-direction: column; min-height: 0; }
.plugin-management-ui-confirm { display: grid; gap: 16px; }
.plugin-management-ui-confirm-note { display: grid; gap: 6px; padding: 12px 14px; border: 1px solid var(--border); border-radius: var(--radius-md); background: var(--surface-soft); }
.plugin-management-ui-confirm-note p { margin: 0; color: var(--muted); }
.plugin-management-ui-frame-shell,
.plugin-management-ui-frame-shell :deep(.app-loading-panel),
.plugin-management-ui-frame-shell :deep(.app-loading-panel__content) { display: flex; flex: 1 1 auto; min-height: 0; width: 100%; }
.plugin-management-ui-frame { width: 100%; min-height: 320px; max-height: 1600px; border: 1px solid var(--border); border-radius: var(--radius-lg); background: var(--surface-strong); }
</style>
