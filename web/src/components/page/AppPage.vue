<script setup lang="ts">
import { MotionDirective as vMotion } from '@vueuse/motion'
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
        <h1 v-if="!$slots.title">{{ title }}</h1>
        <div v-else class="app-page__title-slot-wrapper">
          <slot name="title" />
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

    <div
      v-motion="{
        initial: { opacity: 0 },
        enter: { opacity: 1, transition: { duration: 280, ease: 'easeOut' } },
      }"
      class="app-page__content"
    >
      <slot />
    </div>
  </div>
</template>
