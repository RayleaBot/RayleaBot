<script setup lang="ts">
import { computed, watchEffect } from 'vue'
import zhCN from 'ant-design-vue/es/locale/zh_CN'

import { t } from '@/i18n'
import { resolvePreferenceCssVariables, resolveThemeConfig } from '@/preferences/app'
import { useUiShellStore } from '@/stores/ui-shell'

const uiShellStore = useUiShellStore()
const themeConfig = computed(() => resolveThemeConfig(uiShellStore.preferences))

watchEffect(() => {
  if (typeof document === 'undefined') {
    return
  }

  const root = document.documentElement
  const cssVariables = resolvePreferenceCssVariables(uiShellStore.preferences)

  root.dataset.theme = uiShellStore.preferences.themeMode
  root.dataset.density = uiShellStore.preferences.density
  root.dataset.contentWidth = uiShellStore.preferences.contentWidth

  for (const [key, value] of Object.entries(cssVariables)) {
    root.style.setProperty(key, value)
  }
})
</script>

<template>
  <a-config-provider :locale="zhCN" :theme="themeConfig">
    <a-app :class="['app-root', `app-root--${uiShellStore.preferences.themeMode}`, `app-root--${uiShellStore.preferences.density}`]">
      <RouterView v-slot="{ Component }">
        <component :is="Component" v-if="Component" />
        <div v-else class="app-startup" role="status" aria-live="polite">
          <a-spin :tip="t('app.loading')" />
        </div>
      </RouterView>
    </a-app>
  </a-config-provider>
</template>

<style scoped lang="scss">
.app-startup {
  display: grid;
  min-height: 100vh;
  place-items: center;
  background: var(--app-background, #f6f7fb);
}
</style>
