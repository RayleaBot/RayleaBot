<script setup lang="ts">
import AppButton from '@/components/AppButton.vue'
import type { ManagementContextAction } from '@/lib/management-links'
import { useMotionNavigation } from '@/motion/useMotionNavigation'

const navigate = useMotionNavigation()

defineProps<{
  actions: ManagementContextAction[]
}>()

const emit = defineEmits<{
  action: []
}>()
</script>

<template>
  <div v-if="actions.length" class="management-context-actions">
    <AppButton
      v-for="action in actions"
      :key="action.key"
      size="sm"
      @click="() => { emit('action'); void navigate(action.to) }"
    >
      {{ action.label }}
    </AppButton>
  </div>
</template>

<style scoped lang="scss">
.management-context-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
</style>
