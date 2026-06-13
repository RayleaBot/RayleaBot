<script setup lang="ts">
import { computed } from 'vue'
import { TranslationOutlined } from '@ant-design/icons-vue'

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
      <a-popover placement="bottom" trigger="click">
        <template #content>
          <div class="auth-layout__pending-panel">{{ t('shell.languagePending') }}</div>
        </template>

        <button
          type="button"
          class="auth-layout__toolbar-button"
          :aria-label="t('shell.language')"
          data-testid="auth-language"
        >
          <TranslationOutlined />
        </button>
      </a-popover>
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
  border: 1px solid var(--auth-glass-border);
  border-radius: 999px;
  background: color-mix(in srgb, var(--auth-glass-bg) 80%, transparent);
  backdrop-filter: blur(16px) saturate(140%);
  -webkit-backdrop-filter: blur(16px) saturate(140%);
}

.auth-layout__toolbar-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  border: 0;
  border-radius: 999px;
  background: transparent;
  color: var(--muted);
  font-size: 16px;
  cursor: pointer;
  transition: color 0.2s ease, background-color 0.2s ease, transform 0.2s ease;

  &:hover {
    background: color-mix(in srgb, var(--text) 8%, transparent);
    color: var(--text);
    transform: translateY(-1px);
  }

  &:focus-visible {
    outline: 2px solid var(--auth-accent);
    outline-offset: 1px;
  }
}

.auth-layout__pending-panel {
  max-width: 220px;
  color: var(--muted);
  font-size: 13px;
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

@media (prefers-reduced-motion: reduce) {
  .auth-layout__toolbar-button {
    transition: none;
  }
}
</style>
