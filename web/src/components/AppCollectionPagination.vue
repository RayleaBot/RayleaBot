<script setup lang="ts">
import AppButton from '@/components/AppButton.vue'

defineProps<{ loaded: number; total: number; nextCursor?: string; loading?: boolean }>()
defineEmits<{ more: [] }>()
</script>

<template>
  <div class="collection-pagination">
    <span role="status">已加载 {{ loaded }} / {{ total }}</span>
    <AppButton v-if="nextCursor" :loading="loading" :disabled="loading" @click="$emit('more')">加载更多</AppButton>
  </div>
</template>

<style scoped>
.collection-pagination { display: flex; align-items: center; justify-content: space-between; gap: 1rem; padding-block: 0.75rem; color: var(--muted-foreground); font-size: 0.875rem; }
</style>
