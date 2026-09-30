<script setup lang="ts">
import AppPopover from '@/components/AppPopover.vue'
import AppButton from '@/components/AppButton.vue'
import { FilterIcon } from '@lucide/vue'
import { computed, onBeforeUnmount, onDeactivated, ref } from 'vue'

import { t } from '@/i18n'
import type { LogFilters } from '@/stores/log-state'
import ManagementLogAdvancedField from './ManagementLogAdvancedField.vue'
import { isLogAdvancedFilterActive, type LogAdvancedFilterKey } from './useLogFilterControls'

// fields: the filters that did not fit in the toolbar. The count covers only those, since the rest are in view.
const props = defineProps<{ fields: readonly LogAdvancedFilterKey[] }>()
const filters = defineModel<LogFilters>({ required: true })

const open = ref(false)
const activeFilterCount = computed(() => props.fields.filter(field => isLogAdvancedFilterActive(filters.value, field)).length)

function close() {
  open.value = false
}

onDeactivated(close)
onBeforeUnmount(close)
</script>

<template>
  <AppPopover v-model:open="open" align="end" :width="390" :label="t('logs.filters.more')">
    <template #content>
      <div
        class="log-advanced-filters__panel"
        :aria-label="t('logs.filters.more')"
        @keydown.esc="close"
      >
        <ManagementLogAdvancedField v-for="field in fields" :key="field" v-model="filters" :field="field" />
      </div>
    </template>
    <AppButton
      class="log-advanced-filters__trigger"
      :class="{ 'is-active': activeFilterCount > 0 }"
      :aria-expanded="open"
      aria-haspopup="dialog"
    >
      <template #icon>
        <FilterIcon />
      </template>
      {{ t('logs.filters.more') }}
      <span v-if="activeFilterCount" class="log-advanced-filters__count">{{ activeFilterCount }}</span>
    </AppButton>
  </AppPopover>
</template>

<style scoped lang="scss">
.log-advanced-filters__trigger {
  position: relative;
}

.log-advanced-filters__trigger.is-active {
  border-color: color-mix(in srgb, var(--accent) 52%, var(--border));
  background: var(--surface-accent);
  color: var(--text-accent);
}

// The count sits on the corner like the jump button's, so it never widens the button and reflows the toolbar.
.log-advanced-filters__count {
  position: absolute;
  top: -8px;
  right: -8px;
  min-width: 20px;
  padding: 0 6px;
  border-radius: 10px;
  background: var(--brand-fill);
  color: var(--on-brand);
  box-shadow: var(--shadow-xs);
  font-size: 12px;
  font-weight: 600;
  line-height: 20px;
  font-variant-numeric: tabular-nums;
}

.log-advanced-filters__trigger:focus-visible {
  outline: 2px solid var(--focus);
  outline-offset: var(--focus-outline-offset);
}

.log-advanced-filters__panel {
  display: grid;
  gap: 12px;
  width: 100%;
}

.log-advanced-filters__panel :deep(.app-field) {
  margin-bottom: 0;
}
</style>
