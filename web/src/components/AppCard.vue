<script setup lang="ts">
defineProps<{
  borderless?: boolean
  description?: string
  loading?: boolean
  shadow?: 'sm' | 'md' | 'lg' | 'none'
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

/* Variant: stat — metric cards with top accent bar */
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

.app-card--stat:hover::before {
  background: color-mix(in srgb, var(--accent) 50%, transparent);
}

/* Variant: highlight — stronger border and subtle glow on hover */
.app-card--highlight {
  border-color: color-mix(in srgb, var(--accent) 12%, var(--border));
}

.app-card--highlight:hover {
  border-color: color-mix(in srgb, var(--accent) 30%, var(--border));
  box-shadow: 0 0 0 1px color-mix(in srgb, var(--accent) 10%, transparent), var(--shadow-lg);
}

/* Variant: flat — no lift, minimal hover */
.app-card--flat {
  box-shadow: none;
}

.app-card--flat:hover {
  transform: none;
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
