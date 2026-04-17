<script setup lang="ts">
import { CloseOutlined } from '@ant-design/icons-vue'
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'

import { getLogLevelLabel, getLogProtocolLabel } from '@/lib/display'
import { formatDateTime } from '@/lib/format'
import { t } from '@/i18n'
import type { LogDetailResponse, LogSummary } from '@/types/api'

import ManagementLogDetailContent from './ManagementLogDetailContent.vue'
import { readLogDetailWindowPosition, writeLogDetailWindowPosition } from './log-detail-window-memory'

const desktopBreakpoint = 960
const floatingWindowSafeInset = 12
const floatingWindowPreferredWidth = 680
const floatingWindowMaxWidth = 720
const floatingWindowMaxHeight = 860
const floatingWindowHorizontalDrift = 72

interface FloatingPosition {
  left: number
  top: number
}

interface SummaryChip {
  key: string
  label: string
  tone: 'debug' | 'info' | 'warn' | 'error' | 'neutral'
}

const props = defineProps<{
  open: boolean
  loading: boolean
  error: string | null
  summary: LogSummary | null
  detail: LogDetailResponse | null
  memoryKey: string
  hostElement?: HTMLElement | null
}>()

const emit = defineEmits<{
  close: []
}>()

const panelRef = ref<HTMLElement | null>(null)
const headerRef = ref<HTMLElement | null>(null)
const bodyRef = ref<HTMLElement | null>(null)
const isNarrowScreen = ref(false)
const hostWidth = ref(0)
const hostHeight = ref(0)
const dragging = ref(false)
const floatingPosition = ref<FloatingPosition>({
  left: floatingWindowSafeInset,
  top: floatingWindowSafeInset,
})

const titleId = computed(() => `management-log-detail-${props.memoryKey || 'window'}`)
const selectedLogKey = computed(() => props.summary?.log_id ?? 'log-detail')
const floatingWidth = computed(() => {
  const availableWidth = hostWidth.value - floatingWindowSafeInset * 2
  if (availableWidth <= 0) {
    return floatingWindowPreferredWidth
  }

  return Math.min(floatingWindowPreferredWidth, floatingWindowMaxWidth, availableWidth)
})
const floatingHeight = computed(() => {
  const availableHeight = hostHeight.value - floatingWindowSafeInset * 2
  if (availableHeight <= 0) {
    return floatingWindowMaxHeight
  }

  if (availableHeight < 220) {
    return availableHeight
  }

  return Math.min(floatingWindowMaxHeight, availableHeight)
})
const floatingDefaultLeft = computed(() => Math.max(
  floatingWindowSafeInset,
  hostWidth.value - floatingWidth.value - floatingWindowSafeInset,
))
const floatingLeftBounds = computed(() => {
  const rightEdge = floatingDefaultLeft.value
  const leftHalfBoundary = Math.max(floatingWindowSafeInset, hostWidth.value / 2)
  const availableDrift = Math.min(
    floatingWindowHorizontalDrift,
    Math.max(0, rightEdge - leftHalfBoundary),
  )

  return {
    min: rightEdge - availableDrift,
    max: rightEdge,
  }
})
const floatingTopBounds = computed(() => ({
  min: floatingWindowSafeInset,
  max: Math.max(
    floatingWindowSafeInset,
    hostHeight.value - floatingHeight.value - floatingWindowSafeInset,
  ),
}))
const useFloatingWindow = computed(() => (
  !isNarrowScreen.value
  && Boolean(props.hostElement)
  && hostWidth.value > 0
  && hostHeight.value > 0
))
const floatingWindowStyle = computed(() => ({
  left: `${floatingPosition.value.left}px`,
  top: `${floatingPosition.value.top}px`,
  width: `${floatingWidth.value}px`,
  height: `${floatingHeight.value}px`,
}))
const summaryChips = computed<SummaryChip[]>(() => {
  if (!props.summary) {
    return []
  }

  const chips: SummaryChip[] = []
  if (props.summary.level) {
    const tone = props.summary.level === 'error'
      ? 'error'
      : props.summary.level === 'warn'
        ? 'warn'
        : props.summary.level === 'info'
          ? 'info'
          : 'debug'

    chips.push({
      key: 'level',
      label: getLogLevelLabel(props.summary.level),
      tone,
    })
  }

  if (props.summary.protocol) {
    chips.push({
      key: 'protocol',
      label: getLogProtocolLabel(props.summary.protocol),
      tone: 'neutral',
    })
  }

  return chips
})

let hostResizeObserver: ResizeObserver | null = null
let mediaQueryList: MediaQueryList | null = null
let activePointerId: number | null = null
let dragOffsetX = 0
let dragOffsetY = 0
let previousBodyCursor = ''
let previousBodyUserSelect = ''

function clamp(value: number, min: number, max: number) {
  if (max <= min) {
    return min
  }

  return Math.min(Math.max(value, min), max)
}

function clampFloatingPosition(nextPosition: FloatingPosition) {
  return {
    left: clamp(nextPosition.left, floatingLeftBounds.value.min, floatingLeftBounds.value.max),
    top: clamp(nextPosition.top, floatingTopBounds.value.min, floatingTopBounds.value.max),
  }
}

function defaultFloatingPosition() {
  return clampFloatingPosition({
    left: floatingLeftBounds.value.max,
    top: floatingWindowSafeInset,
  })
}

function syncScreenMode() {
  if (typeof window === 'undefined') {
    return
  }

  if (typeof window.matchMedia === 'function') {
    if (!mediaQueryList) {
      mediaQueryList = window.matchMedia(`(max-width: ${desktopBreakpoint}px)`)
    }
    isNarrowScreen.value = mediaQueryList.matches
    return
  }

  isNarrowScreen.value = window.innerWidth <= desktopBreakpoint
}

function handleMediaQueryChange(event: MediaQueryListEvent) {
  isNarrowScreen.value = event.matches
}

function disconnectMediaQuery() {
  if (!mediaQueryList) {
    return
  }

  if (typeof mediaQueryList.removeEventListener === 'function') {
    mediaQueryList.removeEventListener('change', handleMediaQueryChange)
  } else {
    mediaQueryList.removeListener(handleMediaQueryChange)
  }
  mediaQueryList = null
}

function connectMediaQuery() {
  if (typeof window === 'undefined' || typeof window.matchMedia !== 'function') {
    syncScreenMode()
    return
  }

  disconnectMediaQuery()
  mediaQueryList = window.matchMedia(`(max-width: ${desktopBreakpoint}px)`)
  isNarrowScreen.value = mediaQueryList.matches

  if (typeof mediaQueryList.addEventListener === 'function') {
    mediaQueryList.addEventListener('change', handleMediaQueryChange)
  } else {
    mediaQueryList.addListener(handleMediaQueryChange)
  }
}

function updateHostMetrics() {
  const host = props.hostElement
  if (!host) {
    hostWidth.value = 0
    hostHeight.value = 0
    return
  }

  const rect = host.getBoundingClientRect()
  hostWidth.value = Math.max(0, Math.round(rect.width || host.clientWidth))
  hostHeight.value = Math.max(0, Math.round(rect.height || host.clientHeight))
}

function disconnectHostObserver() {
  hostResizeObserver?.disconnect()
  hostResizeObserver = null
}

function connectHostObserver() {
  disconnectHostObserver()
  updateHostMetrics()

  const host = props.hostElement
  if (!host || typeof window === 'undefined' || typeof window.ResizeObserver !== 'function') {
    return
  }

  hostResizeObserver = new window.ResizeObserver(() => {
    updateHostMetrics()
  })
  hostResizeObserver.observe(host)
}

function rememberFloatingPosition(nextPosition: FloatingPosition) {
  const clamped = clampFloatingPosition(nextPosition)
  floatingPosition.value = clamped
  writeLogDetailWindowPosition(props.memoryKey, clamped)
}

function restoreFloatingPosition() {
  const stored = readLogDetailWindowPosition(props.memoryKey)
  rememberFloatingPosition(stored ?? defaultFloatingPosition())
}

function syncFloatingPositionFromPanel() {
  const panel = panelRef.value
  const host = props.hostElement
  if (!panel || !host) {
    return
  }

  const hostRect = host.getBoundingClientRect()
  const panelRect = panel.getBoundingClientRect()
  rememberFloatingPosition({
    left: panelRect.left - hostRect.left,
    top: panelRect.top - hostRect.top,
  })
}

function applyDragDocumentState() {
  previousBodyUserSelect = document.body.style.userSelect
  previousBodyCursor = document.body.style.cursor
  document.body.style.userSelect = 'none'
  document.body.style.cursor = 'grabbing'
}

function resetDragDocumentState() {
  document.body.style.userSelect = previousBodyUserSelect
  document.body.style.cursor = previousBodyCursor
}

function removeDragListeners() {
  if (typeof window === 'undefined') {
    return
  }

  window.removeEventListener('pointermove', handleWindowPointerMove)
  window.removeEventListener('pointerup', handleWindowPointerUp)
  window.removeEventListener('pointercancel', handleWindowPointerUp)
}

function stopDragging(pointerId?: number) {
  if (pointerId !== undefined && activePointerId !== pointerId) {
    return
  }

  if (activePointerId !== null && headerRef.value?.releasePointerCapture) {
    headerRef.value.releasePointerCapture(activePointerId)
  }

  activePointerId = null
  dragging.value = false
  removeDragListeners()
  resetDragDocumentState()
}

function handleWindowPointerMove(event: PointerEvent) {
  if (activePointerId === null || event.pointerId !== activePointerId || !props.hostElement) {
    return
  }

  const hostRect = props.hostElement.getBoundingClientRect()
  rememberFloatingPosition({
    left: event.clientX - hostRect.left - dragOffsetX,
    top: event.clientY - hostRect.top - dragOffsetY,
  })
}

function handleWindowPointerUp(event: PointerEvent) {
  stopDragging(event.pointerId)
}

function handleHeaderPointerDown(event: PointerEvent) {
  if (!props.open || !useFloatingWindow.value || event.button !== 0 || !props.hostElement) {
    return
  }

  const target = event.target
  if (target instanceof HTMLElement && target.closest('button, a, input, textarea, select, [role="button"]')) {
    return
  }

  const panel = panelRef.value
  if (!panel) {
    return
  }

  syncFloatingPositionFromPanel()

  const panelRect = panel.getBoundingClientRect()
  dragOffsetX = event.clientX - panelRect.left
  dragOffsetY = event.clientY - panelRect.top
  activePointerId = event.pointerId
  dragging.value = true

  if (headerRef.value?.setPointerCapture) {
    headerRef.value.setPointerCapture(event.pointerId)
  }

  applyDragDocumentState()
  window.addEventListener('pointermove', handleWindowPointerMove)
  window.addEventListener('pointerup', handleWindowPointerUp)
  window.addEventListener('pointercancel', handleWindowPointerUp)
  event.preventDefault()
}

function handleWindowKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape' && props.open) {
    emit('close')
  }
}

watch(
  () => props.hostElement,
  () => {
    connectHostObserver()
  },
  { immediate: true },
)

watch(
  [hostWidth, hostHeight, floatingWidth, floatingHeight],
  () => {
    if (!useFloatingWindow.value) {
      return
    }

    const hasStoredPosition = Boolean(readLogDetailWindowPosition(props.memoryKey))
    if (!props.open && !hasStoredPosition) {
      return
    }

    rememberFloatingPosition(hasStoredPosition ? floatingPosition.value : defaultFloatingPosition())
  },
)

watch(
  () => props.open,
  async (open) => {
    if (typeof window !== 'undefined') {
      window.removeEventListener('keydown', handleWindowKeydown)
      if (open) {
        window.addEventListener('keydown', handleWindowKeydown)
      }
    }

    if (!open) {
      stopDragging()
      return
    }

    updateHostMetrics()
    if (!useFloatingWindow.value) {
      return
    }

    restoreFloatingPosition()
    await nextTick()
    panelRef.value?.focus()
  },
  { immediate: true },
)

watch(
  useFloatingWindow,
  async (floating) => {
    if (!props.open || !floating) {
      return
    }

    updateHostMetrics()
    restoreFloatingPosition()
    await nextTick()
    panelRef.value?.focus()
  },
)

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

onMounted(() => {
  connectMediaQuery()
  syncScreenMode()
})

onBeforeUnmount(() => {
  stopDragging()
  disconnectHostObserver()
  disconnectMediaQuery()
  if (typeof window !== 'undefined') {
    window.removeEventListener('keydown', handleWindowKeydown)
  }
})
</script>

<template>
  <a-drawer
    v-if="!useFloatingWindow"
    :open="open"
    :get-container="false"
    placement="right"
    width="min(720px, 92vw)"
    :title="t('logs.detail.title')"
    class="log-detail-drawer"
    @close="emit('close')"
  >
    <ManagementLogDetailContent
      :loading="loading"
      :error="error"
      :summary="summary"
      :detail="detail"
    />
  </a-drawer>

  <Transition name="log-detail-window">
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
        @pointerdown="handleHeaderPointerDown"
      >
        <div class="log-detail-window__handle" aria-hidden="true">
          <span />
          <span />
          <span />
        </div>

        <div class="log-detail-window__heading">
          <div class="log-detail-window__eyebrow">{{ t('logs.detail.title') }}</div>
          <div class="log-detail-window__title-row">
            <h2 :id="titleId">{{ summary?.source || t('display.empty') }}</h2>
            <div v-if="summaryChips.length" class="log-detail-window__chips">
              <span
                v-for="chip in summaryChips"
                :key="chip.key"
                class="log-detail-window__chip"
                :class="`is-${chip.tone}`"
              >
                {{ chip.label }}
              </span>
            </div>
          </div>
          <p class="log-detail-window__subtitle">
            {{ summary ? formatDateTime(summary.timestamp) : t('display.empty') }}
          </p>
        </div>

        <button
          type="button"
          class="log-detail-window__close"
          :aria-label="t('logs.detail.close')"
          @pointerdown.stop
          @click="emit('close')"
        >
          <CloseOutlined />
        </button>
      </header>

      <div ref="bodyRef" class="log-detail-window__body">
        <Transition name="log-detail-window-content" mode="out-in">
          <div :key="selectedLogKey" class="log-detail-window__content">
            <ManagementLogDetailContent
              :loading="loading"
              :error="error"
              :summary="summary"
              :detail="detail"
            />
          </div>
        </Transition>
      </div>
    </section>
  </Transition>
</template>

<style lang="scss" scoped>
.log-detail-drawer :deep(.ant-drawer-body) {
  padding: 16px;
  background:
    linear-gradient(180deg, color-mix(in srgb, var(--surface-soft) 72%, transparent), transparent 24%),
    var(--surface-strong);
}

.log-detail-window {
  position: absolute;
  z-index: 12;
  display: flex;
  flex-direction: column;
  min-height: 0;
  border-radius: 20px;
  border: 1px solid color-mix(in srgb, var(--border-strong) 82%, var(--border));
  background:
    linear-gradient(180deg, color-mix(in srgb, var(--surface-strong) 98%, transparent), color-mix(in srgb, var(--surface-soft) 80%, transparent));
  box-shadow:
    0 22px 50px color-mix(in srgb, var(--text) 12%, transparent),
    0 6px 18px color-mix(in srgb, var(--app-primary) 10%, transparent);
  overflow: hidden;
}

.log-detail-window__header {
  display: flex;
  align-items: flex-start;
  gap: 14px;
  padding: 16px 18px 14px;
  border-bottom: 1px solid color-mix(in srgb, var(--border) 92%, transparent);
  background:
    linear-gradient(180deg, color-mix(in srgb, var(--surface-strong) 96%, transparent), color-mix(in srgb, var(--surface-soft) 82%, transparent));
  cursor: grab;
  user-select: none;
}

.log-detail-window.is-dragging .log-detail-window__header {
  cursor: grabbing;
}

.log-detail-window__handle {
  flex: 0 0 auto;
  display: grid;
  gap: 4px;
  padding-top: 5px;
}

.log-detail-window__handle span {
  display: block;
  width: 14px;
  height: 2px;
  border-radius: 999px;
  background: color-mix(in srgb, var(--muted) 72%, transparent);
}

.log-detail-window__heading {
  min-width: 0;
  flex: 1 1 auto;
}

.log-detail-window__eyebrow {
  color: var(--muted);
  font-size: 0.72rem;
  font-weight: 700;
  letter-spacing: 0.08em;
  text-transform: uppercase;
}

.log-detail-window__title-row {
  display: flex;
  align-items: center;
  gap: 10px;
  flex-wrap: wrap;
  margin-top: 4px;
}

.log-detail-window__title-row h2 {
  margin: 0;
  color: var(--text);
  font-size: 1.04rem;
  line-height: 1.25;
  letter-spacing: -0.02em;
}

.log-detail-window__chips {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-wrap: wrap;
}

.log-detail-window__chip {
  display: inline-flex;
  align-items: center;
  min-height: 26px;
  max-width: 100%;
  padding: 0 10px;
  border-radius: 999px;
  border: 1px solid color-mix(in srgb, var(--border) 92%, transparent);
  background: color-mix(in srgb, var(--surface-soft) 92%, transparent);
  color: var(--text);
  font-size: 0.74rem;
  font-weight: 600;
  line-height: 1.2;
}

.log-detail-window__chip.is-debug {
  background: color-mix(in srgb, var(--surface-soft) 96%, transparent);
  color: var(--muted);
}

.log-detail-window__chip.is-info {
  background: color-mix(in srgb, var(--app-primary) 10%, var(--surface-soft));
  color: color-mix(in srgb, var(--app-primary) 84%, var(--text));
}

.log-detail-window__chip.is-warn {
  background: color-mix(in srgb, var(--app-warning) 12%, var(--surface-soft));
  color: color-mix(in srgb, var(--app-warning) 86%, var(--text));
}

.log-detail-window__chip.is-error {
  background: color-mix(in srgb, var(--app-danger) 12%, var(--surface-soft));
  color: color-mix(in srgb, var(--app-danger) 86%, var(--text));
}

.log-detail-window__chip.is-neutral {
  background: color-mix(in srgb, var(--surface-soft) 88%, transparent);
  color: var(--text);
}

.log-detail-window__subtitle {
  font-family: "Cascadia Mono", "Consolas", monospace;
}

.log-detail-window__subtitle {
  margin: 8px 0 0;
  color: var(--muted);
  font-size: 0.78rem;
  line-height: 1.5;
}

.log-detail-window__close {
  flex: 0 0 auto;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 36px;
  height: 36px;
  padding: 0;
  border: 1px solid color-mix(in srgb, var(--border) 92%, transparent);
  border-radius: 12px;
  background: color-mix(in srgb, var(--surface-soft) 92%, transparent);
  color: var(--muted);
  cursor: pointer;
  transition: border-color 0.2s ease, background-color 0.2s ease, color 0.2s ease, transform 0.12s ease;
}

.log-detail-window__close:hover {
  border-color: color-mix(in srgb, var(--app-primary) 18%, var(--border));
  background: color-mix(in srgb, var(--app-primary) 10%, var(--surface-soft));
  color: var(--text);
}

.log-detail-window__close:active {
  transform: scale(0.96);
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
  transition: opacity 0.18s ease, transform 0.18s ease;
}

.log-detail-window-enter-from,
.log-detail-window-leave-to {
  opacity: 0;
  transform: translate3d(18px, 0, 0);
}

.log-detail-window-content-enter-active,
.log-detail-window-content-leave-active {
  transition: opacity 0.14s ease;
}

.log-detail-window-content-enter-from,
.log-detail-window-content-leave-to {
  opacity: 0;
}
</style>
