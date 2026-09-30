<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import AppField from '@/components/AppField.vue'
import AppInput from '@/components/AppInput.vue'
import AppSelect from '@/components/AppSelect.vue'
import ManagementLogAdvancedField from './ManagementLogAdvancedField.vue'
import ManagementLogAdvancedFilters from './ManagementLogAdvancedFilters.vue'
import {
  fitInlineFilterCount,
  logAdvancedFilterKeys,
  logAdvancedFilterWidths,
  useLogFilterControls,
} from './useLogFilterControls'
import type { LogFilters } from '@/stores/log-state'
import { t } from '@/i18n'

// Edits apply on their own; the workspace debounces them. Level, source and the fields slot always show.
// Protocol, plugin and request ID follow on the same row while it has room; the ones that do not fit
// give way to 更多筛选 from the end, and with room for all of them there is no 更多筛选 at all.
defineProps<{ history?: boolean }>()
const filters = defineModel<LogFilters>({ required: true })
const { selectedLevels, levelOptions } = useLogFilterControls(filters)

// Room for the 更多筛选 button; its count sits on the corner, so the width does not change with it.
const overflowTriggerWidth = 120
const row = ref<HTMLElement | null>(null)
const extra = ref<HTMLElement | null>(null)
const inlineCount = ref<number>(logAdvancedFilterKeys.length)
const inlineFields = computed(() => logAdvancedFilterKeys.slice(0, inlineCount.value))
const overflowFields = computed(() => logAdvancedFilterKeys.slice(inlineCount.value))
let observer: ResizeObserver | null = null

function measure() {
  const element = row.value
  // A kept-alive page that is hidden measures zero; keep the layout it had.
  if (!element || element.clientWidth === 0) return
  const gap = parseFloat(getComputedStyle(element).columnGap) || 0
  const fixed = [...element.children].filter((child): child is HTMLElement => child instanceof HTMLElement
    && !child.hasAttribute('data-log-advanced') && !child.classList.contains('logs-toolbar__actions'))
  const extraWidth = extra.value?.offsetWidth ?? 0
  const fixedWidth = fixed.reduce((sum, child) => sum + child.offsetWidth, 0)
    + gap * Math.max(0, fixed.length - 1)
    + (extraWidth > 0 ? gap + extraWidth : 0)
  inlineCount.value = fitInlineFilterCount({
    rowWidth: element.clientWidth,
    fixedWidth,
    optionalWidths: logAdvancedFilterKeys.map(key => logAdvancedFilterWidths[key]),
    overflowWidth: overflowTriggerWidth,
    gap,
  })
}

onMounted(() => {
  if (typeof window.ResizeObserver === 'function' && row.value) {
    observer = new window.ResizeObserver(() => measure())
    observer.observe(row.value)
    if (extra.value) observer.observe(extra.value)
  }
  void nextTick(measure)
})

// The extra actions (the history quick ranges) only mount once their slot renders.
watch(extra, (element, previous) => {
  if (previous) observer?.unobserve(previous)
  if (element) observer?.observe(element)
}, { flush: 'post' })

onBeforeUnmount(() => {
  observer?.disconnect()
  observer = null
})
</script>

<template>
  <div ref="row" class="logs-filter-grid" :class="{ 'logs-filter-grid--history': history }">
    <AppField class="logs-filter-grid__level" :label="t('logs.filters.level')">
      <AppSelect v-model="selectedLevels" multiple clearable single-line :all-label="t('logs.filters.all')" :options="levelOptions" :placeholder="t('logs.filters.all')" />
    </AppField>
    <AppField class="logs-filter-grid__source" :label="t('logs.filters.source')">
      <AppInput v-model="filters.source" :placeholder="t('logs.filters.sourcePlaceholder')" />
    </AppField>
    <slot name="fields" />
    <ManagementLogAdvancedField
      v-for="field in inlineFields"
      :key="field"
      v-model="filters"
      :field="field"
      data-log-advanced
      :style="{ width: `${logAdvancedFilterWidths[field]}px` }"
    />
    <div v-if="$slots.actions || overflowFields.length" class="logs-toolbar__actions">
      <div v-if="$slots.actions" ref="extra" class="logs-toolbar__extra"><slot name="actions" /></div>
      <ManagementLogAdvancedFilters v-if="overflowFields.length" v-model="filters" :fields="overflowFields" />
    </div>
  </div>
</template>

<style scoped lang="scss">
// Fields keep fixed widths and share top labels, so the fit above holds and hints under a field never
// push the other controls out of line.
.logs-filter-grid { display: flex; flex-wrap: wrap; gap: 12px; align-items: flex-start; width: 100%; }
.logs-filter-grid :deep(.app-field) { flex: none; margin-bottom: 0; }
.logs-filter-grid__level { width: 220px; }
.logs-filter-grid__source { width: 240px; }
// The actions line up with the controls, below the height of a field label and its gap.
.logs-toolbar__actions { display: flex; flex: none; gap: 12px; align-items: center; margin-top: calc(14px * 1.5 + 8px); margin-inline-start: auto; }
.logs-toolbar__extra { display: flex; }
</style>
