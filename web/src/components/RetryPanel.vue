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
    <el-result icon="warning" :title="title" :sub-title="description">
      <template #extra>
        <el-button type="primary" :loading="loading" @click="$emit('retry')">
          {{ retryLabel ?? '重试' }}
        </el-button>
      </template>
    </el-result>
  </section>
</template>
