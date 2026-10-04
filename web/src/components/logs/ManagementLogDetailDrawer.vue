<script setup lang="ts">
import AppButton from '@/components/AppButton.vue'
import AppDrawer from '@/components/AppDrawer.vue'
import AppTag from '@/components/AppTag.vue'
import { GripHorizontalIcon, XIcon } from '@lucide/vue'
import { computed, shallowRef, nextTick, ref, watch } from 'vue'
import { useEventListener } from '@vueuse/core'

import { getLogLevelLabel, getLogProtocolLabel } from '@/lib/display'
import { formatDateTime } from '@/lib/format'
import { t } from '@/i18n'
import type { LogScope } from '@/stores/log-state'
import type { LogDetailResponse, LogSummary } from '@/types/api'

import ManagementLogDetailContent from './ManagementLogDetailContent.vue'
import { useFloatingLogWindow } from './useFloatingLogWindow'

interface SummaryChip {
  key: string
  label: string
  tone: 'neutral' | 'info' | 'warning' | 'danger'
}

const props = defineProps<{
  open: boolean
  loading: boolean
  error: string | null
  summary: LogSummary | null
  detail: LogDetailResponse | null
  scope: LogScope
  memoryKey: string
  hostElement?: HTMLElement | null
}>()

const emit = defineEmits<{
  close: []
}>()

// Keep the last content while the window or drawer plays its exit transition.
const retained = shallowRef({ summary: props.summary, detail: props.detail, loading: props.loading, error: props.error })
watch(() => [props.open, props.summary, props.detail, props.loading, props.error], () => {
  if (props.open) retained.value = { summary: props.summary, detail: props.detail, loading: props.loading, error: props.error }
}, { immediate: true })
const displaySummary = computed(() => props.open ? props.summary : retained.value.summary)
const displayDetail = computed(() => props.open ? props.detail : retained.value.detail)
const displayLoading = computed(() => props.open ? props.loading : retained.value.loading)
const displayError = computed(() => props.open ? props.error : retained.value.error)
let floatingTrigger: HTMLElement | null = null
let focusVersion = 0
function finishClose() {
  if (props.open) return
  retained.value = { summary: null, detail: null, loading: false, error: null }
  if (document.activeElement === document.body && floatingTrigger?.isConnected && floatingTrigger.getClientRects().length) floatingTrigger.focus({ preventScroll: true })
  floatingTrigger = null
}

const panelRef = ref<HTMLElement | null>(null)
const headerRef = ref<HTMLElement | null>(null)
const bodyRef = ref<HTMLElement | null>(null)
const {
  dragging,
  floating: useFloatingWindow,
  restorePosition,
  startDragging,
  stopDragging,
  updateHostMetrics,
  windowStyle: floatingWindowStyle,
} = useFloatingLogWindow({
  hostElement: () => props.hostElement,
  memoryKey: () => props.memoryKey,
  open: () => props.open,
  panel: panelRef,
  handle: headerRef,
})

const titleId = computed(() => `management-log-detail-${props.memoryKey || 'window'}`)
const selectedLogKey = computed(() => displaySummary.value?.log_id ?? 'log-detail')
const summaryChips = computed<SummaryChip[]>(() => {
  const summary = displaySummary.value
  if (!summary) {
    return []
  }

  // The same tones as the level tags in the list.
  const chips: SummaryChip[] = []
  if (summary.level) {
    chips.push({
      key: 'level',
      label: getLogLevelLabel(summary.level),
      tone: summary.level === 'error' ? 'danger' : summary.level === 'warn' ? 'warning' : summary.level === 'info' ? 'info' : 'neutral',
    })
  }

  if (summary.protocol) {
    chips.push({
      key: 'protocol',
      label: getLogProtocolLabel(summary.protocol),
      tone: 'neutral',
    })
  }

  return chips
})

async function focusFloatingWindow() {
  const version = ++focusVersion
  updateHostMetrics()
  restorePosition()
  await nextTick()
  if (version === focusVersion && props.open && useFloatingWindow.value && panelRef.value?.isConnected) {
    panelRef.value.focus({ preventScroll: true })
  }
}

useEventListener(window, 'keydown', (event: KeyboardEvent) => {
  if (event.key === 'Escape' && props.open) {
    emit('close')
  }
})

watch(
  () => props.open,
  async (open) => {
    if (!open) {
      focusVersion += 1
      stopDragging()
      return
    }

    updateHostMetrics()
    if (!useFloatingWindow.value) {
      return
    }

    floatingTrigger = document.activeElement instanceof HTMLElement ? document.activeElement : null
    await focusFloatingWindow()
  },
  { immediate: true },
)

watch(useFloatingWindow, (floating) => {
  if (props.open && floating) void focusFloatingWindow()
})

// A newly selected log starts reading from the top.
watch(
  () => props.summary?.log_id,
  async (nextLogId, previousLogId) => {
    if (!nextLogId || nextLogId === previousLogId) {
      return
    }

    await nextTick()
    if (bodyRef.value) {
      bodyRef.value.scrollTop = 0
    }
  },
)
</script>

<template>
  <AppDrawer
    v-if="!useFloatingWindow"
    :open="open"
    placement="right"
    :width="720"
    :title="t('logs.detail.title')"
    class="log-detail-drawer"
    @close="emit('close')" @after-close="finishClose"
  >
    <ManagementLogDetailContent
      :loading="displayLoading"
      :error="displayError"
      :summary="displaySummary"
      :detail="displayDetail"
      :scope="scope"
      @action="emit('close')"
    />
  </AppDrawer>

  <Transition name="log-detail-window" @after-leave="finishClose">
    <section
      v-if="open && useFloatingWindow"
      ref="panelRef"
      data-testid="management-log-detail-window"
      class="log-detail-window"
      :class="{ 'is-dragging': dragging }"
      :style="floatingWindowStyle"
      role="dialog"
      aria-modal="false"
      :aria-labelledby="titleId"
      tabindex="-1"
    >
      <header
        ref="headerRef"
        class="log-detail-window__header"
        @pointerdown="startDragging"
      >
        <GripHorizontalIcon class="log-detail-window__handle" :size="16" aria-hidden="true" />

        <div class="log-detail-window__heading">
          <h2 :id="titleId">{{ t('logs.detail.title') }}</h2>
          <div class="log-detail-window__title-row">
            <strong class="log-detail-window__source">{{ displaySummary?.source || t('display.empty') }}</strong>
            <div v-if="summaryChips.length" class="log-detail-window__chips">
              <AppTag v-for="chip in summaryChips" :key="chip.key" size="small" :tone="chip.tone">{{ chip.label }}</AppTag>
            </div>
          </div>
          <p class="log-detail-window__subtitle">
            {{ displaySummary ? formatDateTime(displaySummary.timestamp) : t('display.empty') }}
          </p>
        </div>

        <AppButton
          variant="ghost"
          size="icon"
          class="log-detail-window__close"
          :aria-label="t('logs.detail.close')"
          @pointerdown.stop
          @click="emit('close')"
        >
          <XIcon />
        </AppButton>
      </header>

      <div ref="bodyRef" class="log-detail-window__body">
        <Transition name="log-detail-window-content" mode="out-in">
          <div :key="selectedLogKey" class="log-detail-window__content">
            <ManagementLogDetailContent
              :loading="displayLoading"
              :error="displayError"
              :summary="displaySummary"
              :detail="displayDetail"
              :scope="scope"
              summary-in-header
              @action="emit('close')"
            />
          </div>
        </Transition>
      </div>
    </section>
  </Transition>
</template>

<style lang="scss" scoped>
.log-detail-drawer :deep(.app-dialog__body) {
  padding: 16px;
  background: var(--surface-strong);
}

// A floating box like the dialogs: the gray surface and the floating shadow, no border and no header band.
.log-detail-window {
  position: absolute;
  z-index: 12;
  display: flex;
  flex-direction: column;
  min-height: 0;
  border-radius: var(--radius-xl);
  background: var(--surface-strong);
  box-shadow: var(--shadow-floating);
  overflow: hidden;
  --control-fill: var(--surface-raised);
  --control-fill-hover: color-mix(in srgb, var(--surface-raised) 97%, var(--text));
}

.log-detail-window__header {
  display: flex;
  align-items: flex-start;
  gap: 12px;
  padding: 16px 16px 14px 18px;
  border-bottom: 1px solid var(--border);
  cursor: grab;
  user-select: none;
}

.log-detail-window.is-dragging .log-detail-window__header {
  cursor: grabbing;
}

.log-detail-window__handle {
  flex: 0 0 auto;
  margin-top: 3px;
  color: var(--muted);
}

.log-detail-window__heading {
  min-width: 0;
  flex: 1 1 auto;
}

.log-detail-window__title-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-top: 4px;
}

.log-detail-window__heading h2 {
  margin: 0;
  color: var(--text);
  font-size: 16px;
  font-weight: 700;
  line-height: 1.35;
}

.log-detail-window__source {
  min-width: 0;
  color: var(--muted);
  font-family: var(--font-mono);
  font-size: 13px;
  font-weight: 500;
  overflow-wrap: anywhere;
}

.log-detail-window__chips {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.log-detail-window__subtitle {
  margin: 6px 0 0;
  color: var(--muted);
  font-family: var(--font-mono);
  font-size: 13px;
  line-height: 1.5;
}

.log-detail-window__close {
  flex: 0 0 auto;
  cursor: pointer;
}

@media (forced-colors: active) {
  .log-detail-window { border: 1px solid CanvasText; }
}

.log-detail-window__body {
  flex: 1 1 auto;
  min-height: 0;
  overflow: auto;
  padding: 16px 18px 18px;
}

.log-detail-window__content {
  min-height: 100%;
}

.log-detail-window-enter-active,
.log-detail-window-leave-active {
  transition: opacity var(--motion-overlay) var(--motion-easing), transform var(--motion-overlay) var(--motion-easing);
}

.log-detail-window-enter-from,
.log-detail-window-leave-to {
  opacity: 0;
  transform: translate3d(18px, 0, 0);
}

.log-detail-window-content-enter-active,
.log-detail-window-content-leave-active {
  transition: opacity var(--motion-fast) var(--motion-easing);
}

.log-detail-window-content-enter-from,
.log-detail-window-content-leave-to {
  opacity: 0;
}
</style>
