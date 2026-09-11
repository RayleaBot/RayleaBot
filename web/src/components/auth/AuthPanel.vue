<script setup lang="ts">
import { ref } from 'vue'

import RayleaMark from '@/components/brand/RayleaMark.vue'

withDefaults(defineProps<{
  title: string
  subtitle: string
  headingId?: string
}>(), {
  headingId: 'auth-panel-title',
})

const heading = ref<HTMLHeadingElement | null>(null)
defineExpose({ focus: () => heading.value?.focus() })
</script>

<template>
  <section class="auth-panel" :aria-labelledby="headingId">
    <header class="auth-panel__header">
      <div class="auth-panel__mark"><RayleaMark /></div>
      <h1 :id="headingId" ref="heading" class="auth-panel__title" tabindex="-1">{{ title }}</h1>
      <p class="auth-panel__subtitle">{{ subtitle }}</p>
    </header>
    <slot />
    <footer v-if="$slots.footer" class="auth-panel__footer"><slot name="footer" /></footer>
  </section>
</template>

<style scoped lang="scss">
@use '@/styles/breakpoints.generated' as bp;
.auth-panel { width: 100%; padding: 48px 40px 32px; color: var(--auth-text); }
.auth-panel__header { text-align: center; }
.auth-panel__mark {
  display: grid;
  place-items: center;
  width: 64px;
  height: 64px;
  margin: 0 auto 24px;
  color: var(--auth-brand-foreground);
  border: 1px solid var(--auth-glass-edge);
  backdrop-filter: blur(3px);
  border-radius: 20px;
  background: linear-gradient(145deg, var(--auth-glass-highlight), transparent);
  box-shadow: 0 8px 20px color-mix(in srgb, var(--auth-brand-fill) 12%, transparent), inset 0 1px 0 var(--auth-glass-edge);
  :deep(.raylea-mark) { width: 36px; height: 38px; }
}
.auth-panel__title {
  margin: 0;
  color: var(--auth-text);
  font-size: 28px;
  font-weight: 600;
  line-height: 1.3;
  letter-spacing: -.025em;
  text-wrap: balance;
  &:focus { outline: none; }
  &:focus-visible { outline: 2px solid var(--auth-focus); outline-offset: var(--focus-outline-offset); border-radius: 4px; }
}
.auth-panel__subtitle {
  max-width: 100%;
  margin: 12px auto 0;
  color: var(--auth-text-muted);
  font-size: 14px;
  line-height: 1.75;
  text-wrap: pretty;
}
.auth-panel__footer {
  display: grid;
  justify-items: center;
  margin-top: 24px;
  padding-top: 16px;
  border-top: 1px solid var(--auth-glass-divider);
  color: var(--auth-text-muted);
  font-size: 13px;
  line-height: 1.7;
  text-align: center;
  :deep(p) { margin: 0; }
}
.auth-panel :deep(.auth-panel__text-action) {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  min-height: 44px;
  padding: 8px 14px;
  color: var(--auth-glass-link);
  border: 0;
  border-radius: 12px;
  background: transparent;
  font: inherit;
  font-size: 13px;
  cursor: pointer;
  transition: background-color 160ms var(--motion-easing), color 160ms var(--motion-easing);
  &:hover { background: var(--auth-glass-control); }
  &:focus-visible { outline: 2px solid var(--auth-focus); outline-offset: var(--focus-outline-offset); }
  &:disabled { opacity: .55; cursor: wait; }
}
@media (max-width: #{bp.$compactAuth}) {
  .auth-panel { padding: 44px 24px 24px; }
  .auth-panel__mark { margin-bottom: 20px; }
  .auth-panel__title { font-size: 26px; }
}
@media (prefers-reduced-motion: reduce) {
  .auth-panel :deep(.auth-panel__text-action) { transition: none; }
}
@media (forced-colors: active) {
  .auth-panel__mark { border-color: CanvasText; box-shadow: none; }
  .auth-panel__footer { border-color: CanvasText; }
}
</style>
