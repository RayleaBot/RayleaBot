<script setup lang="ts">
import AppTag from '@/components/AppTag.vue'
import AppPopover from '@/components/AppPopover.vue'
import AppSelect from '@/components/AppSelect.vue'
import AppInput from '@/components/AppInput.vue'
import AppField from '@/components/AppField.vue'
import AppButton from '@/components/AppButton.vue'
import { FilterIcon } from '@lucide/vue'
import { computed, onBeforeUnmount, onDeactivated, ref } from 'vue'

import { t } from '@/i18n'

const protocol = defineModel<string | undefined>('protocol')
const pluginIds = defineModel<string[]>('pluginIds', { default: () => [] })
const requestId = defineModel<string>('requestId', { default: '' })

defineProps<{
  pluginOptions: Array<{ label: string; value: string }>
}>()

defineEmits<{
  pluginFocus: []
}>()

const protocolOptions: { value: string; label: string }[] = [{ value: '', label: t('logs.filters.all') }, { value: 'onebot11', label: 'OneBot11' }]
const protocolSelection = computed({ get: () => protocol.value ?? '', set: (value: string) => { protocol.value = value || undefined } })
const open = ref(false)
const activeFilterCount = computed(() => (
  Number(Boolean(protocol.value))
  + Number(pluginIds.value.length > 0)
  + Number(Boolean(requestId.value.trim()))
))

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
        <AppField :label="t('logs.filters.protocol')">
          <AppSelect
            v-model="protocolSelection"
            :options="protocolOptions"
            :placeholder="t('logs.filters.all')"
          />
        </AppField>
        <AppField :label="t('logs.filters.plugin')">
          <AppSelect
            v-model="pluginIds"
            multiple
            clearable
            :options="pluginOptions"
            :placeholder="t('logs.filters.all')"
            @focus="$emit('pluginFocus')"
            @open="$event && $emit('pluginFocus')"
          />
        </AppField>
        <AppField :label="t('logs.filters.requestId')">
          <AppInput v-model="requestId" :placeholder="t('logs.filters.requestPlaceholder')" />
        </AppField>
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
        <AppTag v-if="activeFilterCount" tone="info">{{ activeFilterCount }}</AppTag>
      </AppButton>

  </AppPopover>
</template>

<style scoped lang="scss">
.log-advanced-filters__trigger.is-active {
  border-color: color-mix(in srgb, var(--accent) 52%, var(--border));
  background: var(--surface-accent);
  color: var(--text-accent);
}

.log-advanced-filters__trigger:focus-visible {
  outline: 2px solid var(--accent);
  outline-offset: 2px;
}

.log-advanced-filters__panel {
  display: grid;
  gap: 12px;
  width: 100%;
}

</style>
