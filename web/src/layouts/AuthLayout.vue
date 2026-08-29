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
  width: clamp(160px, 24vw, 340px);
  height: auto;
  opacity: .12;
}
.auth-layout__fold--near { left: max(24px, calc(50% - 640px)); top: 24%; transform: rotate(-8deg); }

.auth-layout__surface {
  position: relative;
  width: min(440px, 100%);
  color: var(--auth-text);
  border: 1px solid var(--auth-border);
  border-radius: 16px;
  background: var(--auth-surface);
  animation: auth-surface-enter 240ms var(--motion-easing) both;
}

.auth-layout__toolbar { position: absolute; top: 24px; right: 24px; z-index: 2; }

.auth-layout__theme-toggle.ant-btn {
  display: grid;
  place-items: center;
  width: 36px;
  min-width: 36px;
  height: 36px;
  padding: 0;
  color: var(--auth-text-muted);
  border: 1px solid var(--auth-border);
  border-radius: 50%;
  background: var(--auth-control);
  transition: background-color 160ms var(--motion-easing), color 160ms var(--motion-easing);

  &:hover { color: var(--auth-brand-foreground); background: var(--auth-brand-soft); }
  &:focus-visible { outline: 2px solid var(--auth-focus); outline-offset: 2px; }
}

@keyframes auth-surface-enter {
  from { opacity: .72; transform: translateY(6px); }
  to { opacity: 1; transform: translateY(0); }
}

@media (max-height: 650px) {
  .auth-layout { place-items: start center; padding-block: 24px; }
}

@media (max-width: 600px) {
  .auth-layout { padding: 40px 20px; }
  .auth-layout__fold--near { left: -80px; top: 2%; opacity: .05; }
  .auth-layout__toolbar { top: 20px; right: 20px; }
  .auth-layout__theme-toggle.ant-btn { width: 44px; min-width: 44px; height: 44px; }
}

@media (prefers-reduced-motion: reduce) {
  .auth-layout__surface, .auth-layout__theme-toggle.ant-btn { animation: none; transition: none; }
}

@media (forced-colors: active) {
  .auth-layout__art { display: none; }
  .auth-layout__surface, .auth-layout__theme-toggle.ant-btn { border-color: CanvasText; }
  .auth-layout__theme-toggle.ant-btn:focus-visible { outline-color: Highlight; }
}
</style>
