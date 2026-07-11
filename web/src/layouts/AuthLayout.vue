<script setup lang="ts">
import { computed } from 'vue'
import { BulbFilled, BulbOutlined } from '@ant-design/icons-vue'

import AuthParticleField from '@/components/auth/AuthParticleField.vue'
import { t } from '@/i18n'
import {
  resolveAuthCssVariables,
  resolveAuthParticlePalette,
  resolveAuthThemeConfig,
} from '@/preferences/auth'
import { useUiShellStore } from '@/stores/ui-shell'

const uiShellStore = useUiShellStore()
const authThemeConfig = computed(() => resolveAuthThemeConfig(uiShellStore.themeMode))
const authThemeStyle = computed(() => resolveAuthCssVariables(uiShellStore.themeMode))
const authParticlePalette = computed(() => resolveAuthParticlePalette(uiShellStore.themeMode))
const themeToggleLabel = computed(() => (
  uiShellStore.themeMode === 'dark' ? t('shell.switchLightTheme') : t('shell.switchDarkTheme')
))
</script>

<template>
  <a-config-provider :theme="authThemeConfig">
    <main
      class="auth-layout"
      :data-auth-theme="uiShellStore.themeMode"
      :style="authThemeStyle"
    >
      <div aria-hidden="true" class="auth-layout__motion">
        <Transition name="auth-atmosphere" appear>
          <div
            :key="uiShellStore.themeMode"
            class="auth-layout__atmosphere"
            :style="authThemeStyle"
          >
            <div class="auth-layout__ambient" />
            <AuthParticleField :palette="authParticlePalette" />
          </div>
        </Transition>
      </div>

      <section class="auth-layout__surface">
        <div class="auth-layout__toolbar">
          <a-tooltip :title="themeToggleLabel">
            <a-button
              class="auth-layout__theme-toggle"
              type="text"
              :aria-label="themeToggleLabel"
              data-testid="auth-theme-toggle"
              @click="uiShellStore.toggleThemeMode()"
            >
              <span class="auth-layout__theme-icon" aria-hidden="true">
                <BulbOutlined
                  class="auth-layout__theme-glyph auth-layout__theme-glyph--light"
                />
                <BulbFilled
                  class="auth-layout__theme-glyph auth-layout__theme-glyph--dark"
                />
              </span>
            </a-button>
          </a-tooltip>
        </div>

        <RouterView />
      </section>
    </main>
  </a-config-provider>
</template>

<style scoped lang="scss">
.auth-layout {
  position: relative;
  display: grid;
  width: 100%;
  min-height: 100vh;
  min-height: 100dvh;
  place-items: center;
  overflow: auto;
  padding: clamp(72px, 10vh, 120px) 24px;
  color: var(--auth-text);
  background-color: var(--auth-canvas);
  isolation: isolate;
  transition:
    color 200ms cubic-bezier(0.16, 1, 0.3, 1),
    background-color 200ms cubic-bezier(0.16, 1, 0.3, 1);
}

.auth-layout__motion {
  position: fixed;
  z-index: -1;
  inset: 0;
  contain: paint;
  overflow: hidden;
  pointer-events: none;
}

.auth-layout__atmosphere {
  position: absolute;
  inset: 0;
}

.auth-layout__ambient {
  position: absolute;
  inset: 0;
  background-image:
    radial-gradient(circle at 50% 45%, var(--auth-canvas-focus) 0, transparent min(36vw, 32rem)),
    radial-gradient(circle at 16% 12%, var(--auth-canvas-wash) 0, transparent min(44vw, 40rem));
  background-position: center;
}

.auth-atmosphere-enter-active,
.auth-atmosphere-leave-active {
  transition: opacity 200ms cubic-bezier(0.16, 1, 0.3, 1);
}

.auth-atmosphere-enter-from,
.auth-atmosphere-leave-to {
  opacity: 0;
}

.auth-atmosphere-leave-active {
  position: absolute;
}

.auth-layout__surface {
  position: relative;
  z-index: 1;
  width: min(520px, calc(100vw - 32px));
  color: var(--auth-text);
  border: 1px solid var(--auth-border);
  border-radius: 12px;
  background: var(--auth-surface);
  box-shadow: var(--auth-panel-shadow), inset 0 1px 0 var(--auth-panel-highlight);
  animation: auth-surface-enter 220ms cubic-bezier(0.16, 1, 0.3, 1) both;
  transition:
    color 200ms cubic-bezier(0.16, 1, 0.3, 1),
    background-color 200ms cubic-bezier(0.16, 1, 0.3, 1),
    border-color 200ms cubic-bezier(0.16, 1, 0.3, 1),
    box-shadow 200ms cubic-bezier(0.16, 1, 0.3, 1);
}

.auth-layout__toolbar {
  position: absolute;
  top: 28px;
  right: 28px;
  z-index: 2;
}

.auth-layout__theme-toggle.ant-btn {
  display: grid;
  width: 40px;
  min-width: 40px;
  height: 40px;
  padding: 0;
  place-items: center;
  color: var(--auth-text-muted);
  border: 1px solid var(--auth-border);
  border-radius: 8px;
  background: var(--auth-control);
  box-shadow: inset 0 1px 0 var(--auth-panel-highlight);
  transition:
    color 160ms cubic-bezier(0.16, 1, 0.3, 1),
    background-color 160ms cubic-bezier(0.16, 1, 0.3, 1),
    border-color 160ms cubic-bezier(0.16, 1, 0.3, 1),
    box-shadow 160ms cubic-bezier(0.16, 1, 0.3, 1);

  &:hover,
  &:focus-visible {
    color: var(--auth-cool);
    border-color: var(--auth-cool);
    background: var(--auth-cool-soft);
    box-shadow: inset 0 1px 0 var(--auth-panel-highlight), 0 0 0 2px var(--auth-cool-soft);
  }
}

.auth-layout__theme-icon {
  position: relative;
  display: block;
  width: 16px;
  height: 16px;
}

.auth-layout__theme-glyph {
  position: absolute;
  inset: 0;
  transition: opacity 160ms cubic-bezier(0.16, 1, 0.3, 1);
}

.auth-layout__theme-glyph--dark,
.auth-layout[data-auth-theme='dark'] .auth-layout__theme-glyph--light {
  opacity: 0;
}

.auth-layout[data-auth-theme='dark'] .auth-layout__theme-glyph--dark {
  opacity: 1;
}

@keyframes auth-surface-enter {
  from {
    opacity: 0;
  }

  to {
    opacity: 1;
  }
}

@media (max-height: 650px) {
  .auth-layout {
    place-items: start center;
    padding-block: 64px 24px;
  }
}

@media (max-width: 600px) {
  .auth-layout {
    padding: 64px 12px 20px;
  }

  .auth-layout__toolbar {
    top: 18px;
    right: 18px;
  }

  .auth-layout__theme-toggle.ant-btn {
    width: 44px;
    min-width: 44px;
    height: 44px;
  }
}

@media (prefers-reduced-motion: reduce) {
  .auth-layout,
  .auth-layout__surface,
  .auth-layout__theme-toggle.ant-btn,
  .auth-layout__theme-glyph,
  .auth-atmosphere-enter-active,
  .auth-atmosphere-leave-active {
    animation: none;
    transition: none;
  }
}
</style>
