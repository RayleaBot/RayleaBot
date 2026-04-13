<script setup lang="ts">
import { computed } from 'vue'

import { useUiShellStore } from '@/stores/ui-shell'

const uiShellStore = useUiShellStore()

const pageClasses = computed(() => ({
  'app-page--fixed-width': uiShellStore.preferences.contentWidth === 'fixed',
}))

defineProps<{
  description?: string
  eyebrow?: string
  fullHeight?: boolean
  title: string
}>()
</script>

<template>
  <div :class="['app-page', pageClasses, { 'app-page--full-height': fullHeight }]">
    <header class="app-page__header">
      <div class="app-page__heading">
        <span v-if="eyebrow" class="page-eyebrow">{{ eyebrow }}</span>
        <h1>{{ title }}</h1>
        <p v-if="description">{{ description }}</p>
      </div>

      <div v-if="$slots.extra" class="app-page__extra">
        <slot name="extra" />
      </div>
    </header>

    <div v-if="$slots.toolbar" class="app-page__toolbar">
      <slot name="toolbar" />
    </div>

    <div class="app-page__content">
      <slot />
    </div>
  </div>
</template>
