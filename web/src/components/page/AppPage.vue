<script setup lang="ts">
import { computed } from 'vue'

import { useUiShellStore } from '@/stores/ui-shell'

const uiShellStore = useUiShellStore()

const props = withDefaults(defineProps<{
  description?: string
  eyebrow?: string
  fullHeight?: boolean
  showHeader?: boolean
  title: string
  width?: 'detail' | 'form' | 'wide'
}>(), {
  showHeader: true,
  width: 'wide',
})

const pageClasses = computed(() => {
  const usesFixedWidth = uiShellStore.preferences.contentWidth === 'fixed'

  return {
    'app-page--fixed-width': usesFixedWidth,
    [`app-page--${props.width}`]: usesFixedWidth && props.width !== 'wide',
  }
})
</script>

<template>
  <div :class="['app-page', pageClasses, { 'app-page--full-height': fullHeight }]">
    <h1 v-if="!showHeader" class="sr-only">{{ title }}</h1>
    <header v-else class="app-page__header">
      <div class="app-page__heading">
        <span v-if="eyebrow" class="page-eyebrow">{{ eyebrow }}</span>
        <div class="app-page__title-row">
          <h1 v-if="!$slots.title">{{ title }}</h1>
          <div v-else class="app-page__title-slot-wrapper">
            <slot name="title" />
          </div>
          <div v-if="$slots.status" class="app-page__status">
            <slot name="status" />
          </div>
        </div>
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
