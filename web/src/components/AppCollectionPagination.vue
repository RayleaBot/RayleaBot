<script setup lang="ts">
import { t } from '@/i18n'
import AppButton from '@/components/AppButton.vue'

defineProps<{ loaded: number; total: number; nextCursor?: string; loading?: boolean }>()
defineEmits<{ more: [] }>()
</script>

<template>
  <!-- An empty or still-loading collection has nothing to count; its own empty or loading state speaks for it. -->
  <div v-if="loaded > 0 || total > 0 || nextCursor" class="collection-pagination">
    <span role="status">{{ t('ui.loadedCount', { loaded, total }) }}</span>
    <AppButton v-if="nextCursor" :loading="loading" :disabled="loading" @click="$emit('more')">{{ t('ui.loadMore') }}</AppButton>
  </div>
</template>

<style scoped>
.collection-pagination { display: flex; align-items: center; justify-content: space-between; gap: 1rem; padding-block: 0.75rem; color: var(--muted); font-size: 0.875rem; }
</style>
