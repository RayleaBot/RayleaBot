<script setup lang="ts">
import type { Component } from 'vue'

// One row of a sectioned settings form: the section name on the left, its fields on the right.
// Rows share a box and are separated by dividers, never by nested boxes.
defineProps<{ title: string; icon?: Component }>()
</script>

<template>
  <section class="setting-section">
    <div class="setting-section__intro">
      <component :is="icon" v-if="icon" class="setting-section__icon" :size="20" aria-hidden="true" />
      <h2 class="setting-section__title">{{ title }}</h2>
    </div>
    <div class="setting-section__controls">
      <slot />
    </div>
  </section>
</template>

<style scoped lang="scss">
.setting-section {
  display: grid;
  grid-template-columns: minmax(150px, 190px) minmax(0, 1fr);
  gap: 24px;
  padding: 18px 20px;
  border-top: 1px solid var(--border);
}

.setting-section:first-child {
  border-top: 0;
}

.setting-section__intro {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  min-width: 0;
  padding-top: 2px;
}

.setting-section__icon {
  flex-shrink: 0;
  color: var(--muted);
}

.setting-section__title {
  margin: 0;
  color: var(--text);
  font-size: 15px;
  font-weight: 600;
  line-height: 20px;
}

.setting-section__controls {
  display: grid;
  gap: 12px;
  min-width: 0;
}
</style>
