<script setup lang="ts">
import AppButton from '@/components/AppButton.vue'
import type { RouteLocationRaw } from 'vue-router'

import { useMotionNavigation } from '@/motion/useMotionNavigation'

const navigate = useMotionNavigation()

defineProps<{
  actions: Array<{
    key: string
    label: string
    to: RouteLocationRaw
  }>
  size?: 'large' | 'middle' | 'small'
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
      :size="size === 'large' ? 'lg' : size === 'middle' ? 'default' : 'sm'"
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
