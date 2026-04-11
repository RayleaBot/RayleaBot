<script setup lang="ts">
defineProps<{
  title: string
  description: string
  loading?: boolean
  retryLabel?: string
}>()

defineEmits<{
  retry: []
}>()
</script>

<template>
  <section class="retry-panel" role="alert">
    <a-result status="warning" :title="title" :sub-title="description">
      <template #extra>
        <a-button type="primary" :loading="loading" @click="$emit('retry')">
          {{ retryLabel ?? '重试' }}
        </a-button>
      </template>
    </a-result>
  </section>
</template>
