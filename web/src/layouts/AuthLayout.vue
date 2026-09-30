<script setup lang="ts">
import { computed } from 'vue'

import RayleaMark from '@/components/brand/RayleaMark.vue'
import ThemeModeMenu from '@/components/shell/ThemeModeMenu.vue'
import { t } from '@/i18n'
import { resolveAuthCssVariables } from '@/preferences/auth'
import { useUiShellStore } from '@/stores/ui-shell'
import { applyThemeWithMotion } from '@/motion/runtime'
import type { ThemeMode } from '@/preferences/app'
import type { ThemeMotionOrigin } from '@/motion/runtime'

const uiShellStore = useUiShellStore()
const authThemeStyle = computed(() => resolveAuthCssVariables(uiShellStore.resolvedThemeMode))

function setThemeModeWithMotion(mode: ThemeMode, origin: ThemeMotionOrigin) {
  if (mode !== uiShellStore.themeMode) applyThemeWithMotion(() => uiShellStore.setThemeMode(mode), origin)
}
</script>

<template>
    <main class="auth-layout" :data-auth-theme="uiShellStore.resolvedThemeMode" :style="authThemeStyle">
      <div class="auth-layout__brand">
        <RayleaMark />
        <span>{{ t('app.brand') }}</span>
      </div>
      <div class="auth-layout__content">
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
        <p class="auth-layout__caption">{{ t('auth.surface') }}</p>
      </div>
    </main>
</template>

<style scoped lang="scss">
@use '@/styles/breakpoints.generated' as bp;
// A white page with one light gray panel; the fields and secondary controls inside it are white.
.auth-layout {
  position: relative;
  display: grid;
  place-items: center;
  width: 100%;
  min-height: 100vh;
  min-height: 100dvh;
  padding: 100px 24px 64px;
  color: var(--auth-text);
  background: var(--auth-canvas);
  caret-color: var(--auth-brand-foreground);
  scrollbar-color: var(--auth-border-control) var(--auth-canvas);
  ::selection { color: var(--auth-on-brand); background: var(--auth-brand-fill); }
}
.auth-layout__brand {
  position: absolute;
  top: 32px;
  left: 40px;
  display: flex;
  align-items: center;
  gap: 10px;
  color: var(--auth-text);
  font-size: var(--font-size-lg);
  font-weight: 700;
  letter-spacing: -.02em;
  :deep(.raylea-mark) { width: 26px; height: 28px; }
}
.auth-layout__content { width: min(448px, 100%); }
.auth-layout__surface {
  --control-fill: var(--auth-control);
  --control-fill-hover: color-mix(in srgb, var(--auth-control) 97%, var(--auth-text));
  position: relative;
  width: 100%;
  color: var(--auth-text);
  border: 1px solid transparent;
  border-radius: 28px;
  background: var(--auth-surface);
  box-shadow: var(--shadow-card);
  animation: auth-surface-enter 420ms var(--motion-easing) both;
}
.auth-layout__toolbar { position: absolute; top: 16px; right: 16px; z-index: 2; }
.auth-layout__theme-toggle.app-button {
  display: grid;
  place-items: center;
  width: 44px;
  min-width: 44px;
  height: 44px;
  padding: 0;
  color: var(--auth-text-muted);
  border: 1px solid transparent;
  border-radius: 50%;
  background: transparent;
  transition: background-color 160ms var(--motion-easing), color 160ms var(--motion-easing);
  &:hover { color: var(--auth-text); background: var(--auth-control); }
  &:focus-visible { outline: 2px solid var(--auth-focus); outline-offset: var(--focus-outline-offset); }
}
.auth-layout__caption {
  margin: 24px 0 0;
  color: var(--auth-text);
  font-size: 12px;
  line-height: 1.6;
  text-align: center;
}
@keyframes auth-surface-enter {
  from { opacity: .75; transform: translateY(8px); }
  to { opacity: 1; transform: translateY(0); }
}
@media (max-width: #{bp.$compactAuth}) {
  .auth-layout { padding: 88px 20px 32px; }
  .auth-layout__brand { top: 26px; left: 26px; font-size: 16px; }
  .auth-layout__surface { border-radius: 24px; }
  .auth-layout__toolbar { top: 12px; right: 12px; }
  .auth-layout__caption { margin-top: 20px; }
}
@media (max-height: #{bp.$shortViewport}) {
  .auth-layout { place-items: start center; padding-top: 80px; }
}
@media (prefers-reduced-motion: reduce) {
  .auth-layout__surface, .auth-layout__theme-toggle.app-button { animation: none; transition: none; }
}
@media (forced-colors: active) {
  .auth-layout__surface { border-color: CanvasText; background: Canvas; box-shadow: none; }
  .auth-layout__theme-toggle.app-button { border-color: CanvasText; }
  .auth-layout__theme-toggle.app-button:focus-visible { outline-color: Highlight; }
}
</style>
