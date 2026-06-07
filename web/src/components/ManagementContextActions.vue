<script setup lang="ts">
import type { RouteLocationRaw } from 'vue-router'

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
    <RouterLink
      v-for="action in actions"
      :key="action.key"
      :to="action.to"
      custom
      v-slot="{ navigate }"
    >
      <a-button :size="size ?? 'small'" @click="() => { emit('action'); navigate() }">
        {{ action.label }}
      </a-button>
    </RouterLink>
  </div>
</template>

<style scoped lang="scss">
.management-context-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
</style>
