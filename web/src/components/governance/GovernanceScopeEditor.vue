<script setup lang="ts">
import { computed } from 'vue'
import AppSelect from '@/components/AppSelect.vue'
import AppInput from '@/components/AppInput.vue'
import { t } from '@/i18n'
import type { GovernanceScope } from '@/types/governance'

const scope = defineModel<GovernanceScope>({ required: true })
const selectedMode = computed(() => scope.value.source_protocol === 'qqofficial' ? 'qqofficial' : scope.value.kind === 'instance' ? 'onebot-instance' : 'onebot-global')
function changeMode(value: string) {
  scope.value = value === 'onebot-global'
    ? { kind: 'global', source_protocol: 'onebot11', source_adapter: '', bot_id: '' }
    : { kind: 'instance', source_protocol: value === 'qqofficial' ? 'qqofficial' : 'onebot11', source_adapter: '', bot_id: '' }
}
const options = computed(() => [
  { value: 'onebot-global', label: t('accessLists.namespace.onebotGlobal') },
  { value: 'onebot-instance', label: t('accessLists.namespace.onebotInstance') },
  { value: 'qqofficial', label: t('accessLists.namespace.qqOfficial') },
])
</script>

<template>
  <div class="scope-editor">
    <AppSelect :model-value="selectedMode" :options="options" :aria-label="t('accessLists.namespace.label')" @update:model-value="changeMode" />
    <template v-if="selectedMode !== 'onebot-global'">
      <AppInput v-model="scope.source_adapter" :aria-label="t('accessLists.namespace.adapter')" :placeholder="t('accessLists.namespace.adapter')" />
      <AppInput v-model="scope.bot_id" :aria-label="t('accessLists.namespace.bot')" :placeholder="t('accessLists.namespace.bot')" />
    </template>
  </div>
</template>

<style scoped>
.scope-editor { display: grid; gap: 6px; }
</style>
