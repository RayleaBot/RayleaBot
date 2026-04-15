<script setup lang="ts">
import type { Component } from 'vue'

type StatusTone = 'default' | 'success' | 'warning' | 'danger' | 'primary'

withDefaults(defineProps<{
  description?: string
  icon?: Component
  label: string
  tone?: StatusTone
  value: string | number
}>(), {
  tone: 'default',
})
</script>

<template>
  <div :class="['app-stat-card stat-card', `app-stat-card--${tone}`, `stat-card--${tone}`]">
    <component :is="icon" v-if="icon" class="app-stat-card__icon" />
    <div class="app-stat-card__content">
      <span class="app-stat-card__label">{{ label }}</span>
      <strong class="app-stat-card__value">{{ value }}</strong>
      <span v-if="description" class="app-stat-card__desc">{{ description }}</span>
    </div>
    <div class="app-stat-card__accent" />
  </div>
</template>

<style scoped lang="scss">
.app-stat-card {
  position: relative;
  display: flex;
  align-items: flex-start;
  gap: 14px;
  padding: 16px;
  border-radius: var(--app-card-radius);
  border: 1px solid var(--border);
  background: var(--surface-strong);
  overflow: hidden;
  transition: transform 0.2s ease, box-shadow 0.2s ease, border-color 0.2s ease;
}

.app-stat-card:hover {
  transform: translateY(-2px);
  box-shadow: var(--shadow-lg);
  border-color: var(--border-strong);
}

.app-stat-card__accent {
  position: absolute;
  inset: 0 0 auto;
  height: 3px;
  background: color-mix(in srgb, var(--muted) 30%, transparent);
}

.app-stat-card--primary .app-stat-card__accent {
  background: color-mix(in srgb, var(--accent) 80%, transparent);
}

.app-stat-card--success .app-stat-card__accent {
  background: color-mix(in srgb, var(--success) 80%, transparent);
}

.app-stat-card--warning .app-stat-card__accent {
  background: color-mix(in srgb, var(--warning) 80%, transparent);
}

.app-stat-card--danger .app-stat-card__accent {
  background: color-mix(in srgb, var(--danger) 80%, transparent);
}

.app-stat-card__icon {
  flex-shrink: 0;
  font-size: 2rem;
  color: color-mix(in srgb, var(--accent) 24%, transparent);
  margin-top: 2px;
}

.app-stat-card--success .app-stat-card__icon {
  color: color-mix(in srgb, var(--success) 30%, transparent);
}

.app-stat-card--warning .app-stat-card__icon {
  color: color-mix(in srgb, var(--warning) 30%, transparent);
}

.app-stat-card--danger .app-stat-card__icon {
  color: color-mix(in srgb, var(--danger) 30%, transparent);
}

.app-stat-card__content {
  display: flex;
  flex-direction: column;
  gap: 4px;
  min-width: 0;
}

.app-stat-card__label {
  font-size: 0.78rem;
  font-weight: 600;
  letter-spacing: 0.02em;
  text-transform: uppercase;
  color: var(--muted);
}

.app-stat-card__value {
  font-size: 1.6rem;
  font-weight: 700;
  line-height: 1.2;
  color: var(--text);
}

.app-stat-card__desc {
  font-size: 0.84rem;
  color: var(--muted);
  line-height: 1.4;
}
</style>
