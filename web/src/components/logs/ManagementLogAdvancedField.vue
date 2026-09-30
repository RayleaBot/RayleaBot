<script setup lang="ts">
import AppField from '@/components/AppField.vue'
import AppInput from '@/components/AppInput.vue'
import AppSelect from '@/components/AppSelect.vue'
import PluginPicker from '@/components/plugins/PluginPicker.vue'
import { t } from '@/i18n'
import type { LogFilters } from '@/stores/log-state'
import { useLogFilterControls, type LogAdvancedFilterKey } from './useLogFilterControls'

// One of the filters that sits in the toolbar while there is room and in 更多筛选 otherwise.
defineProps<{ field: LogAdvancedFilterKey }>()
const filters = defineModel<LogFilters>({ required: true })
const { protocolOptions, protocol, pluginIds, requestId } = useLogFilterControls(filters)
</script>

<template>
  <AppField v-if="field === 'protocol'" :label="t('logs.filters.protocol')">
    <AppSelect v-model="protocol" :options="protocolOptions" :placeholder="t('logs.filters.all')" />
  </AppField>
  <AppField v-else-if="field === 'plugin'" :label="t('logs.filters.plugin')">
    <PluginPicker v-model="pluginIds" multiple :placeholder="t('logs.filters.all')" />
  </AppField>
  <AppField v-else :label="t('logs.filters.requestId')">
    <AppInput v-model="requestId" :placeholder="t('logs.filters.requestPlaceholder')" />
  </AppField>
</template>
