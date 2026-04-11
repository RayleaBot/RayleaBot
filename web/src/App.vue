<script setup lang="ts">
import { computed, watchEffect } from 'vue'
import zhCN from 'ant-design-vue/es/locale/zh_CN'

import { resolveThemeConfig } from '@/preferences/app'
import { useUiShellStore } from '@/stores/ui-shell'

const uiShellStore = useUiShellStore()
const themeConfig = computed(() => resolveThemeConfig(uiShellStore.themeMode))

watchEffect(() => {
  if (typeof document === 'undefined') {
    return
  }

  document.documentElement.dataset.theme = uiShellStore.themeMode
})
</script>

<template>
  <a-config-provider :locale="zhCN" :theme="themeConfig">
    <a-app :class="['app-root', `app-root--${uiShellStore.themeMode}`]">
      <RouterView />
    </a-app>
  </a-config-provider>
</template>
