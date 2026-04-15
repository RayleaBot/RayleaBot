<script setup lang="ts">
defineProps<{
  borderless?: boolean
  description?: string
  loading?: boolean
  shadow?: 'sm' | 'md' | 'lg' | 'none'
  title?: string
}>()
</script>

<template>
  <a-card
    :bordered="!borderless"
    :class="['app-card', shadow ? `app-card--shadow-${shadow}` : '']"
  >
    <template v-if="title || $slots.extra" #title>
      <div class="app-card__header">
        <div class="app-card__title">
          <span v-if="title" class="app-card__title-text">{{ title }}</span>
          <span v-if="description" class="app-card__desc">{{ description }}</span>
        </div>
        <div v-if="$slots.extra" class="app-card__extra">
          <slot name="extra" />
        </div>
      </div>
    </template>

    <a-skeleton v-if="loading" active :paragraph="{ rows: 4 }" />
    <slot v-else />
  </a-card>
</template>

<style scoped lang="scss">
.app-card {
  transition: transform 0.2s ease, box-shadow 0.2s ease, border-color 0.2s ease;
}

.app-card:hover {
  transform: translateY(-1px);
  box-shadow: var(--shadow-lg);
  border-color: var(--border-strong);
}

.app-card--shadow-sm:hover {
  box-shadow: var(--shadow-sm);
}

.app-card--shadow-md:hover {
  box-shadow: var(--shadow);
}

.app-card--shadow-lg:hover {
  box-shadow: var(--shadow-lg);
}

.app-card--shadow-none:hover {
  box-shadow: none;
}

.app-card__header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}

.app-card__title {
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.app-card__title-text {
  font-size: 1rem;
  font-weight: 600;
  line-height: 1.3;
  color: var(--text);
}

.app-card__desc {
  font-size: 0.8rem;
  color: var(--muted);
  line-height: 1.4;
}

.app-card__extra {
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}
</style>
