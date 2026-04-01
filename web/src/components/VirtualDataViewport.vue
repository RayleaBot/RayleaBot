<script setup lang="ts" generic="T">
import { computed, nextTick, onBeforeUnmount, onMounted, ref } from 'vue'

interface Props<TItem> {
  items: TItem[]
  itemHeight?: number
  viewportHeight?: number | string
  overscan?: number
  emptyLabel?: string
  getItemKey?: (item: TItem, index: number) => string | number
}

const props = withDefaults(defineProps<Props<T>>(), {
  itemHeight: 160,
  overscan: 3,
  emptyLabel: '暂无数据',
  getItemKey: undefined,
})

const scrollTop = ref(0)
const scrollerRef = ref<HTMLElement | null>(null)
const measuredViewportHeight = ref<number | null>(null)
let resizeObserver: ResizeObserver | null = null

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

const visibleCount = computed(() => {
  return Math.max(1, Math.ceil(effectiveViewportHeight.value / props.itemHeight) + props.overscan * 2)
})

const startIndex = computed(() => Math.max(0, Math.floor(scrollTop.value / props.itemHeight) - props.overscan))
const endIndex = computed(() => Math.min(props.items.length, startIndex.value + visibleCount.value))
const visibleItems = computed(() => props.items.slice(startIndex.value, endIndex.value))
const offsetY = computed(() => startIndex.value * props.itemHeight)
const totalHeight = computed(() => props.items.length * props.itemHeight)

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

onMounted(async () => {
  await nextTick()
  measureViewport()

  if (typeof window.ResizeObserver !== 'function' || !scrollerRef.value) {
    return
  }

  resizeObserver = new window.ResizeObserver((entries) => {
    const entry = entries[0]
    updateMeasuredViewportHeight(entry?.contentRect.height)
  })

  resizeObserver.observe(scrollerRef.value)
})

onBeforeUnmount(() => {
  resizeObserver?.disconnect()
  resizeObserver = null
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
      :style="viewportStyle"
      @scroll="handleScroll"
    >
      <div class="data-viewport__canvas" :style="{ height: `${totalHeight}px` }">
        <div class="data-viewport__stack" :style="{ transform: `translateY(${offsetY}px)` }">
          <div
            v-for="(item, localIndex) in visibleItems"
            :key="resolveKey(item, startIndex + localIndex)"
            class="data-viewport__row"
            :style="{ height: `${itemHeight}px` }"
          >
            <slot :item="item" :index="startIndex + localIndex" />
          </div>
        </div>
      </div>
    </div>
  </section>
</template>
