<script setup lang="ts" generic="T">
import { computed, ref } from 'vue'

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
  viewportHeight: 560,
  overscan: 3,
  emptyLabel: '暂无数据',
  getItemKey: undefined,
})

const scrollTop = ref(0)

const viewportStyle = computed(() => ({
  height: typeof props.viewportHeight === 'number' ? `${props.viewportHeight}px` : props.viewportHeight,
}))

const visibleCount = computed(() => {
  const viewport = typeof props.viewportHeight === 'number'
    ? props.viewportHeight
    : Number.parseInt(props.viewportHeight, 10) || 560

  return Math.max(1, Math.ceil(viewport / props.itemHeight) + props.overscan * 2)
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

function resolveKey(item: T, index: number) {
  if (props.getItemKey) {
    return props.getItemKey(item, index)
  }

  return index
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

    <div v-else class="data-viewport__scroller" :style="viewportStyle" @scroll="handleScroll">
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
