<script setup lang="ts" generic="T">
import { computed, nextTick, onActivated, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useVirtualizer, type Rect, type VirtualItem, type Virtualizer } from '@tanstack/vue-virtual'

interface Props<TItem> {
  items: TItem[]
  itemHeight?: number
  viewportHeight?: number | string
  dynamicItemHeight?: boolean
  overscan?: number
  followBottom?: boolean
  topThreshold?: number
  bottomThreshold?: number
  emptyLabel?: string
  getItemKey?: (item: TItem, index: number) => string | number
}

const props = withDefaults(defineProps<Props<T>>(), {
  itemHeight: 160,
  dynamicItemHeight: false,
  followBottom: false,
  topThreshold: 16,
  bottomThreshold: 24,
  overscan: 3,
  emptyLabel: '暂无数据',
  getItemKey: undefined,
})

const emit = defineEmits<{
  'at-bottom-change': [value: boolean]
  'reach-top': []
}>()

const scrollerRef = ref<HTMLElement | null>(null)
const scrollTop = ref(0)
const measuredViewportHeight = ref<number | null>(null)
const measurementsSettled = ref(!props.dynamicItemHeight)
let measurementsSettledToken = 0
let lastAtBottom = true
let topReachArmed = false
let followBottomPausedByUser = false
let pendingProgrammaticScrollEvents = 0
let pendingProgrammaticScrollResetHandle: number | null = null
let lastProgrammaticScrollTop: number | null = null
let pendingPrependedAnchorRestoreToken = 0
let previousFirstKey = firstItemKey()
let previousLastKey = lastItemKey()

const viewportStyle = computed(() => {
  if (props.viewportHeight === undefined) {
    return undefined
  }

  return {
    height: typeof props.viewportHeight === 'number' ? `${props.viewportHeight}px` : props.viewportHeight,
  }
})

const effectiveViewportHeight = computed(() => {
  if (measuredViewportHeight.value && measuredViewportHeight.value > 0) {
    return measuredViewportHeight.value
  }

  if (typeof props.viewportHeight === 'number') {
    return props.viewportHeight
  }

  if (typeof props.viewportHeight === 'string') {
    const parsed = Number.parseInt(props.viewportHeight, 10)
    if (Number.isFinite(parsed) && parsed > 0) {
      return parsed
    }
  }

  return 560
})

const rowVirtualizer = useVirtualizer<HTMLElement, HTMLElement>(computed(() => ({
  count: props.items.length,
  getScrollElement: () => scrollerRef.value,
  estimateSize: () => props.itemHeight,
  initialRect: { width: 0, height: effectiveViewportHeight.value },
  observeElementRect: observeScrollerRect,
  overscan: props.overscan,
  getItemKey: (index) => resolveKey(props.items[index]!, index),
  measureElement: props.dynamicItemHeight
    ? (element, entry, instance) => {
        const fallbackHeight = instance.options.estimateSize(0)
        if (entry?.borderBoxSize) {
          const box = entry.borderBoxSize[0]
          if (box) {
            return normalizeMeasuredItemHeight(box.blockSize, fallbackHeight)
          }
        }

        const rectHeight = element.getBoundingClientRect().height
        if (rectHeight > 0) {
          return normalizeMeasuredItemHeight(rectHeight, fallbackHeight)
        }

        const measuredHeight = element.offsetHeight || element.scrollHeight
        return normalizeMeasuredItemHeight(measuredHeight, fallbackHeight)
      }
    : undefined,
  useAnimationFrameWithResizeObserver: props.dynamicItemHeight,
})))

const virtualItems = computed(() => rowVirtualizer.value.getVirtualItems())
const totalHeight = computed(() => rowVirtualizer.value.getTotalSize())
const visibleRows = computed(() => (
  virtualItems.value
    .map((virtualItem) => {
      const item = props.items[virtualItem.index]
      return item === undefined ? null : { item, virtualItem }
    })
    .filter((row): row is { item: T, virtualItem: VirtualItem } => row !== null)
))

function normalizeMeasuredItemHeight(nextHeight: number, fallbackHeight: number) {
  if (!Number.isFinite(nextHeight) || nextHeight <= 0) {
    return fallbackHeight
  }

  const nearestPixel = Math.round(nextHeight)
  if (Math.abs(nextHeight - nearestPixel) <= 0.1) {
    return Math.max(1, nearestPixel)
  }

  return Math.max(1, nextHeight)
}

function resolveKey(item: T, index: number) {
  if (props.getItemKey) {
    return props.getItemKey(item, index)
  }

  return index
}

function firstItemKey() {
  return props.items.length > 0 ? resolveKey(props.items[0]!, 0) : null
}

function lastItemKey() {
  const lastIndex = props.items.length - 1
  return lastIndex >= 0 ? resolveKey(props.items[lastIndex]!, lastIndex) : null
}

function updateMeasuredViewportHeight(nextHeight?: number) {
  if (!nextHeight || Number.isNaN(nextHeight)) {
    return
  }

  measuredViewportHeight.value = Math.max(1, Math.round(nextHeight))
}

function readScrollerRect(element: HTMLElement): Rect {
  const rect = element.getBoundingClientRect()
  return {
    width: Math.round(rect.width || element.clientWidth || 0),
    height: Math.max(1, Math.round(rect.height || element.clientHeight || effectiveViewportHeight.value)),
  }
}

function observeScrollerRect(
  instance: Virtualizer<HTMLElement, HTMLElement>,
  callback: (rect: Rect) => void,
) {
  const element = instance.scrollElement
  if (!element) {
    return undefined
  }

  const notify = (rect: Rect) => {
    updateMeasuredViewportHeight(rect.height)
    callback(rect)
  }

  notify(readScrollerRect(element))

  const targetWindow = element.ownerDocument.defaultView
  if (!targetWindow?.ResizeObserver) {
    return undefined
  }

  const observer = new targetWindow.ResizeObserver((entries) => {
    const entry = entries[0]
    if (entry?.contentRect.height) {
      notify({
        width: Math.round(entry.contentRect.width || element.clientWidth || 0),
        height: Math.max(1, Math.round(entry.contentRect.height)),
      })
      return
    }

    notify(readScrollerRect(element))
  })

  observer.observe(element)

  return () => {
    observer.unobserve(element)
  }
}

function measureViewport() {
  const target = scrollerRef.value
  if (!target) {
    return
  }

  updateMeasuredViewportHeight(target.getBoundingClientRect().height || target.clientHeight)
}

function clearPendingProgrammaticScrollReset() {
  if (pendingProgrammaticScrollResetHandle === null) {
    return
  }

  window.clearTimeout(pendingProgrammaticScrollResetHandle)
  pendingProgrammaticScrollResetHandle = null
}

function noteProgrammaticScrollEvent(nextScrollTop: number) {
  pendingProgrammaticScrollEvents += 1
  lastProgrammaticScrollTop = nextScrollTop
  clearPendingProgrammaticScrollReset()
  pendingProgrammaticScrollResetHandle = window.setTimeout(() => {
    pendingProgrammaticScrollEvents = 0
    lastProgrammaticScrollTop = null
    pendingProgrammaticScrollResetHandle = null
  }, 80)
}

function consumeProgrammaticScrollEvent(nextScrollTop: number) {
  if (pendingProgrammaticScrollEvents < 1) {
    return false
  }

  if (lastProgrammaticScrollTop !== null && Math.abs(nextScrollTop - lastProgrammaticScrollTop) > 1) {
    return false
  }

  pendingProgrammaticScrollEvents = Math.max(0, pendingProgrammaticScrollEvents - 1)
  if (pendingProgrammaticScrollEvents === 0) {
    lastProgrammaticScrollTop = null
    clearPendingProgrammaticScrollReset()
  }

  return true
}

function handleScroll(event: Event) {
  const target = event.target as HTMLElement | null
  scrollTop.value = target?.scrollTop ?? 0
  if (target) {
    const userInitiated = !consumeProgrammaticScrollEvent(target.scrollTop)
    syncViewportState(target, { userInitiated })
  }
}

function handleWheel(event: WheelEvent) {
  if (event.deltaY < 0) {
    pauseFollowBottomByUser()
    const scroller = scrollerRef.value
    if (scroller && scroller.scrollTop <= props.topThreshold && topReachArmed) {
      topReachArmed = false
      emit('reach-top')
    }
  }
}

function isNearBottom(scroller: HTMLElement) {
  const scrollHeight = Math.max(scroller.scrollHeight, totalHeight.value)
  const clientHeight = Math.max(scroller.clientHeight, effectiveViewportHeight.value)
  return scrollHeight - clientHeight - scroller.scrollTop <= props.bottomThreshold
}

function syncViewportState(scroller: HTMLElement, options: { userInitiated?: boolean } = {}) {
  const nextAtBottom = isNearBottom(scroller)
  if (options.userInitiated && props.followBottom && !nextAtBottom) {
    followBottomPausedByUser = true
  }
  const shouldSuppressBottomLoss = (
    !nextAtBottom
    && options.userInitiated === false
    && props.followBottom
    && !followBottomPausedByUser
  )
  if (nextAtBottom) {
    followBottomPausedByUser = false
  }

  if (!shouldSuppressBottomLoss && nextAtBottom !== lastAtBottom) {
    lastAtBottom = nextAtBottom
    emit('at-bottom-change', nextAtBottom)
  }

  if (scroller.scrollTop <= props.topThreshold) {
    if (topReachArmed) {
      topReachArmed = false
      emit('reach-top')
    }
    return
  }

  topReachArmed = true
}

function pauseFollowBottomByUser() {
  if (followBottomPausedByUser) {
    return
  }

  followBottomPausedByUser = true
  if (lastAtBottom) {
    lastAtBottom = false
    emit('at-bottom-change', false)
  }
}

function scrollToOffset(nextScrollTop: number) {
  const scroller = scrollerRef.value
  if (!scroller) {
    return
  }

  const maxScrollTop = Math.max(0, Math.max(scroller.scrollHeight, totalHeight.value) - scroller.clientHeight)
  const clamped = Math.max(0, Math.min(nextScrollTop, maxScrollTop))
  if (clamped !== scroller.scrollTop) {
    noteProgrammaticScrollEvent(clamped)
  }
  scroller.scrollTop = clamped
  scrollTop.value = clamped
  scroller.dispatchEvent(new Event('scroll'))
  syncViewportState(scroller, { userInitiated: false })
}

function scrollToBottom() {
  const scroller = scrollerRef.value
  if (!scroller) {
    return
  }

  followBottomPausedByUser = false
  scrollToOffset(Math.max(scroller.scrollHeight, totalHeight.value) - scroller.clientHeight)
}

function clampScrollPosition(scroller: HTMLElement) {
  if (scroller.clientHeight <= 0 || Math.max(scroller.scrollHeight, totalHeight.value) <= 0) {
    return false
  }

  const maxScrollTop = Math.max(0, Math.max(scroller.scrollHeight, totalHeight.value) - scroller.clientHeight)
  const nextScrollTop = Math.min(Math.max(scroller.scrollTop, 0), maxScrollTop)
  if (Math.abs(nextScrollTop - scroller.scrollTop) <= 1) {
    return false
  }

  scrollToOffset(nextScrollTop)
  return true
}

async function restorePrependedAnchor(previousScrollTop: number, previousScrollHeight: number) {
  pendingPrependedAnchorRestoreToken += 1
  const restoreToken = pendingPrependedAnchorRestoreToken

  await nextTick()
  if (restoreToken !== pendingPrependedAnchorRestoreToken) {
    return
  }

  const supportsRaf = typeof window !== 'undefined' && typeof window.requestAnimationFrame === 'function'

  applyAnchorDelta(previousScrollTop, previousScrollHeight)

  if (!supportsRaf) {
    return
  }

  for (let attempt = 0; attempt < 3; attempt += 1) {
    await new Promise<void>((resolve) => {
      window.requestAnimationFrame(() => resolve())
    })
    if (restoreToken !== pendingPrependedAnchorRestoreToken) {
      return
    }

    applyAnchorDelta(previousScrollTop, previousScrollHeight)
  }
}

function applyAnchorDelta(previousScrollTop: number, previousScrollHeight: number) {
  const scroller = scrollerRef.value
  if (!scroller) {
    return
  }

  const currentScrollHeight = Math.max(scroller.scrollHeight, totalHeight.value)
  const delta = Math.max(0, currentScrollHeight - previousScrollHeight)
  const nextScrollTop = previousScrollTop + delta
  if (Math.abs(nextScrollTop - scroller.scrollTop) <= 1) {
    return
  }

  scrollToOffset(nextScrollTop)
}

function measureVisibleDynamicRows() {
  if (!props.dynamicItemHeight) {
    return
  }

  void nextTick(() => {
    rowVirtualizer.value.measure()
  })
}

async function settleDynamicMeasurements() {
  if (measurementsSettled.value) {
    return
  }

  if (!props.dynamicItemHeight) {
    measurementsSettled.value = true
    return
  }

  const token = ++measurementsSettledToken
  await nextTick()
  if (token !== measurementsSettledToken) {
    return
  }

  if (typeof window === 'undefined' || typeof window.requestAnimationFrame !== 'function') {
    measurementsSettled.value = true
    return
  }

  await new Promise<void>((resolve) => {
    window.requestAnimationFrame(() => resolve())
  })
  if (token !== measurementsSettledToken) {
    return
  }

  await new Promise<void>((resolve) => {
    window.requestAnimationFrame(() => resolve())
  })
  if (token !== measurementsSettledToken) {
    return
  }

  measurementsSettled.value = true
}

async function syncScrollerLifecycle() {
  await nextTick()

  const scroller = scrollerRef.value
  if (!scroller) {
    return
  }

  measureViewport()

  if (props.followBottom) {
    scrollToBottom()
    return
  }

  if (clampScrollPosition(scroller)) {
    return
  }

  syncViewportState(scroller)
}

function measureRowElement(element: Element | null) {
  if (!props.dynamicItemHeight || !(element instanceof HTMLElement)) {
    return
  }

  const scroller = scrollerRef.value
  const shouldKeepBottom = Boolean(
    scroller
    && props.followBottom
    && !followBottomPausedByUser
    && (lastAtBottom || isNearBottom(scroller)),
  )
  rowVirtualizer.value.measureElement(element)
  if (shouldKeepBottom) {
    void nextTick(() => {
      const currentScroller = scrollerRef.value
      if (currentScroller && props.followBottom && !followBottomPausedByUser) {
        scrollToBottom()
      }
    })
  }
}

function rowStyle(virtualItem: VirtualItem) {
  const style: Record<string, string> = {
    transform: `translateY(${virtualItem.start}px)`,
  }

  if (!props.dynamicItemHeight) {
    style.height = `${props.itemHeight}px`
  }

  return style
}

let preItemUpdateScrollTop = 0
let preItemUpdateScrollHeight = 0

watch(
  () => props.items,
  () => {
    const scroller = scrollerRef.value
    if (!scroller) {
      preItemUpdateScrollTop = 0
      preItemUpdateScrollHeight = totalHeight.value
      return
    }

    preItemUpdateScrollTop = scroller.scrollTop
    preItemUpdateScrollHeight = scroller.scrollHeight
  },
  { flush: 'sync' },
)

watch(
  () => props.items,
  (nextItems, previousItems) => {
    const scroller = scrollerRef.value
    const wasAtBottom = scroller ? isNearBottom(scroller) : lastAtBottom
    const previousScrollTop = preItemUpdateScrollTop
    const previousScrollHeight = preItemUpdateScrollHeight
    const nextFirstKey = firstItemKey()
    const nextLastKey = lastItemKey()
    const previousFirstItemKey = previousItems.length > 0 ? resolveKey(previousItems[0]!, 0) : previousFirstKey
    const previousLastItemKey = previousItems.length > 0
      ? resolveKey(previousItems[previousItems.length - 1]!, previousItems.length - 1)
      : previousLastKey
    const prepended = previousFirstItemKey !== null && nextFirstKey !== previousFirstItemKey
      && nextItems.some((item, index) => resolveKey(item, index) === previousFirstItemKey)
    const appended = previousLastItemKey !== null && nextLastKey !== previousLastItemKey
      && nextItems.some((item, index) => resolveKey(item, index) === previousLastItemKey)

    if (prepended && scroller) {
      void restorePrependedAnchor(previousScrollTop, previousScrollHeight)
    } else if (
      scroller
      && !followBottomPausedByUser
      && (props.followBottom || wasAtBottom)
      && (appended || (previousItems.length === 0 && nextItems.length > 0) || nextItems.length !== previousItems.length)
    ) {
      void nextTick(() => {
        scrollToBottom()
      })
    } else if (scroller) {
      void nextTick(() => {
        clampScrollPosition(scroller)
        syncViewportState(scroller, { userInitiated: false })
      })
    }

    previousFirstKey = nextFirstKey
    previousLastKey = nextLastKey
    if (scroller && scroller.scrollTop > props.topThreshold) {
      topReachArmed = true
    }
  },
  { flush: 'post' },
)

watch(
  () => [props.itemHeight, props.dynamicItemHeight, props.overscan] as const,
  () => {
    measureVisibleDynamicRows()
  },
  { flush: 'post' },
)

watch(
  () => props.followBottom,
  (followBottom) => {
    if (!followBottom) {
      return
    }

    followBottomPausedByUser = false
    nextTick(() => {
      if (props.followBottom && !followBottomPausedByUser) {
        scrollToBottom()
      }
    })
  },
)

watch(
  scrollerRef,
  (scroller) => {
    if (!scroller) {
      return
    }

    void syncScrollerLifecycle()
  },
  { flush: 'post' },
)

onMounted(() => {
  previousFirstKey = firstItemKey()
  previousLastKey = lastItemKey()
  void syncScrollerLifecycle()
  void settleDynamicMeasurements()
})

onActivated(() => {
  void syncScrollerLifecycle()
  void settleDynamicMeasurements()
})

onBeforeUnmount(() => {
  pendingPrependedAnchorRestoreToken += 1
  measurementsSettledToken += 1
  clearPendingProgrammaticScrollReset()
})

defineExpose({
  getScrollMetrics() {
    const scroller = scrollerRef.value
    return {
      clientHeight: scroller?.clientHeight ?? 0,
      scrollHeight: scroller ? Math.max(scroller.scrollHeight, totalHeight.value) : totalHeight.value,
      scrollTop: scroller?.scrollTop ?? 0,
    }
  },
  isAtBottom() {
    const scroller = scrollerRef.value
    return scroller ? isNearBottom(scroller) : true
  },
  scrollToBottom,
  scrollToOffset,
})
</script>

<template>
  <section class="data-viewport">
    <div v-if="$slots.header" class="data-viewport__header">
      <slot name="header" />
    </div>

    <div v-if="items.length === 0" class="data-viewport__empty">
      {{ emptyLabel }}
    </div>

    <div
      v-else
      ref="scrollerRef"
      class="data-viewport__scroller"
      :class="{ 'is-measurements-settled': measurementsSettled }"
      :style="viewportStyle"
      @scroll="handleScroll"
      @wheel.passive="handleWheel"
    >
      <div class="data-viewport__canvas" :style="{ height: `${totalHeight}px` }">
        <div
          v-for="{ item, virtualItem } in visibleRows"
          :key="virtualItem.key"
          :data-index="virtualItem.index"
          class="data-viewport__row"
          :style="rowStyle(virtualItem)"
          :ref="measureRowElement"
        >
          <slot :item="item" :index="virtualItem.index" />
        </div>
      </div>
    </div>
  </section>
</template>
