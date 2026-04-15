<script setup lang="ts" generic="T">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'

interface Props<TItem> {
  items: TItem[]
  itemHeight?: number
  viewportHeight?: number | string
  dynamicItemHeight?: boolean
  overscan?: number
  emptyLabel?: string
  getItemKey?: (item: TItem, index: number) => string | number
}

const props = withDefaults(defineProps<Props<T>>(), {
  itemHeight: 160,
  dynamicItemHeight: false,
  overscan: 3,
  emptyLabel: '暂无数据',
  getItemKey: undefined,
})

const scrollTop = ref(0)
const scrollerRef = ref<HTMLElement | null>(null)
const measuredViewportHeight = ref<number | null>(null)
const measurementVersion = ref(0)
let resizeObserver: ResizeObserver | null = null
const measuredHeights = new Map<string | number, number>()
const rowRefs = new Map<string | number, HTMLElement>()

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
  const previousHeight = measuredHeights.get(key) ?? props.itemHeight
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

onMounted(async () => {
  await nextTick()
  measureViewport()

  if (typeof window.ResizeObserver !== 'function' || !scrollerRef.value) {
    return
  }

  resizeObserver = new window.ResizeObserver((entries) => {
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

  resizeObserver.observe(scrollerRef.value)
  for (const [key, element] of rowRefs.entries()) {
    resizeObserver.observe(element)
    measureRowElement(key, element)
  }
})

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  resizeObserver = null
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
