<script setup lang="ts">
defineProps<{
  borderless?: boolean
  description?: string
  loading?: boolean
  shadow?: 'sm' | 'md' | 'lg' | 'none'
  size?: 'default' | 'small'
  title?: string
  variant?: 'default' | 'stat' | 'highlight' | 'flat'
}>()
</script>

<template>
  <a-card
    :bordered="!borderless"
    :class="[
      'app-card',
      shadow ? `app-card--shadow-${shadow}` : '',
      variant ? `app-card--${variant}` : '',
    ]"
    :size="size"
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
  transition: border-color 150ms ease, background-color 150ms ease;
  box-shadow: var(--shadow-xs);
}

.app-card:hover {
  border-color: var(--border-strong);
}

.app-card--stat {
  position: relative;
  overflow: hidden;
}

.app-card--stat::before {
  content: '';
  position: absolute;
  inset: 0 0 auto;
  height: 2px;
  background: color-mix(in srgb, var(--muted) 26%, transparent);
}

.app-card--highlight {
  border-color: color-mix(in srgb, var(--accent) 12%, var(--border));
}

.app-card--highlight:hover {
  border-color: color-mix(in srgb, var(--accent) 30%, var(--border));
}

.app-card--flat {
  box-shadow: none;
}

.app-card--flat:hover {
  box-shadow: none;
  border-color: var(--border-strong);
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
