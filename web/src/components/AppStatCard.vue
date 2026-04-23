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
    <div v-if="icon" class="app-stat-card__icon-wrap">
      <component :is="icon" class="app-stat-card__icon" />
    </div>
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
  box-shadow: var(--shadow-xs);
  overflow: hidden;
  transition: transform 0.2s ease, box-shadow 0.2s ease, border-color 0.2s ease;
}

.app-stat-card:hover {
  transform: scale(1.02);
  box-shadow: var(--shadow-card);
  border-color: var(--border-strong);
}

.app-stat-card__accent {
  position: absolute;
  inset: 0 0 auto;
  height: 3px;
  background: var(--muted);
}

.app-stat-card--primary .app-stat-card__accent {
  background: var(--accent);
}

.app-stat-card--success .app-stat-card__accent {
  background: var(--success);
}

.app-stat-card--warning .app-stat-card__accent {
  background: var(--warning);
}

.app-stat-card--danger .app-stat-card__accent {
  background: var(--danger);
}

.app-stat-card__icon-wrap {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  width: 40px;
  height: 40px;
  border-radius: 50%;
  border: 1px solid var(--border);
  background: var(--surface-soft);
  flex-shrink: 0;
}

.app-stat-card__icon-wrap::before {
  content: "";
  position: absolute;
  inset: -1px;
  border-radius: 50%;
  background: var(--shadow-card);
  z-index: -1;
}

.app-stat-card--primary .app-stat-card__icon-wrap {
  background: var(--surface-accent);
  border-color: color-mix(in srgb, var(--accent) 30%, transparent);
}

.app-stat-card--success .app-stat-card__icon-wrap {
  background: var(--surface-success);
  border-color: color-mix(in srgb, var(--success) 30%, transparent);
}

.app-stat-card--warning .app-stat-card__icon-wrap {
  background: var(--surface-warning);
  border-color: color-mix(in srgb, var(--warning) 30%, transparent);
}

.app-stat-card--danger .app-stat-card__icon-wrap {
  background: var(--surface-danger);
  border-color: color-mix(in srgb, var(--danger) 30%, transparent);
}

.app-stat-card__icon {
  font-size: 1.4rem;
  color: var(--accent);
}

.app-stat-card--primary .app-stat-card__icon {
  color: var(--accent);
}

.app-stat-card--success .app-stat-card__icon {
  color: var(--success);
}

.app-stat-card--warning .app-stat-card__icon {
  color: var(--warning);
}

.app-stat-card--danger .app-stat-card__icon {
  color: var(--danger);
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
