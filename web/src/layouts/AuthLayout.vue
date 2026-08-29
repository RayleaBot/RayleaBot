<script setup lang="ts">
import { computed } from 'vue'

import RayleaMark from '@/components/brand/RayleaMark.vue'
import ThemeModeMenu from '@/components/shell/ThemeModeMenu.vue'
import { resolveAuthCssVariables, resolveAuthThemeConfig } from '@/preferences/auth'
import { useUiShellStore } from '@/stores/ui-shell'
import { applyThemeWithMotion } from '@/motion/runtime'
import type { ThemeMode } from '@/preferences/app'

const uiShellStore = useUiShellStore()
const authThemeConfig = computed(() => resolveAuthThemeConfig(uiShellStore.resolvedThemeMode))
const authThemeStyle = computed(() => resolveAuthCssVariables(uiShellStore.resolvedThemeMode))

function setThemeModeWithMotion(mode: ThemeMode) {
  applyThemeWithMotion(() => uiShellStore.setThemeMode(mode))
}
</script>

<template>
  <a-config-provider :theme="authThemeConfig">
    <main class="auth-layout" :data-auth-theme="uiShellStore.resolvedThemeMode" :style="authThemeStyle">
      <div aria-hidden="true" class="auth-layout__art">
        <RayleaMark class="auth-layout__fold auth-layout__fold--near" variant="neutral" />
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
  isolation: isolate;
  display: grid;
  place-items: center;
  width: 100%;
  min-height: 100vh;
  min-height: 100dvh;
  overflow: auto;
  padding: 64px 24px;
  color: var(--auth-text);
  background: var(--auth-canvas);
}

.auth-layout__art {
  position: fixed;
  z-index: -1;
  inset: 0;
  overflow: hidden;
  contain: paint;
  pointer-events: none;
}

.auth-layout__fold {
  position: absolute;
  width: clamp(190px, 23vw, 300px);
  height: auto;
  opacity: .07;
}
.auth-layout__fold--near {
  top: 50%;
  left: max(20px, calc(50% - 390px));
  transform: translateY(-54%) rotate(-10deg);
}

.auth-layout__surface {
  position: relative;
  width: min(432px, 100%);
  color: var(--auth-text);
  border: 0;
  border-radius: 18px;
  background: var(--auth-surface);
  box-shadow: var(--auth-panel-shadow);
  animation: auth-surface-enter 220ms var(--motion-easing) both;
}

.auth-layout__toolbar { position: absolute; top: 28px; right: 28px; z-index: 2; }

.auth-layout__theme-toggle.ant-btn {
  display: grid;
  place-items: center;
  width: 36px;
  min-width: 36px;
  height: 36px;
  padding: 0;
  color: var(--auth-text-muted);
  border: 0;
  border-radius: 10px;
  background: transparent;
  transition: background-color 160ms var(--motion-easing), color 160ms var(--motion-easing);

  &:hover { color: var(--auth-brand-foreground); background: var(--auth-control-hover); }
  &:focus-visible { outline: 2px solid var(--auth-focus); outline-offset: 2px; }
}

@keyframes auth-surface-enter {
  from { opacity: .82; transform: translateY(4px); }
  to { opacity: 1; transform: translateY(0); }
}

@media (max-height: 650px) {
  .auth-layout { place-items: start center; padding-block: 24px; }
}

@media (max-width: 600px) {
  .auth-layout { padding: 32px 16px; }
  .auth-layout__surface { border-radius: 16px; }
  .auth-layout__fold { width: 180px; }
  .auth-layout__fold--near { top: 8%; left: -72px; opacity: .04; transform: rotate(-10deg); }
  .auth-layout__toolbar { top: 20px; right: 20px; }
  .auth-layout__theme-toggle.ant-btn { width: 44px; min-width: 44px; height: 44px; }
}

@media (prefers-reduced-motion: reduce) {
  .auth-layout__surface, .auth-layout__theme-toggle.ant-btn { animation: none; transition: none; }
}

@media (forced-colors: active) {
  .auth-layout__art { display: none; }
  .auth-layout__surface {
    border: 1px solid CanvasText;
    box-shadow: none;
  }
  .auth-layout__theme-toggle.ant-btn { border: 1px solid CanvasText; }
  .auth-layout__theme-toggle.ant-btn:focus-visible { outline-color: Highlight; }
}
</style>
