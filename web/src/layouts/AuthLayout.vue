<script setup lang="ts">
import { computed, ref } from 'vue'

import RayleaMark from '@/components/brand/RayleaMark.vue'
import ThemeModeMenu from '@/components/shell/ThemeModeMenu.vue'
import { useLiquidGlass } from '@/components/auth/liquid-glass'
import { t } from '@/i18n'
import { resolveAuthCssVariables } from '@/preferences/auth'
import { useUiShellStore } from '@/stores/ui-shell'
import { applyThemeWithMotion } from '@/motion/runtime'
import type { ThemeMode } from '@/preferences/app'
import type { ThemeMotionOrigin } from '@/motion/runtime'

const uiShellStore = useUiShellStore()
const surface = ref<HTMLElement | null>(null)
const { filterId, displacement, moveHighlight, resetHighlight } = useLiquidGlass(surface)
const authThemeStyle = computed(() => resolveAuthCssVariables(uiShellStore.resolvedThemeMode))

function setThemeModeWithMotion(mode: ThemeMode, origin: ThemeMotionOrigin) {
  if (mode !== uiShellStore.themeMode) applyThemeWithMotion(() => uiShellStore.setThemeMode(mode), origin)
}
</script>

<template>
    <main class="auth-layout" :data-auth-theme="uiShellStore.resolvedThemeMode" :style="authThemeStyle">
      <div aria-hidden="true" class="auth-layout__art" />
      <svg class="auth-layout__filters" aria-hidden="true" focusable="false">
        <defs>
          <filter
            v-if="displacement"
            :id="filterId"
            x="0" y="0"
            :width="displacement.width" :height="displacement.height"
            filterUnits="userSpaceOnUse"
            color-interpolation-filters="sRGB"
          >
            <feGaussianBlur in="SourceGraphic" stdDeviation="1.2" result="softened" />
            <feImage :href="displacement.url" x="0" y="0" :width="displacement.width" :height="displacement.height" result="lens" />
            <feDisplacementMap in="softened" in2="lens" scale="36" xChannelSelector="R" yChannelSelector="G" />
          </filter>
        </defs>
      </svg>
      <div class="auth-layout__brand">
        <RayleaMark />
        <span>{{ t('app.brand') }}</span>
      </div>
      <div class="auth-layout__content">
        <section
          ref="surface"
          class="auth-layout__surface"
          :style="displacement ? { '--auth-refraction': `url(#${filterId})` } : undefined"
          @pointermove="moveHighlight"
          @pointerleave="resetHighlight"
        >
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
.auth-layout {
  --auth-glass-edge: color-mix(in srgb, var(--auth-panel-highlight) 58%, transparent);
  --auth-glass-highlight: color-mix(in srgb, var(--auth-panel-highlight) 18%, transparent);
  --auth-glass-surface: color-mix(in srgb, var(--auth-surface) 9%, transparent);
  --auth-glass-control: color-mix(in srgb, var(--auth-control) 58%, transparent);
  --auth-glass-control-hover: color-mix(in srgb, var(--auth-panel-highlight) 58%, transparent);
  --auth-glass-divider: color-mix(in srgb, var(--auth-brand-foreground) 14%, transparent);
  --auth-glass-link: var(--auth-brand-fill-pressed);
  --auth-glass-shadow: 0 24px 80px -24px color-mix(in srgb, var(--auth-brand-foreground) 24%, transparent), 0 4px 16px -8px color-mix(in srgb, var(--auth-brand-foreground) 12%, transparent);

  position: relative;
  isolation: isolate;
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
.auth-layout[data-auth-theme='dark'] {
  --auth-glass-link: var(--auth-brand-foreground);
  --auth-glass-edge: color-mix(in srgb, var(--auth-brand-foreground) 22%, transparent);
  --auth-glass-highlight: color-mix(in srgb, var(--auth-brand-foreground) 12%, transparent);
  --auth-glass-surface: color-mix(in srgb, var(--auth-surface) 18%, transparent);
  --auth-glass-control: color-mix(in srgb, var(--auth-control) 28%, transparent);
  --auth-glass-control-hover: color-mix(in srgb, var(--auth-control-hover) 42%, transparent);
  --auth-glass-shadow: var(--auth-panel-shadow);
}
.auth-layout__art {
  position: fixed;
  z-index: -1;
  inset: 0;
  background: url('@/assets/auth/celadon-glass.webp') center bottom no-repeat;
  background-size: max(100vw, 210vh) auto;
  pointer-events: none;
}
.auth-layout[data-auth-theme='dark'] .auth-layout__art { filter: brightness(.27) saturate(.7); }
.auth-layout__brand {
  position: absolute;
  top: 32px;
  left: 40px;
  display: flex;
  align-items: center;
  gap: 10px;
  color: var(--auth-brand-foreground);
  font-size: 17px;
  font-weight: 600;
  letter-spacing: -.02em;
  :deep(.raylea-mark) { width: 26px; height: 28px; }
}
.auth-layout__content { width: min(448px, 100%); }
.auth-layout[data-auth-theme='light'] .auth-layout__surface {
  --auth-text-muted: color-mix(in srgb, var(--auth-text) 85%, var(--auth-brand-foreground));
}
.auth-layout__surface {
  position: relative;
  width: 100%;
  color: var(--auth-text);
  border: 1px solid transparent;
  border-radius: 36px;
  background: var(--auth-surface);
  box-shadow:
    var(--auth-glass-shadow),
    inset 0 1px 0 var(--auth-glass-edge),
    inset 0 -1px 0 var(--auth-glass-edge),
    inset 1px 0 1px var(--auth-glass-highlight),
    inset -1px 0 1px var(--auth-glass-highlight);
  animation: auth-surface-enter 420ms var(--motion-easing) both;
}
@supports (backdrop-filter: blur(1px)) or (-webkit-backdrop-filter: blur(1px)) {
  .auth-layout__surface {
    background: linear-gradient(145deg, var(--auth-glass-highlight), transparent 42%, var(--auth-glass-highlight)), var(--auth-glass-surface);
    -webkit-backdrop-filter: blur(5px) saturate(112%);
    backdrop-filter: var(--auth-refraction, blur(5px)) saturate(112%);
  }
}
// Keep the CSS material in engines with differing SVG backdrop support.
@supports (-webkit-backdrop-filter: blur(1px)) or (-moz-appearance: none) {
  .auth-layout__surface { backdrop-filter: blur(5px) saturate(112%); }
}
.auth-layout__filters { position: absolute; width: 0; height: 0; pointer-events: none; }
.auth-layout__surface::before {
  content: '';
  position: absolute;
  z-index: 1;
  inset: -1px;
  padding: 1.5px;
  border-radius: inherit;
  background:
    radial-gradient(ellipse at var(--glass-light-x, 15%) var(--glass-light-y, 0%), var(--auth-panel-highlight), transparent 65%),
    linear-gradient(135deg, var(--auth-glass-edge), transparent 25%, transparent 65%, var(--auth-glass-edge));
  -webkit-mask: linear-gradient(var(--auth-panel-highlight) 0 0) content-box, linear-gradient(var(--auth-panel-highlight) 0 0);
  -webkit-mask-composite: xor;
  mask: linear-gradient(var(--auth-panel-highlight) 0 0) content-box, linear-gradient(var(--auth-panel-highlight) 0 0);
  mask-composite: exclude;
  pointer-events: none;
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
  &:hover { color: var(--auth-brand-foreground); background: var(--auth-glass-control); }
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
  .auth-layout__surface { border-radius: 28px; }
  .auth-layout__art { background-position: 58% bottom; }
  .auth-layout__toolbar { top: 12px; right: 12px; }
  .auth-layout__caption { margin-top: 20px; }
}
@media (max-height: #{bp.$shortViewport}) {
  .auth-layout { place-items: start center; padding-top: 80px; }
}
@media (prefers-reduced-motion: reduce) {
  .auth-layout__surface, .auth-layout__theme-toggle.app-button { animation: none; transition: none; }
}
@media (prefers-reduced-transparency: reduce) {
  .auth-layout__surface { background: var(--auth-surface); -webkit-backdrop-filter: none; backdrop-filter: none; }
}
@media (forced-colors: active) {
  .auth-layout__art, .auth-layout__surface::before { display: none; }
  .auth-layout__surface { border-color: CanvasText; background: Canvas; box-shadow: none; backdrop-filter: none; }
  .auth-layout__theme-toggle.app-button { border-color: CanvasText; }
  .auth-layout__theme-toggle.app-button:focus-visible { outline-color: Highlight; }
}
</style>
