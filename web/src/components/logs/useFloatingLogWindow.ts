import { computed, onBeforeUnmount, ref, watch, type Ref } from 'vue'
import { useMediaQuery, useResizeObserver } from '@vueuse/core'

import { breakpoints } from '@/preferences/breakpoints.generated'
import {
  readLogDetailWindowPosition,
  writeLogDetailWindowPosition,
  type LogDetailWindowPosition,
} from './log-detail-window-memory'

const safeInset = 12
const preferredWidth = 680
const maxWidth = 720
const maxHeight = 860
// The window may only be dragged a short way left from the right edge, never over the list's left half.
const horizontalDrift = 72

interface FloatingLogWindowOptions {
  hostElement: () => HTMLElement | null | undefined
  memoryKey: () => string
  open: () => boolean
  panel: Ref<HTMLElement | null>
  handle: Ref<HTMLElement | null>
}

function clamp(value: number, min: number, max: number) {
  if (max <= min) {
    return min
  }

  return Math.min(Math.max(value, min), max)
}

// Positions a non-modal log detail window inside its host and remembers where it was dragged.
export function useFloatingLogWindow(options: FloatingLogWindowOptions) {
  const isNarrowScreen = useMediaQuery(`(max-width: ${breakpoints.protocolPanel}px)`)
  const hostWidth = ref(0)
  const hostHeight = ref(0)
  const dragging = ref(false)
  const position = ref<LogDetailWindowPosition>({ left: safeInset, top: safeInset })

  const width = computed(() => {
    const availableWidth = hostWidth.value - safeInset * 2
    if (availableWidth <= 0) {
      return preferredWidth
    }

    return Math.min(preferredWidth, maxWidth, availableWidth)
  })
  const height = computed(() => {
    const availableHeight = hostHeight.value - safeInset * 2
    if (availableHeight <= 0) {
      return maxHeight
    }

    if (availableHeight < 220) {
      return availableHeight
    }

    return Math.min(maxHeight, availableHeight)
  })
  const defaultLeft = computed(() => Math.max(safeInset, hostWidth.value - width.value - safeInset))
  const leftBounds = computed(() => {
    const rightEdge = defaultLeft.value
    const leftHalfBoundary = Math.max(safeInset, hostWidth.value / 2)
    const availableDrift = Math.min(horizontalDrift, Math.max(0, rightEdge - leftHalfBoundary))

    return {
      min: rightEdge - availableDrift,
      max: rightEdge,
    }
  })
  const topBounds = computed(() => ({
    min: safeInset,
    max: Math.max(safeInset, hostHeight.value - height.value - safeInset),
  }))
  const floating = computed(() => (
    !isNarrowScreen.value
    && Boolean(options.hostElement())
    && hostWidth.value > 0
    && hostHeight.value > 0
  ))
  const windowStyle = computed(() => ({
    left: `${position.value.left}px`,
    top: `${position.value.top}px`,
    width: `${width.value}px`,
    height: `${height.value}px`,
  }))

  function clampPosition(nextPosition: LogDetailWindowPosition) {
    return {
      left: clamp(nextPosition.left, leftBounds.value.min, leftBounds.value.max),
      top: clamp(nextPosition.top, topBounds.value.min, topBounds.value.max),
    }
  }

  function defaultPosition() {
    return clampPosition({ left: leftBounds.value.max, top: safeInset })
  }

  function updateHostMetrics() {
    const host = options.hostElement()
    if (!host) {
      hostWidth.value = 0
      hostHeight.value = 0
      return
    }

    const rect = host.getBoundingClientRect()
    hostWidth.value = Math.max(0, Math.round(rect.width || host.clientWidth))
    hostHeight.value = Math.max(0, Math.round(rect.height || host.clientHeight))
  }

  function rememberPosition(nextPosition: LogDetailWindowPosition) {
    const clamped = clampPosition(nextPosition)
    position.value = clamped
    writeLogDetailWindowPosition(options.memoryKey(), clamped)
  }

  function restorePosition() {
    rememberPosition(readLogDetailWindowPosition(options.memoryKey()) ?? defaultPosition())
  }

  let activePointerId: number | null = null
  let dragOffsetX = 0
  let dragOffsetY = 0
  let previousBodyCursor = ''
  let previousBodyUserSelect = ''

  function handleWindowPointerMove(event: PointerEvent) {
    const host = options.hostElement()
    if (activePointerId === null || event.pointerId !== activePointerId || !host) {
      return
    }

    const hostRect = host.getBoundingClientRect()
    rememberPosition({
      left: event.clientX - hostRect.left - dragOffsetX,
      top: event.clientY - hostRect.top - dragOffsetY,
    })
  }

  function handleWindowPointerUp(event: PointerEvent) {
    stopDragging(event.pointerId)
  }

  function stopDragging(pointerId?: number) {
    if (pointerId !== undefined && activePointerId !== pointerId) {
      return
    }

    if (activePointerId !== null && options.handle.value?.releasePointerCapture) {
      options.handle.value.releasePointerCapture(activePointerId)
    }

    const wasDragging = dragging.value
    activePointerId = null
    dragging.value = false
    if (typeof window !== 'undefined') {
      window.removeEventListener('pointermove', handleWindowPointerMove)
      window.removeEventListener('pointerup', handleWindowPointerUp)
      window.removeEventListener('pointercancel', handleWindowPointerUp)
    }
    if (wasDragging) {
      document.body.style.userSelect = previousBodyUserSelect
      document.body.style.cursor = previousBodyCursor
    }
  }

  // Dragging starts from the window header, but not from its buttons or links.
  function startDragging(event: PointerEvent) {
    const host = options.hostElement()
    const panel = options.panel.value
    if (!options.open() || !floating.value || event.button !== 0 || !host || !panel) {
      return
    }

    const target = event.target
    if (target instanceof HTMLElement && target.closest('button, a, input, textarea, select, [role="button"]')) {
      return
    }

    const hostRect = host.getBoundingClientRect()
    const panelRect = panel.getBoundingClientRect()
    rememberPosition({ left: panelRect.left - hostRect.left, top: panelRect.top - hostRect.top })

    dragOffsetX = event.clientX - panelRect.left
    dragOffsetY = event.clientY - panelRect.top
    activePointerId = event.pointerId
    dragging.value = true
    options.handle.value?.setPointerCapture?.(event.pointerId)

    previousBodyUserSelect = document.body.style.userSelect
    previousBodyCursor = document.body.style.cursor
    document.body.style.userSelect = 'none'
    document.body.style.cursor = 'grabbing'
    window.addEventListener('pointermove', handleWindowPointerMove)
    window.addEventListener('pointerup', handleWindowPointerUp)
    window.addEventListener('pointercancel', handleWindowPointerUp)
    event.preventDefault()
  }

  watch(options.hostElement, updateHostMetrics, { immediate: true })
  useResizeObserver(options.hostElement, updateHostMetrics)

  // Keep a remembered or open window inside the host when either size changes.
  watch([hostWidth, hostHeight, width, height], () => {
    if (!floating.value) {
      return
    }

    const hasStoredPosition = Boolean(readLogDetailWindowPosition(options.memoryKey()))
    if (!options.open() && !hasStoredPosition) {
      return
    }

    rememberPosition(hasStoredPosition ? position.value : defaultPosition())
  })

  onBeforeUnmount(() => stopDragging())

  return {
    dragging,
    floating,
    restorePosition,
    startDragging,
    stopDragging,
    updateHostMetrics,
    windowStyle,
  }
}
