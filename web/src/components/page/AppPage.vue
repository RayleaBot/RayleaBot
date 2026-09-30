<script setup lang="ts">
import { computed } from 'vue'
import { MenuIcon } from '@lucide/vue'

import AppButton from '@/components/AppButton.vue'
import ShellSearchButton from '@/components/shell/ShellSearchButton.vue'
import { t } from '@/i18n'
import { useUiShellStore } from '@/stores/ui-shell'

const uiShellStore = useUiShellStore()

const props = withDefaults(defineProps<{
  description?: string
  fullHeight?: boolean
  title: string
  width?: 'detail' | 'form' | 'wide'
}>(), {
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
    <!-- The first row of every page: title and description, then page search and the page's own actions. -->
    <header class="app-page__header">
      <AppButton class="app-page__menu mobile-only" variant="ghost" size="icon" :aria-label="t('shell.openMenu')" @click="uiShellStore.setMobileMenuOpen(true)">
        <template #icon><MenuIcon /></template>
      </AppButton>
      <div v-if="$slots.leading" class="app-page__leading">
        <slot name="leading" />
      </div>
      <div class="app-page__heading">
        <div class="app-page__title-row">
          <h1 v-if="!$slots.title">{{ title }}</h1>
          <div v-else class="app-page__title-slot-wrapper">
            <slot name="title" />
          </div>
          <div v-if="$slots.status" class="app-page__status">
            <slot name="status" />
          </div>
        </div>
        <p v-if="$slots.description || description" class="app-page__description">
          <slot name="description">{{ description }}</slot>
        </p>
      </div>

      <div class="app-page__tools">
        <ShellSearchButton />
        <div v-if="$slots.extra" class="app-page__extra">
          <slot name="extra" />
        </div>
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
