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
  background: transparent;
}

.auth-layout__ambient::before,
.auth-layout__ambient::after {
  position: absolute;
  content: '';
  border: 1px solid var(--auth-canvas-focus);
  border-radius: 14px;
}

.auth-layout__ambient::before {
  width: min(54vw, 720px);
  height: min(54vw, 720px);
  top: 50%;
  left: 50%;
  opacity: 0.45;
  transform: translate(-50%, -50%);
}

.auth-layout__ambient::after {
  width: min(22vw, 280px);
  height: min(22vw, 280px);
  top: 8%;
  left: 8%;
  background: var(--auth-canvas-wash);
}

.auth-layout__surface {
  position: relative;
  z-index: 1;
  width: min(520px, calc(100vw - 32px));
  color: var(--auth-text);
  border: 1px solid var(--auth-border);
  border-radius: 14px;
  background: var(--auth-surface);
  box-shadow: var(--auth-panel-shadow);
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
  border-radius: 10px;
  background: var(--auth-control);
  transition:
    color 160ms cubic-bezier(0.16, 1, 0.3, 1),
    background-color 160ms cubic-bezier(0.16, 1, 0.3, 1),
    border-color 160ms cubic-bezier(0.16, 1, 0.3, 1),
    box-shadow 160ms cubic-bezier(0.16, 1, 0.3, 1);

  &:hover {
    color: var(--auth-brand-foreground);
    border-color: var(--auth-brand-foreground);
    background: var(--auth-brand-soft);
  }

  &:focus-visible {
    color: var(--auth-brand-foreground);
    border-color: var(--auth-brand-foreground);
    background: var(--auth-brand-soft);
    outline: 2px solid var(--auth-focus);
    outline-offset: 2px;
    box-shadow: none;
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

@media (forced-colors: active) {
  .auth-layout__surface,
  .auth-layout__theme-toggle.ant-btn {
    border-color: CanvasText;
  }

  .auth-layout__theme-toggle.ant-btn:focus-visible {
    outline-color: Highlight;
  }
}
</style>
