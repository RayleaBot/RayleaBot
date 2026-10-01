<script setup lang="ts">
import { t } from '@/i18n'
import { onBeforeUnmount } from 'vue'
import { ToastClose, ToastDescription, ToastProvider, ToastRoot, ToastViewport } from 'reka-ui'
import { CircleCheckIcon, CircleAlertIcon, InfoIcon, TriangleAlertIcon, XIcon } from '@lucide/vue'
import { dismissToast, toasts } from './toast-state'
import { motionDuration, prefersReducedMotion } from '@/motion/runtime'
const icons = { success: CircleCheckIcon, error: CircleAlertIcon, info: InfoIcon, warning: TriangleAlertIcon }
const exitTimers = new Set<ReturnType<typeof setTimeout>>()
function closeToast(id: number) {
  const timer = setTimeout(() => { dismissToast(id); exitTimers.delete(timer) }, prefersReducedMotion() ? 0 : motionDuration.control)
  exitTimers.add(timer)
}
onBeforeUnmount(() => exitTimers.forEach(clearTimeout))
</script>
<template>
  <ToastProvider :duration="4500" swipe-direction="right">
    <ToastRoot v-for="toast in toasts" :key="toast.id" class="app-toast" :data-level="toast.level" :duration="toast.level === 'error' ? 7000 : 4500" type="foreground" @update:open="!$event && closeToast(toast.id)">
      <div class="app-toast__frame">
        <div class="app-toast__card">
          <component :is="icons[toast.level]" class="app-toast__icon" aria-hidden="true" />
          <ToastDescription class="app-toast__description">{{ toast.content }}</ToastDescription>
          <ToastClose class="app-toast__close" :aria-label="t('ui.closeToast')"><XIcon :size="14" /></ToastClose>
        </div>
      </div>
    </ToastRoot>
    <ToastViewport class="app-toast-viewport" :label="t('ui.notifications', { hotkey: '{hotkey}' })" />
  </ToastProvider>
</template>
<style lang="scss">
// The stack lets clicks through its gaps; only the cards take the pointer.
.app-toast-viewport { position: fixed; z-index: 1600; right: max(20px, env(safe-area-inset-right)); top: max(88px, env(safe-area-inset-top)); display: flex; flex-direction: column; width: min(380px, calc(100vw - 32px)); margin: 0; padding: 0; list-style: none; outline: none; pointer-events: none; }

// Each toast is a grid row that closes to nothing as it leaves, so the toasts below slide up instead of jumping.
// The row has no size of its own; the spacing is the card's margin inside it.
.app-toast { --toast-tone: var(--text-info); --toast-tone-soft: var(--surface-info); display: grid; grid-template-rows: 1fr; outline: none; }
.app-toast[data-level=success] { --toast-tone: var(--text-success); --toast-tone-soft: var(--surface-success); }
.app-toast[data-level=warning] { --toast-tone: var(--text-warning); --toast-tone-soft: var(--surface-warning); }
.app-toast[data-level=error] { --toast-tone: var(--text-danger); --toast-tone-soft: var(--surface-danger); }
.app-toast__frame { display: flow-root; min-height: 0; }

// A raised card with a hairline edge separates from both the white page and the grey boxes it floats over.
.app-toast__card { display: flex; align-items: flex-start; gap: 10px; margin-bottom: 8px; padding: 12px 10px 12px 12px; border: 1px solid var(--border); border-radius: var(--radius-lg); background: var(--surface-raised); color: var(--text); box-shadow: var(--shadow-floating); pointer-events: auto; }
.app-toast:focus-visible .app-toast__card { outline: 2px solid var(--focus); outline-offset: 2px; }

// The status shape is filled with the soft tone and only the mark is drawn, as on the status tags.
.app-toast__icon { flex-shrink: 0; width: 24px; height: 24px; color: var(--toast-tone); }
.app-toast__icon :is(circle, path:first-child) { fill: var(--toast-tone-soft); stroke: none; }
.app-toast__description { flex: 1; min-width: 0; padding-block: 1.5px; font-size: 14px; line-height: 1.5; overflow-wrap: anywhere; text-wrap: pretty; }

// The close button shows while the toast is pointed at or focused; the timer pauses then as well.
.app-toast__close { display: grid; place-items: center; flex-shrink: 0; width: 24px; height: 24px; border-radius: 50%; color: var(--muted); opacity: 0; transition: opacity var(--motion-fast, 160ms) var(--motion-easing), background-color var(--motion-fast, 160ms) var(--motion-easing), color var(--motion-fast, 160ms) var(--motion-easing); }
.app-toast:hover .app-toast__close, .app-toast:focus-within .app-toast__close { opacity: 1; }
.app-toast__close:hover { background: var(--nav-hover); color: var(--text); }
.app-toast__close:focus-visible { outline: 2px solid var(--focus); outline-offset: 1px; }

.app-toast[data-state=open] { animation: app-toast-enter var(--motion-content, 200ms) var(--motion-easing); }
// A leaving toast fades out first while its row closes with an ease-in-out, so the toasts below settle into place.
.app-toast[data-state=closed] { animation: app-toast-leave var(--motion-fast, 160ms) cubic-bezier(0, 0, .2, 1) forwards, app-toast-collapse var(--motion-fast, 160ms) cubic-bezier(.4, 0, .2, 1) forwards; }
.app-toast[data-swipe=move] { transform: translateX(var(--reka-toast-swipe-move-x)); }
.app-toast[data-swipe=cancel] { transform: translateX(0); transition: transform var(--motion-fast, 160ms) var(--motion-easing); }
.app-toast[data-swipe=end] { animation: app-toast-swipe-out var(--motion-fast, 160ms) ease-out forwards, app-toast-collapse var(--motion-fast, 160ms) cubic-bezier(.4, 0, .2, 1) forwards; }
@keyframes app-toast-enter { from { opacity: 0; transform: translateX(16px) scale(.98); } }
@keyframes app-toast-leave { to { opacity: 0; transform: translateX(12px); } }
@keyframes app-toast-collapse { to { grid-template-rows: 0fr; } }
@keyframes app-toast-swipe-out { from { transform: translateX(var(--reka-toast-swipe-end-x)); } to { opacity: 0; transform: translateX(calc(100% + 20px)); } }

@media (pointer: coarse) { .app-toast__close { min-width: 44px; min-height: 44px; opacity: 1; } }
@media (prefers-reduced-motion: reduce), (forced-colors: active) { .app-toast[data-state], .app-toast[data-swipe], .app-toast__close { animation: none; transition: none; } }
@media (forced-colors: active) { .app-toast__card { border-color: CanvasText; box-shadow: none; } .app-toast__close { opacity: 1; } }
</style>
