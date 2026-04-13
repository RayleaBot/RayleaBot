<script setup lang="ts">
import { computed } from 'vue'
import { BulbOutlined, TranslationOutlined } from '@ant-design/icons-vue'

import { t } from '@/i18n'
import { useUiShellStore } from '@/stores/ui-shell'

const uiShellStore = useUiShellStore()
const themeToggleLabel = computed(() => (
  uiShellStore.themeMode === 'dark' ? t('shell.switchLightTheme') : t('shell.switchDarkTheme')
))
</script>

<template>
  <div class="auth-layout">
    <section class="auth-layout__hero">
      <div class="auth-layout__toolbar">
        <a-popover placement="bottom" trigger="click">
          <template #content>
            <div class="auth-layout__pending-panel">{{ t('shell.languagePending') }}</div>
          </template>

          <a-button
            class="auth-layout__toolbar-button"
            type="text"
            :aria-label="t('shell.language')"
            data-testid="auth-language"
          >
            <template #icon>
              <TranslationOutlined />
            </template>
          </a-button>
        </a-popover>
        <a-tooltip :title="themeToggleLabel">
          <a-button
            class="auth-layout__toolbar-button"
            type="text"
            :aria-label="themeToggleLabel"
            data-testid="auth-theme-toggle"
            @click="uiShellStore.toggleThemeMode()"
          >
            <template #icon>
              <BulbOutlined />
            </template>
          </a-button>
        </a-tooltip>
      </div>

      <div class="auth-layout__intro-panel">
        <div class="auth-layout__brand">
          <span class="auth-layout__brand-badge">R</span>
          <div class="auth-layout__brand-copy">
            <strong>RayleaBot</strong>
            <span>{{ t('auth.surface') }}</span>
          </div>
        </div>

        <div class="auth-layout__hero-copy">
          <h1>{{ t('auth.heroTitle') }}</h1>
          <p>{{ t('auth.heroBody') }}</p>
        </div>

        <div class="auth-layout__highlights">
          <span>{{ t('auth.heroFeatureStatus') }}</span>
          <span>{{ t('auth.heroFeaturePlugins') }}</span>
          <span>{{ t('auth.heroFeatureProtocols') }}</span>
        </div>
      </div>
    </section>

    <section class="auth-layout__panel">
      <RouterView />
    </section>
  </div>
</template>
