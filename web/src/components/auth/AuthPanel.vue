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
.auth-panel { width: 100%; padding: 48px 40px 32px; color: var(--auth-text); }
.auth-panel__header { text-align: center; }
.auth-panel__mark {
  display: grid;
  place-items: center;
  width: 112px;
  height: 112px;
  margin: 0 auto 24px;
  color: var(--auth-text);
  border: 1px solid transparent;
  border-radius: 28px;
  background: var(--auth-control);
  box-shadow: var(--shadow-xs);
  :deep(.raylea-mark) { width: 96px; height: 96px; }
}
.auth-panel__title {
  margin: 0;
  color: var(--auth-text);
  font-size: 28px;
  font-weight: 700;
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
  border-top: 1px solid var(--auth-border);
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
  color: var(--auth-brand-foreground);
  border: 0;
  border-radius: 12px;
  background: transparent;
  font: inherit;
  font-size: 13px;
  cursor: pointer;
  transition: background-color 160ms var(--motion-easing), color 160ms var(--motion-easing);
  &:hover { background: var(--auth-control); }
  &:focus-visible { outline: 2px solid var(--auth-focus); outline-offset: var(--focus-outline-offset); }
  &:disabled { opacity: .55; cursor: wait; }
}
@media (prefers-reduced-motion: reduce) {
  .auth-panel :deep(.auth-panel__text-action) { transition: none; }
}
@media (forced-colors: active) {
  .auth-panel__mark { border-color: CanvasText; box-shadow: none; }
  .auth-panel__footer { border-color: CanvasText; }
}
</style>
