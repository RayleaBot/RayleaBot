<script setup lang="ts">
import { computed } from 'vue'

import AuthParticleField from '@/components/auth/AuthParticleField.vue'
import ThemeModeMenu from '@/components/shell/ThemeModeMenu.vue'
import {
  resolveAuthCssVariables,
  resolveAuthParticlePalette,
  resolveAuthThemeConfig,
} from '@/preferences/auth'
import { useUiShellStore } from '@/stores/ui-shell'
import { applyThemeWithMotion } from '@/motion/runtime'
import type { ThemeMode } from '@/preferences/app'

const uiShellStore = useUiShellStore()
const authThemeConfig = computed(() => resolveAuthThemeConfig(uiShellStore.resolvedThemeMode))
const authThemeStyle = computed(() => resolveAuthCssVariables(uiShellStore.resolvedThemeMode))
const authParticlePalette = computed(() => resolveAuthParticlePalette(uiShellStore.resolvedThemeMode))

function setThemeModeWithMotion(mode: ThemeMode) {
  applyThemeWithMotion(() => uiShellStore.setThemeMode(mode))
}
</script>

<template>
  <a-config-provider :theme="authThemeConfig">
    <main
      class="auth-layout"
      :data-auth-theme="uiShellStore.resolvedThemeMode"
      :style="authThemeStyle"
    >
      <div aria-hidden="true" class="auth-layout__motion">
        <div class="auth-layout__atmosphere" :style="authThemeStyle">
          <div class="auth-layout__ambient" />
          <AuthParticleField :palette="authParticlePalette" />
        </div>
      </div>

      <section class="auth-layout__surface">
        <div class="auth-layout__toolbar">
          <ThemeModeMenu
            class="auth-layout__theme-toggle"
            :mode="uiShellStore.themeMode"
            :resolved-mode="uiShellStore.resolvedThemeMode"
            test-id="auth-theme-toggle"
            @change="setThemeModeWithMotion"
          />
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
  .auth-layout__theme-toggle.ant-btn {
    animation: none;
    transition: none;
  }
}
</style>
