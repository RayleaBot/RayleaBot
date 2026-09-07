<script setup lang="ts">
import { Switch } from '@/components/ui/switch'
const model = defineModel<boolean>({ default: false })
defineProps<{ disabled?: boolean }>()
</script>
<template>
  <Switch :model-value="model" :disabled="disabled" class="app-switch" @update:model-value="model = $event" />
</template>
<style>
.app-switch { width: 40px; height: 24px; padding: 2px; }
.app-switch [data-slot=switch-thumb] { width: 18px; height: 18px; background: var(--on-brand); }
.app-switch [data-state=checked] { translate: 16px 0; }
</style>
