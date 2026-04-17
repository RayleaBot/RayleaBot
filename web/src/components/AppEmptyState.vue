<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  actionLabel?: string
  description?: string
  icon?: 'box' | 'search' | 'command' | 'plugin' | 'log' | 'generic'
  title?: string
}>()

const emit = defineEmits<{
  (e: 'action'): void
}>()

const iconClass = computed(() => `app-empty-state__icon--${props.icon ?? 'generic'}`)
</script>

<template>
  <div class="app-empty-state">
    <div class="app-empty-state__visual" :class="iconClass">
      <slot name="icon">
        <!-- Generic / Box icon (CSS-drawn) -->
        <div v-if="!icon || icon === 'box'" class="css-icon-box">
          <div class="css-icon-box__lid" />
          <div class="css-icon-box__body" />
        </div>
        <!-- Search icon -->
        <div v-else-if="icon === 'search'" class="css-icon-search">
          <div class="css-icon-search__ring" />
          <div class="css-icon-search__handle" />
        </div>
        <!-- Command icon -->
        <div v-else-if="icon === 'command'" class="css-icon-command">
          <div class="css-icon-command__prompt">&gt;_</div>
        </div>
        <!-- Plugin icon -->
        <div v-else-if="icon === 'plugin'" class="css-icon-plugin">
          <div class="css-icon-plugin__block" />
          <div class="css-icon-plugin__block css-icon-plugin__block--small" />
        </div>
        <!-- Log icon -->
        <div v-else-if="icon === 'log'" class="css-icon-log">
          <div v-for="i in 4" :key="i" class="css-icon-log__line" :class="{ 'css-icon-log__line--short': i === 4 }" />
        </div>
      </slot>
    </div>

    <div v-if="title || description" class="app-empty-state__text">
      <strong v-if="title" class="app-empty-state__title">{{ title }}</strong>
      <p v-if="description" class="app-empty-state__desc">{{ description }}</p>
    </div>

    <div v-if="actionLabel" class="app-empty-state__action">
      <a-button type="primary" @click="emit('action')">
        {{ actionLabel }}
      </a-button>
    </div>
  </div>
</template>

<style scoped lang="scss">
.app-empty-state {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: var(--space-lg);
  padding: var(--space-2xl) var(--space-lg);
  text-align: center;
}

.app-empty-state__visual {
  width: 80px;
  height: 80px;
  border-radius: var(--radius-xl);
  background: var(--surface-soft);
  border: 1px solid var(--border);
  display: flex;
  align-items: center;
  justify-content: center;
  position: relative;
}

.app-empty-state__text {
  display: flex;
  flex-direction: column;
  gap: var(--space-sm);
  max-width: 320px;
}

.app-empty-state__title {
  font-size: 1rem;
  font-weight: 600;
  color: var(--text);
}

.app-empty-state__desc {
  margin: 0;
  font-size: 0.88rem;
  color: var(--muted);
  line-height: 1.55;
}

.app-empty-state__action {
  margin-top: var(--space-xs);
}

/* CSS-drawn icons */
.css-icon-box {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 2px;
}

.css-icon-box__lid {
  width: 28px;
  height: 8px;
  border-radius: 2px 2px 0 0;
  background: color-mix(in srgb, var(--muted) 35%, transparent);
}

.css-icon-box__body {
  width: 24px;
  height: 16px;
  border-radius: 0 0 3px 3px;
  background: color-mix(in srgb, var(--muted) 22%, transparent);
}

.css-icon-search {
  position: relative;
  width: 32px;
  height: 32px;
}

.css-icon-search__ring {
  width: 20px;
  height: 20px;
  border-radius: 50%;
  border: 2.5px solid color-mix(in srgb, var(--muted) 40%, transparent);
}

.css-icon-search__handle {
  position: absolute;
  bottom: 4px;
  right: 6px;
  width: 10px;
  height: 2.5px;
  border-radius: 2px;
  background: color-mix(in srgb, var(--muted) 40%, transparent);
  transform: rotate(45deg);
  transform-origin: left center;
}

.css-icon-command {
  display: flex;
  align-items: center;
  justify-content: center;
}

.css-icon-command__prompt {
  font-family: "Cascadia Mono", "Consolas", monospace;
  font-size: 1.1rem;
  font-weight: 600;
  color: color-mix(in srgb, var(--muted) 50%, transparent);
  letter-spacing: 0.04em;
}

.css-icon-plugin {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 3px;
}

.css-icon-plugin__block {
  width: 26px;
  height: 14px;
  border-radius: 3px;
  background: color-mix(in srgb, var(--muted) 25%, transparent);
}

.css-icon-plugin__block--small {
  width: 18px;
  height: 10px;
}

.css-icon-log {
  display: flex;
  flex-direction: column;
  gap: 4px;
  width: 28px;
}

.css-icon-log__line {
  height: 3px;
  border-radius: 2px;
  background: color-mix(in srgb, var(--muted) 28%, transparent);
}

.css-icon-log__line--short {
  width: 60%;
}
</style>
