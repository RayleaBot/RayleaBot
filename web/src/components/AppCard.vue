<script setup lang="ts">
import { Card } from '@/components/ui/card'
import AppSkeleton from './AppSkeleton.vue'
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
  <Card
    :data-borderless="borderless || undefined"
    :class="[
      'app-card',
      shadow ? `app-card--shadow-${shadow}` : '',
      variant ? `app-card--${variant}` : '',
    ]"
    :size="size === 'small' ? 'sm' : 'default'"
  >
    <header v-if="title || $slots.title || $slots.extra" class="app-card__head">
      <div class="app-card__header">
        <div class="app-card__title">
          <slot name="title"><span v-if="title" class="app-card__title-text">{{ title }}</span></slot>
          <span v-if="description" class="app-card__desc">{{ description }}</span>
        </div>
        <div v-if="$slots.extra" class="app-card__extra">
          <slot name="extra" />
        </div>
      </div>
    </header>
    <div class="app-card__body">
      <AppSkeleton v-if="loading" :rows="4" />
      <slot v-else />
    </div>
  </Card>
</template>

<style scoped lang="scss">
.app-card {
  gap: 0;
  padding: 0;
  background: var(--surface-strong);
  border: 1px solid var(--border);
  box-shadow: none;
  transition: border-color var(--motion-fast) var(--motion-easing), background-color var(--motion-fast) var(--motion-easing);
}
.app-card[data-borderless] { border-color: transparent; }
.app-card__head { padding: 16px 20px; border-bottom: 1px solid var(--border); }
.app-card__body { padding: 20px; min-width: 0; }
.app-card[data-size=sm] .app-card__head { padding: 12px 16px; }
.app-card[data-size=sm] .app-card__body { padding: 16px; }

.app-card--highlight {
  border-color: color-mix(in srgb, var(--attention) 28%, var(--border));
  background: var(--surface-attention);
}

.app-card--flat,
.app-card--stat {
  box-shadow: none;
  background: transparent;
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
  font-size: 13px;
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
