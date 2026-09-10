<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import RayleaMark from '@/components/brand/RayleaMark.vue'

const props = defineProps<{ pluginId: string; icon?: string; version?: string; remoteUrl?: string; sourceKey?: string; refreshKey?: number }>()
const failed = ref(false)
const resourceKey = computed(() => `${props.sourceKey ?? ''}:${props.pluginId}:${props.remoteUrl ?? props.icon ?? ''}:${props.version ?? ''}:${props.refreshKey ?? 0}`)
const source = computed(() => {
  if (failed.value) return undefined
  if (props.remoteUrl !== undefined) return props.remoteUrl.trim() || undefined
  return props.icon?.trim() ? `/api/plugins/${encodeURIComponent(props.pluginId)}/icon` : undefined
})

watch(resourceKey, () => { failed.value = false })
</script>

<template>
  <span class="plugin-icon" aria-hidden="true">
    <img v-if="source" :key="resourceKey" :src="source" alt="" loading="lazy" decoding="async" referrerpolicy="no-referrer" @error="failed = true" />
    <RayleaMark v-else variant="neutral" />
  </span>
</template>

<style scoped>
.plugin-icon { display: grid; width: 40px; height: 40px; flex: none; place-items: center; }
.plugin-icon img { display: block; width: 100%; height: 100%; object-fit: contain; }
.plugin-icon :deep(.raylea-mark) { width: 32px; height: 32px; }
</style>
