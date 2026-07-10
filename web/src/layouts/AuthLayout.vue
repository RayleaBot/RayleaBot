<script setup lang="ts">
import { computed } from 'vue'

import AuroraBackground from '@/components/auth/AuroraBackground.vue'
import ThemeToggleSwitch from '@/components/shell/ThemeToggleSwitch.vue'
import { t } from '@/i18n'
import { useUiShellStore } from '@/stores/ui-shell'

const uiShellStore = useUiShellStore()
const themeToggleLabel = computed(() => (
  uiShellStore.themeMode === 'dark' ? t('shell.switchLightTheme') : t('shell.switchDarkTheme')
))
</script>

<template>
  <div class="auth-layout">
    <AuroraBackground />

    <div class="auth-layout__toolbar">
      <a-tooltip :title="themeToggleLabel">
        <ThemeToggleSwitch
          class="auth-layout__theme-toggle"
          :checked="uiShellStore.themeMode === 'dark'"
          :label="themeToggleLabel"
          size="default"
          test-id="auth-theme-toggle"
          @toggle="uiShellStore.toggleThemeMode()"
        />
      </a-tooltip>
    </div>

    <RouterView />
  </div>
</template>

<style scoped lang="scss">
.auth-layout {
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 100vh;
  padding: 72px 24px 32px;
  background: var(--aurora-bg);
  isolation: isolate;
}

.auth-layout__toolbar {
  position: absolute;
  top: 20px;
  right: 24px;
  z-index: 10;
  display: flex;
  align-items: center;
  gap: 10px;
  padding: 8px 10px;
  border: 1px solid var(--auth-toolbar-border);
  border-radius: 999px;
  background: var(--auth-toolbar-bg);
}

@media (max-width: 960px) {
  .auth-layout {
    padding: 64px 20px 28px;
  }
}

@media (max-width: 480px) {
  .auth-layout {
    padding: 60px 14px 22px;
  }

  .auth-layout__toolbar {
    top: 12px;
    right: 14px;
  }
}

</style>
