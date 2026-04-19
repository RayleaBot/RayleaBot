<script setup lang="ts" generic="T">
import { computed, nextTick, onActivated, onBeforeUnmount, onBeforeUpdate, onMounted, onUpdated, ref, watch } from 'vue'

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

const scrollTop = ref(0)
const scrollerRef = ref<HTMLElement | null>(null)
const measuredViewportHeight = ref<number | null>(null)
const measurementVersion = ref(0)
let resizeObserver: ResizeObserver | null = null
let observedScroller: HTMLElement | null = null
const measuredHeights = new Map<string | number, number>()
const measuredKeys = new Set<string | number>()
const rowRefs = new Map<string | number, HTMLElement>()
let lastAtBottom = true
let topReachArmed = false
let followBottomPausedByUser = false
let pendingProgrammaticScrollEvents = 0
let pendingProgrammaticScrollResetHandle: number | null = null
let lastProgrammaticScrollTop: number | null = null
let pendingListMutation: {
  previousKeys: Array<string | number>
  scrollHeight: number
  scrollTop: number
  atBottom: boolean
  anchorKey: string | number | null
  anchorOffset: number
  anchorViewportOffset: number | null
} | null = null

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

const layoutMetrics = computed(() => {
  measurementVersion.value

  const heights = new Array<number>(props.items.length)
  const offsets = new Array<number>(props.items.length)
  let total = 0

  for (let index = 0; index < props.items.length; index += 1) {
    const item = props.items[index]
    const key = resolveKey(item, index)
    const measuredHeight = props.dynamicItemHeight ? measuredHeights.get(key) : undefined
    const nextHeight = measuredHeight && measuredHeight > 0 ? measuredHeight : props.itemHeight
    offsets[index] = total
    heights[index] = nextHeight
    total += nextHeight
  }

  return {
    heights,
    offsets,
    totalHeight: total,
  }
})

const visibleStartIndex = computed(() => {
  if (!props.dynamicItemHeight) {
    return Math.min(props.items.length, Math.floor(scrollTop.value / props.itemHeight))
  }

  const { heights, offsets } = layoutMetrics.value
  for (let index = 0; index < heights.length; index += 1) {
    if (offsets[index]! + heights[index]! > scrollTop.value) {
      return index
    }
  }
  return props.items.length
})

const visibleCount = computed(() => {
  if (!props.dynamicItemHeight) {
    return Math.max(1, Math.ceil(effectiveViewportHeight.value / props.itemHeight) + props.overscan * 2)
  }

  const { heights, offsets } = layoutMetrics.value
  const viewportBottom = scrollTop.value + effectiveViewportHeight.value
  let visibleRows = 0
  for (let index = visibleStartIndex.value; index < props.items.length; index += 1) {
    visibleRows += 1
    if (offsets[index]! + heights[index]! >= viewportBottom) {
      break
    }
  }
  return Math.max(1, visibleRows + props.overscan * 2)
})

const startIndex = computed(() => Math.max(0, visibleStartIndex.value - props.overscan))
const endIndex = computed(() => Math.min(props.items.length, startIndex.value + visibleCount.value))
const visibleItems = computed(() => props.items.slice(startIndex.value, endIndex.value))
const offsetY = computed(() => {
  if (!props.dynamicItemHeight) {
    return startIndex.value * props.itemHeight
  }
  return layoutMetrics.value.offsets[startIndex.value] ?? 0
})
const totalHeight = computed(() => (
  props.dynamicItemHeight
    ? layoutMetrics.value.totalHeight
    : props.items.length * props.itemHeight
))

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
  }
}

function updateMeasuredViewportHeight(nextHeight?: number) {
  if (!nextHeight || Number.isNaN(nextHeight)) {
    return
  }

  measuredViewportHeight.value = Math.max(1, Math.round(nextHeight))
}

function measureViewport() {
  const target = scrollerRef.value
  if (!target) {
    return
  }

  updateMeasuredViewportHeight(target.getBoundingClientRect().height || target.clientHeight)
}

function createResizeObserver() {
  if (typeof window.ResizeObserver !== 'function') {
    return null
  }

  return new window.ResizeObserver((entries) => {
    for (const entry of entries) {
      if (entry.target === scrollerRef.value) {
        updateMeasuredViewportHeight(entry.contentRect.height)
        continue
      }

      if (!props.dynamicItemHeight || !(entry.target instanceof HTMLElement)) {
        continue
      }

      for (const [key, element] of rowRefs.entries()) {
        if (element === entry.target) {
          updateMeasuredRowHeight(key, entry.contentRect.height)
          break
        }
      }
    }
  })
}

function ensureResizeObserver() {
  const scroller = scrollerRef.value
  if (!scroller) {
    return
  }

  if (resizeObserver && observedScroller === scroller) {
    return
  }

  resizeObserver?.disconnect()
  resizeObserver = createResizeObserver()
  observedScroller = scroller

  if (!resizeObserver) {
    return
  }

  resizeObserver.observe(scroller)
  for (const [key, element] of rowRefs.entries()) {
    resizeObserver.observe(element)
    measureRowElement(key, element)
  }
}

function clearResizeObserver() {
  resizeObserver?.disconnect()
  resizeObserver = null
  observedScroller = null
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

async function syncScrollerLifecycle() {
  await nextTick()

  const scroller = scrollerRef.value
  if (!scroller) {
    return
  }

  measureViewport()
  ensureResizeObserver()

  if (props.followBottom) {
    scrollToBottom()
    return
  }

  if (clampScrollPosition(scroller)) {
    return
  }

  syncViewportState(scroller)
}

function resolveKey(item: T, index: number) {
  if (props.getItemKey) {
    return props.getItemKey(item, index)
  }

  return index
}

function resolveVisibleKey(item: T, localIndex: number) {
  return resolveKey(item, startIndex.value + localIndex)
}

function updateMeasuredRowHeight(key: string | number, nextHeight: number) {
  if (!props.dynamicItemHeight || !Number.isFinite(nextHeight) || nextHeight <= 0) {
    return
  }

  const roundedHeight = Math.max(1, Math.ceil(nextHeight))
  const hadMeasuredHeight = measuredKeys.has(key)
  const previousHeight = measuredHeights.get(key) ?? props.itemHeight
  measuredKeys.add(key)
  if (previousHeight === roundedHeight) {
    return
  }

  const itemIndex = findItemIndexByKey(key)
  const metrics = layoutMetrics.value
  const rowOffset = itemIndex >= 0
    ? (metrics.offsets[itemIndex] ?? itemIndex * props.itemHeight)
    : 0

  measuredHeights.set(key, roundedHeight)
  measurementVersion.value += 1

  const heightDelta = roundedHeight - previousHeight
  if (heightDelta !== 0 && rowOffset + previousHeight <= scrollTop.value) {
    syncScrollAnchor(heightDelta)
  }
}

function findItemIndexByKey(key: string | number) {
  for (let index = 0; index < props.items.length; index += 1) {
    if (resolveKey(props.items[index]!, index) === key) {
      return index
    }
  }

  return -1
}

function syncScrollAnchor(offsetDelta: number) {
  const scroller = scrollerRef.value
  if (!scroller) {
    return
  }

  const nextScrollTop = Math.max(0, scroller.scrollTop + offsetDelta)
  if (nextScrollTop === scroller.scrollTop) {
    return
  }

  scroller.scrollTop = nextScrollTop
  scrollTop.value = nextScrollTop
}

function scrollToOffset(nextScrollTop: number) {
  const scroller = scrollerRef.value
  if (!scroller) {
    return
  }

  const clamped = Math.max(0, Math.min(nextScrollTop, scroller.scrollHeight - scroller.clientHeight))
  if (clamped !== scroller.scrollTop) {
    noteProgrammaticScrollEvent(clamped)
  }
  scroller.scrollTop = clamped
  scrollTop.value = clamped
  syncViewportState(scroller, { userInitiated: false })
}

function scrollToBottom() {
  const scroller = scrollerRef.value
  if (!scroller) {
    return
  }

  followBottomPausedByUser = false
  scrollToOffset(scroller.scrollHeight - scroller.clientHeight)
}

function clampScrollPosition(scroller: HTMLElement) {
  if (scroller.clientHeight <= 0 || scroller.scrollHeight <= 0) {
    return false
  }

  const maxScrollTop = Math.max(0, scroller.scrollHeight - scroller.clientHeight)
  const nextScrollTop = Math.min(Math.max(scroller.scrollTop, 0), maxScrollTop)
  if (Math.abs(nextScrollTop - scroller.scrollTop) <= 1) {
    return false
  }

  scrollToOffset(nextScrollTop)
  return true
}

function isNearBottom(scroller: HTMLElement) {
  const scrollHeight = Math.max(scroller.scrollHeight, totalHeight.value)
  const clientHeight = Math.max(scroller.clientHeight, effectiveViewportHeight.value)
  return scrollHeight - clientHeight - scroller.scrollTop <= props.bottomThreshold
}

function syncViewportState(scroller: HTMLElement, options: { userInitiated?: boolean } = {}) {
  const nextAtBottom = isNearBottom(scroller)
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

function measureRowElement(key: string | number, element: HTMLElement) {
  const rectHeight = element.getBoundingClientRect().height
  const nextHeight = rectHeight > 0 ? rectHeight : element.scrollHeight
  updateMeasuredRowHeight(key, nextHeight)
}

function setMeasuredRowRef(key: string | number, element: Element | null) {
  if (!props.dynamicItemHeight) {
    return
  }
  const previous = rowRefs.get(key)
  if (previous && previous !== element && resizeObserver) {
    resizeObserver.unobserve(previous)
  }

  if (!(element instanceof HTMLElement)) {
    if (previous && resizeObserver) {
      resizeObserver.unobserve(previous)
    }
    rowRefs.delete(key)
    return
  }

  rowRefs.set(key, element)
  if (resizeObserver) {
    resizeObserver.observe(element)
  }
  measureRowElement(key, element)
}

function rowStyle(index: number) {
  if (!props.dynamicItemHeight) {
    return { height: `${props.itemHeight}px` }
  }

  return undefined
}

onMounted(() => {
  void syncScrollerLifecycle()
})

onActivated(() => {
  void syncScrollerLifecycle()
})

onBeforeUnmount(() => {
  clearPendingProgrammaticScrollReset()
  clearResizeObserver()
})

onBeforeUpdate(() => {
  const scroller = scrollerRef.value
  if (!scroller) {
    pendingListMutation = null
    return
  }

  const anchorIndex = Math.min(props.items.length - 1, Math.max(0, visibleStartIndex.value))
  const hasAnchor = props.items.length > 0 && anchorIndex >= 0
  const anchorKey = hasAnchor ? resolveKey(props.items[anchorIndex]!, anchorIndex) : null
  const anchorTop = hasAnchor
    ? (props.dynamicItemHeight
      ? (layoutMetrics.value.offsets[anchorIndex] ?? anchorIndex * props.itemHeight)
      : anchorIndex * props.itemHeight)
    : 0
  const anchorElement = anchorKey !== null ? rowRefs.get(anchorKey) ?? null : null
  const anchorViewportOffset = anchorElement
    ? anchorElement.getBoundingClientRect().top - scroller.getBoundingClientRect().top
    : null

  pendingListMutation = {
    previousKeys: props.items.map((item, index) => resolveKey(item, index)),
    scrollHeight: scroller.scrollHeight,
    scrollTop: scroller.scrollTop,
    atBottom: isNearBottom(scroller),
    anchorKey,
    anchorOffset: hasAnchor ? Math.max(0, scroller.scrollTop - anchorTop) : 0,
    anchorViewportOffset,
  }
})

onUpdated(() => {
  const scroller = scrollerRef.value
  const snapshot = pendingListMutation
  pendingListMutation = null
  if (!scroller || !snapshot) {
    return
  }

  const nextKeys = props.items.map((item, index) => resolveKey(item, index))
  const prepended = didPrepend(snapshot.previousKeys, nextKeys)
  const appended = didAppend(snapshot.previousKeys, nextKeys)
  const keysChanged = !areKeyArraysEqual(snapshot.previousKeys, nextKeys)
  const contentHeightChanged = scroller.scrollHeight !== snapshot.scrollHeight
  const shouldFollowBottomForListChanges = !followBottomPausedByUser && (props.followBottom || snapshot.atBottom)
  const shouldFollowBottomForHeightChanges = !followBottomPausedByUser && (props.followBottom || snapshot.atBottom)

  if (prepended) {
    const nextScrollHeight = scroller.scrollHeight
    const offsetDelta = nextScrollHeight - snapshot.scrollHeight
    if (offsetDelta !== 0 && snapshot.scrollTop > props.topThreshold) {
      scrollToOffset(snapshot.scrollTop + offsetDelta)
    }
    return
  }

  if (shouldFollowBottomForListChanges && (appended || (snapshot.previousKeys.length === 0 && nextKeys.length > 0))) {
    scrollToBottom()
    return
  }

  if (shouldFollowBottomForListChanges && nextKeys.length > 0 && keysChanged) {
    scrollToBottom()
    return
  }

  if (shouldFollowBottomForHeightChanges && nextKeys.length > 0 && contentHeightChanged) {
    scrollToBottom()
    return
  }

  if (!shouldFollowBottomForHeightChanges && contentHeightChanged && snapshot.anchorKey !== null) {
    const anchorElement = rowRefs.get(snapshot.anchorKey)
    if (anchorElement && snapshot.anchorViewportOffset !== null) {
      const nextViewportOffset = anchorElement.getBoundingClientRect().top - scroller.getBoundingClientRect().top
      const viewportDelta = nextViewportOffset - snapshot.anchorViewportOffset
      if (viewportDelta !== 0) {
        scrollToOffset(scroller.scrollTop + viewportDelta)
        return
      }
    }

    const anchorIndex = nextKeys.indexOf(snapshot.anchorKey)
    if (anchorIndex >= 0) {
      const anchorTop = props.dynamicItemHeight
        ? (layoutMetrics.value.offsets[anchorIndex] ?? anchorIndex * props.itemHeight)
        : anchorIndex * props.itemHeight
      scrollToOffset(anchorTop + snapshot.anchorOffset)
      return
    }
  }

  if (clampScrollPosition(scroller)) {
    return
  }

  syncViewportState(scroller, { userInitiated: false })
})

watch(
  () => props.items.map((item, index) => resolveKey(item, index)),
  (nextKeys) => {
    const activeKeys = new Set(nextKeys)
    let changed = false

    for (const key of Array.from(measuredHeights.keys())) {
      if (!activeKeys.has(key)) {
        measuredHeights.delete(key)
        changed = true
      }
    }

    for (const key of Array.from(measuredKeys.keys())) {
      if (!activeKeys.has(key)) {
        measuredKeys.delete(key)
      }
    }

    for (const [key, element] of Array.from(rowRefs.entries())) {
      if (!activeKeys.has(key)) {
        resizeObserver?.unobserve(element)
        rowRefs.delete(key)
      }
    }

    if (changed) {
      measurementVersion.value += 1
    }
  },
)

watch(
  scrollerRef,
  (scroller) => {
    if (!scroller) {
      clearResizeObserver()
      return
    }

    void syncScrollerLifecycle()
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
      scrollToBottom()
    })
  },
)

defineExpose({
  getScrollMetrics() {
    const scroller = scrollerRef.value
    return {
      clientHeight: scroller?.clientHeight ?? 0,
      scrollHeight: scroller?.scrollHeight ?? 0,
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

function didPrepend(previousKeys: Array<string | number>, nextKeys: Array<string | number>) {
  if (previousKeys.length === 0 || nextKeys.length === 0) {
    return false
  }

  const previousFirstKey = previousKeys[0]
  if (previousFirstKey === undefined) {
    return false
  }

  const nextIndex = nextKeys.indexOf(previousFirstKey)
  return nextIndex > 0
}

function didAppend(previousKeys: Array<string | number>, nextKeys: Array<string | number>) {
  if (previousKeys.length === 0 || nextKeys.length === 0) {
    return false
  }

  const previousLastKey = previousKeys[previousKeys.length - 1]
  if (previousLastKey === undefined) {
    return false
  }

  const nextIndex = nextKeys.indexOf(previousLastKey)
  return nextIndex >= 0 && nextIndex < nextKeys.length - 1
}

function areKeyArraysEqual(left: Array<string | number>, right: Array<string | number>) {
  if (left.length !== right.length) {
    return false
  }

  for (let index = 0; index < left.length; index += 1) {
    if (left[index] !== right[index]) {
      return false
    }
  }

  return true
}
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
      :style="viewportStyle"
      @scroll="handleScroll"
      @wheel.passive="handleWheel"
    >
      <div class="data-viewport__canvas" :style="{ height: `${totalHeight}px` }">
        <div class="data-viewport__stack" :style="{ transform: `translateY(${offsetY}px)` }">
          <div
            v-for="(item, localIndex) in visibleItems"
            :key="resolveVisibleKey(item, localIndex)"
            class="data-viewport__row"
            :style="rowStyle(startIndex + localIndex)"
            :ref="dynamicItemHeight ? (element) => setMeasuredRowRef(resolveVisibleKey(item, localIndex), element) : undefined"
          >
            <slot :item="item" :index="startIndex + localIndex" />
          </div>
        </div>
      </div>
    </div>
  </section>
</template>
