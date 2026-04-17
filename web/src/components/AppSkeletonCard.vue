<script setup lang="ts">
defineProps<{
  rows?: number
  showHeader?: boolean
}>()
</script>

<template>
  <div class="app-skeleton-card">
    <div v-if="showHeader" class="app-skeleton-card__header">
      <div class="app-skeleton-card__title-shimmer" />
    </div>
    <div class="app-skeleton-card__body">
      <div
        v-for="i in (rows ?? 4)"
        :key="i"
        class="app-skeleton-card__row"
        :style="{ width: i % 3 === 0 ? '72%' : i % 3 === 1 ? '92%' : '58%' }"
      />
    </div>
  </div>
</template>

<style scoped lang="scss">
.app-skeleton-card {
  border-radius: var(--app-card-radius);
  border: 1px solid var(--border);
  background: var(--surface-strong);
  overflow: hidden;
}

.app-skeleton-card__header {
  padding: 14px;
  border-bottom: 1px solid var(--border);
}

.app-skeleton-card__title-shimmer {
  height: 16px;
  width: 120px;
  border-radius: 4px;
  background: linear-gradient(90deg, var(--surface-soft), var(--surface), var(--surface-soft));
  background-size: 200% 100%;
  animation: skeleton-shimmer 1.4s linear infinite;
}

.app-skeleton-card__body {
  padding: 14px;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.app-skeleton-card__row {
  height: 12px;
  border-radius: 4px;
  background: linear-gradient(90deg, var(--surface-soft), var(--surface), var(--surface-soft));
  background-size: 200% 100%;
  animation: skeleton-shimmer 1.4s linear infinite;
}

@keyframes skeleton-shimmer {
  0% {
    background-position: 200% 0;
  }
  100% {
    background-position: -200% 0;
  }
}

@media (prefers-reduced-motion: reduce) {
  .app-skeleton-card__title-shimmer,
  .app-skeleton-card__row {
    animation: none;
    background: var(--surface-soft);
  }
}
</style>
