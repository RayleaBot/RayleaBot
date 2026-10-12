<script setup lang="ts">
import { computed } from 'vue'
import AppField from '@/components/AppField.vue'
import AppSelect from '@/components/AppSelect.vue'
import AppInput from '@/components/AppInput.vue'
import { t } from '@/i18n'
import type { GovernanceScope } from '@/types/governance'

const scope = defineModel<GovernanceScope>({ required: true })
const selectedMode = computed(() => scope.value.source_protocol === 'qqofficial' ? 'qqofficial' : scope.value.kind === 'account' ? 'onebot-account' : 'onebot-global')
function changeMode(value: string) {
  scope.value = value === 'onebot-global'
    ? { kind: 'global', source_protocol: 'onebot11', bot_id: '' }
    : { kind: 'account', source_protocol: value === 'qqofficial' ? 'qqofficial' : 'onebot11', bot_id: '' }
}
const options = computed(() => [
  { value: 'onebot-global', label: t('accessLists.namespace.onebotGlobal') },
  { value: 'onebot-account', label: t('accessLists.namespace.onebotAccount') },
  { value: 'qqofficial', label: t('accessLists.namespace.qqOfficial') },
])
</script>

<template>
  <div class="scope-editor">
    <AppSelect :model-value="selectedMode" :options="options" :aria-label="t('accessLists.namespace.label')" @update:model-value="changeMode" />
    <!-- A floating label keeps the field named after it has a value; a placeholder alone disappears. -->
    <AppField v-if="selectedMode !== 'onebot-global'" floating :label="t('accessLists.namespace.bot')">
      <AppInput v-model="scope.bot_id" />
    </AppField>
  </div>
</template>

<style scoped>
.scope-editor { display: grid; gap: 6px; }
.scope-editor :deep(.app-field) { margin-bottom: 0; }
</style>
