<script setup lang="ts">
import { FilterOutlined } from '@ant-design/icons-vue'
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
  <a-popover
    v-model:open="open"
    trigger="click"
    placement="bottomRight"
    :destroy-tooltip-on-hide="true"
  >
    <template #content>
      <a-form
        layout="vertical"
        class="log-advanced-filters__panel"
        :aria-label="t('logs.filters.more')"
        @keydown.esc="close"
      >
        <a-form-item :label="t('logs.filters.protocol')">
          <a-select
            v-model:value="protocol"
            allow-clear
            :options="[{ label: 'OneBot11', value: 'onebot11' }]"
            :placeholder="t('logs.filters.all')"
          />
        </a-form-item>
        <a-form-item :label="t('logs.filters.plugin')">
          <a-select
            v-model:value="pluginIds"
            mode="multiple"
            allow-clear
            :options="pluginOptions"
            :placeholder="t('logs.filters.all')"
            @focus="$emit('pluginFocus')"
          />
        </a-form-item>
        <a-form-item :label="t('logs.filters.requestId')">
          <a-input v-model:value="requestId" :placeholder="t('logs.filters.requestPlaceholder')" />
        </a-form-item>
      </a-form>
    </template>

    <a-badge :count="activeFilterCount" :show-zero="false" size="small">
      <a-button
        class="log-advanced-filters__trigger"
        :class="{ 'is-active': activeFilterCount > 0 }"
        :aria-expanded="open"
        aria-haspopup="dialog"
      >
        <template #icon>
          <FilterOutlined />
        </template>
        {{ t('logs.filters.more') }}
      </a-button>
    </a-badge>
  </a-popover>
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
  width: min(390px, calc(100vw - 40px));
}

.log-advanced-filters__panel :deep(.ant-form-item) {
  margin-bottom: 12px;
}

.log-advanced-filters__panel :deep(.ant-form-item:last-child) {
  margin-bottom: 0;
}
</style>
