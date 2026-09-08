<script setup lang="ts">
withDefaults(defineProps<{ size?: number }>(), { size: 40 })
</script>
<template><span class="app-avatar" :style="{ width: size + 'px', height: size + 'px' }"><slot><slot name="icon" /></slot></span></template>
<style scoped>
.app-avatar { display: inline-flex; align-items: center; justify-content: center; flex: none; overflow: hidden; border-radius: 50%; background: var(--surface-soft); color: var(--muted); font-weight: 600; }
.app-avatar :deep(img) { width: 100%; height: 100%; object-fit: cover; }
.app-avatar :deep(svg) { width: 20px; height: 20px; }
</style>
