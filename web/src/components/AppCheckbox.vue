<script setup lang="ts">
import { useId } from 'vue'
import { Checkbox } from '@/components/ui/checkbox'
import { Label } from '@/components/ui/label'
defineProps<{ disabled?: boolean; id?: string }>()
const model = defineModel<boolean>({ default: false })
const generatedId = useId()
</script>
<template>
  <div class="app-checkbox">
    <Checkbox :id="id || generatedId" :model-value="model" :disabled="disabled" @update:model-value="model = $event === true" />
    <Label v-if="$slots.default" :for="id || generatedId" class="app-checkbox__label"><slot /></Label>
  </div>
</template>
<style scoped>
.app-checkbox { display: flex; align-items: center; gap: 10px; min-height: 36px; }
.app-checkbox__label { cursor: pointer; font-size: 14px; font-weight: 400; line-height: 1.5; }
.app-checkbox :deep([data-state=checked]) { color: var(--on-brand); }
@media (pointer: coarse) { .app-checkbox { min-height: 44px; } }
</style>
