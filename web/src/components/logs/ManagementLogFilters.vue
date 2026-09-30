<script setup lang="ts">
import AppButton from '@/components/AppButton.vue'
import AppField from '@/components/AppField.vue'
import AppInput from '@/components/AppInput.vue'
import AppSelect from '@/components/AppSelect.vue'
import ManagementLogAdvancedFilters from './ManagementLogAdvancedFilters.vue'
import { useLogFilterControls } from './useLogFilterControls'
import type { LogFilters } from '@/stores/log-state'
import { t } from '@/i18n'

// pending: the fields differ from the filters of the list on screen until they are applied.
defineProps<{ history?: boolean; pending?: boolean }>()
defineEmits<{ apply: [] }>()
const filters = defineModel<LogFilters>({ required: true })
const { selectedLevels, levelOptions } = useLogFilterControls(filters)
</script>

<template>
  <div class="logs-filter-grid" :class="{ 'logs-filter-grid--history': history }">
    <AppField :label="t('logs.filters.level')">
      <AppSelect v-model="selectedLevels" multiple clearable single-line :all-label="t('logs.filters.all')" :options="levelOptions" :placeholder="t('logs.filters.all')" />
    </AppField>
    <AppField :label="t('logs.filters.source')">
      <AppInput v-model="filters.source" :placeholder="t('logs.filters.sourcePlaceholder')" />
    </AppField>
    <slot name="fields" />
    <div class="logs-toolbar__actions">
      <slot name="actions" />
      <ManagementLogAdvancedFilters
        v-model:protocol="filters.protocol"
        v-model:plugin-ids="filters.pluginIds"
        v-model:request-id="filters.requestId"
      />
      <span class="logs-toolbar__pending" :class="{ 'is-idle': !pending }" aria-live="polite">{{ pending ? t('logs.filters.pending') : '' }}</span>
      <AppButton class="logs-toolbar__apply" variant="default" :aria-label="t('logs.filters.apply')" @click="$emit('apply')">{{ t('logs.filters.apply') }}</AppButton>
    </div>
  </div>
</template>

<style scoped lang="scss">
// Fields share top labels and align at the top, so hints under the time range never push controls out of line.
.logs-filter-grid { display: flex; flex-wrap: wrap; gap: 12px; align-items: flex-start; width: 100%; }
.logs-filter-grid :deep(.app-field) { flex: 1 1 200px; max-width: 320px; margin-bottom: 0; }
.logs-filter-grid--history :deep(.app-field) { flex-basis: 190px; max-width: 300px; }
.logs-filter-grid :deep(.app-field:first-child) { max-width: 220px; }
.logs-toolbar__actions { display: flex; flex: 0 0 auto; gap: 8px; justify-content: flex-end; align-items: center; align-self: flex-end; margin-inline-start: auto; }
.logs-filter-grid--history .logs-toolbar__actions { flex-wrap: wrap; }
.logs-toolbar__pending { color: var(--text-attention); font-size: 13px; font-weight: 500; white-space: nowrap; }
// The live region stays rendered so the change is announced; while idle it gives its gap back.
.logs-toolbar__pending.is-idle { margin-inline-end: -8px; }
</style>
