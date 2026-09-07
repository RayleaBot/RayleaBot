<script setup lang="ts">
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
      <component :is="icons[toast.level]" class="app-toast__icon" aria-hidden="true" />
      <ToastDescription class="app-toast__description">{{ toast.content }}</ToastDescription>
      <ToastClose class="app-toast__close" aria-label="关闭提示"><XIcon :size="16" /></ToastClose>
    </ToastRoot>
    <ToastViewport class="app-toast-viewport" label="通知 ({hotkey})" />
  </ToastProvider>
</template>
<style>
.app-toast-viewport { position: fixed; z-index: 1600; right: max(20px, env(safe-area-inset-right)); top: max(20px, env(safe-area-inset-top)); display: grid; gap: 10px; width: min(380px, calc(100vw - 32px)); margin: 0; padding: 0; list-style: none; outline: none; }
.app-toast { display: flex; align-items: start; gap: 10px; padding: 14px; border: 1px solid var(--border); border-radius: 12px; background: var(--surface-strong); color: var(--text); box-shadow: var(--shadow-floating); }
.app-toast__description { flex: 1; min-width: 0; font-size: 14px; line-height: 1.6; overflow-wrap: anywhere; }
.app-toast__icon { width: 20px; height: 20px; margin-top: 2px; flex-shrink: 0; color: var(--text-accent); }
.app-toast[data-level=success] .app-toast__icon { color: var(--text-success); }
.app-toast[data-level=error] .app-toast__icon { color: var(--text-danger); }
.app-toast[data-level=warning] .app-toast__icon { color: var(--text-warning); }
.app-toast__close { display: grid; place-items: center; flex-shrink: 0; width: 28px; height: 28px; margin: -2px -4px -2px 0; border-radius: 6px; color: var(--muted); }
.app-toast__close:hover { background: var(--surface-soft); color: var(--text); }
.app-toast[data-state=open] { animation: app-popup-enter var(--motion-content, 200ms) cubic-bezier(.16,1,.3,1); }
.app-toast[data-state=closed] { animation: app-popup-exit var(--motion-fast, 160ms) ease; }
.app-toast[data-swipe=move] { transform: translateX(var(--reka-toast-swipe-move-x)); }
.app-toast[data-swipe=cancel] { transform: translateX(0); transition: transform var(--motion-fast, 160ms); }
@media (pointer: coarse) { .app-toast__close { min-width: 44px; min-height: 44px; } }
@media (prefers-reduced-motion: reduce), (forced-colors: active) { .app-toast[data-state], .app-toast[data-swipe] { animation: none; transition: none; } }
</style>
