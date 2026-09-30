<script setup lang="ts">
import { ArrowUpRightIcon } from '@lucide/vue'
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
      <ArrowUpRightIcon class="management-context-actions__arrow" aria-hidden="true" />
    </AppButton>
  </div>
</template>

<style scoped lang="scss">
.management-context-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}

.management-context-actions__arrow {
  width: 14px;
  height: 14px;
  color: var(--muted);
}
</style>
