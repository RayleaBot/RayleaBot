<script setup lang="ts">
import { computed, nextTick, provide, ref, watch } from 'vue'
import { DialogContent, DialogDescription, DialogOverlay, DialogPortal, DialogRoot, DialogTitle } from 'reka-ui'
import { motion } from 'motion-v'
import { XIcon } from '@lucide/vue'
import { useResizeObserver } from '@vueuse/core'
import { useOverlayMotion } from '@/motion/presets'
import AppButton from './AppButton.vue'
import { overlayLayerKey, useOverlayLayer } from './overlay-layer'

defineOptions({ inheritAttrs: false })
const props = withDefaults(defineProps<{
  open: boolean; title: string; description?: string; width?: number; busy?: boolean
  dismissible?: boolean; role?: 'dialog' | 'alertdialog'; initialFocus?: string; fallbackFocus?: string
  placement?: 'center' | 'left' | 'right' | 'bottom'
}>(), { width: 640, dismissible: true, role: 'dialog', placement: 'center' })
const emit = defineEmits<{ close: []; afterClose: [] }>()
const active = ref(props.open)
const { layer, activate, release } = useOverlayLayer()
provide(overlayLayerKey, layer)
const overlayMotion = useOverlayMotion()
const header = ref<HTMLElement | null>(null)
const body = ref<HTMLElement | null>(null)
const bodyContent = ref<HTMLElement | null>(null)
const footer = ref<HTMLElement | null>(null)
const contentHeight = ref<number>()
const motionState = computed(() => {
  const preset = overlayMotion.value
  if (props.placement === 'center') return { initial: preset.initial, animate: { ...preset.animate, height: contentHeight.value || 'auto' }, exit: preset.exit }
  if (props.placement === 'bottom') {
    const y = preset.transition.duration === 0 ? 0 : 24
    return { initial: { opacity: 0, y, scale: 1 }, animate: { opacity: 1, y: 0, scale: 1, height: contentHeight.value || 'auto' }, exit: { opacity: 0, y, scale: 1, transition: preset.exit.transition } }
  }
  const x = preset.transition.duration === 0 ? 0 : props.placement === 'left' ? -24 : 24
  return { initial: { opacity: 0, x, scale: 1 }, animate: { opacity: 1, x: 0, scale: 1 }, exit: { opacity: 0, x, scale: 1, transition: preset.exit.transition } }
})
function measureContent() {
  if (!header.value || !body.value || !bodyContent.value) return
  const style = getComputedStyle(body.value)
  // Animate height itself so CSS centering remains exact while async content changes.
  contentHeight.value = header.value.offsetHeight + bodyContent.value.offsetHeight
    + Number.parseFloat(style.paddingTop) + Number.parseFloat(style.paddingBottom)
    + (footer.value?.offsetHeight || 0)
}
useResizeObserver([header, bodyContent, footer], measureContent)
let trigger: HTMLElement | null = null
// Owners may clear the deleted object in afterClose before Reka restores focus.
let fallbackFocus: string | undefined
watch(() => props.open, (open) => {
  if (open) {
    fallbackFocus = props.fallbackFocus
    activate()
    if (!trigger || !active.value) trigger = document.activeElement instanceof HTMLElement ? document.activeElement : null
    active.value = true
  }
}, { immediate: true })
function requestClose() {
  if (props.open && !props.busy && props.dismissible) emit('close')
}
function finishExit() {
  if (props.open) return
  active.value = false
  release()
  emit('afterClose')
}
function motionComplete(definition: unknown) {
  if (definition && typeof definition === 'object' && 'opacity' in definition && definition.opacity === 0) finishExit()
}
function focusOnOpen(event: Event) {
  if (!props.initialFocus) return
  const target = document.querySelector<HTMLElement>(props.initialFocus)
  if (target) { event.preventDefault(); target.focus() }
}
function restoreFocus(event: Event) {
  event.preventDefault()
  void nextTick(() => {
    if (props.open) return
    const target = trigger?.isConnected && trigger !== document.body ? trigger
      : fallbackFocus ? document.querySelector<HTMLElement>(fallbackFocus) : null
    target?.focus()
  })
}
</script>
<template>
  <DialogRoot :open="active" @update:open="requestClose">
    <DialogPortal force-mount>
        <DialogOverlay v-if="active" force-mount as-child>
          <motion.div class="app-dialog-overlay" :style="{ zIndex: layer }" :initial="{ opacity: 0 }" :animate="{ opacity: open ? 1 : 0 }" :transition="{ ...overlayMotion.transition, duration: open ? overlayMotion.transition.duration : overlayMotion.exit.transition.duration }" />
        </DialogOverlay>
        <DialogContent
          v-if="active" force-mount as-child :role="role"
          @escape-key-down.prevent="requestClose"
          @pointer-down-outside.prevent="requestClose"
          @interact-outside.prevent
          @open-auto-focus="focusOnOpen" @close-auto-focus="restoreFocus"
        >
          <motion.section
            v-bind="$attrs" data-slot="app-dialog" class="app-dialog" :data-placement="placement" :style="{ width: placement === 'bottom' ? '100vw' : `${width}px`, ...(placement === 'left' || placement === 'right' ? { height: '100dvh' } : {}), zIndex: layer + 1 }"
            :initial="motionState.initial" :animate="open ? motionState.animate : motionState.exit" :transition="overlayMotion.transition"
            @animation-complete="motionComplete"
          >
            <header ref="header" class="app-dialog__header">
              <div>
                <DialogTitle class="app-dialog__title">{{ title }}</DialogTitle>
                <DialogDescription :class="description ? 'app-dialog__description' : 'sr-only'">{{ description || title }}</DialogDescription>
              </div>
              <AppButton v-if="dismissible" variant="ghost" size="icon" :disabled="busy" aria-label="关闭弹窗" @click="requestClose"><XIcon /></AppButton>
            </header>
            <div ref="body" class="app-dialog__body" :aria-busy="busy || undefined"><div ref="bodyContent" class="app-dialog__body-content"><slot /></div></div>
            <footer v-if="$slots.footer" ref="footer" class="app-dialog__footer"><slot name="footer" /></footer>
          </motion.section>
        </DialogContent>
    </DialogPortal>
  </DialogRoot>
</template>
<style scoped>
.app-dialog-overlay { position: fixed; inset: 0; z-index: 1200; background: color-mix(in srgb, var(--text) 42%, transparent); }
:global([data-theme=dark]) .app-dialog-overlay { background: color-mix(in srgb, var(--bg) 42%, transparent); }
.app-dialog { position: fixed; z-index: 1201; top: 50%; left: 50%; translate: -50% -50%; transform-origin: 50% 50%; display: flex; flex-direction: column; max-width: calc(100vw - 32px); max-height: calc(100dvh - 48px); overflow: hidden; background: var(--surface-strong); color: var(--text); border-radius: 16px; box-shadow: var(--shadow-floating); outline: none; }
.app-dialog__header { display: flex; align-items: flex-start; justify-content: space-between; gap: 16px; padding: 24px 24px 20px; }
.app-dialog__header > div { min-width: 0; }
.app-dialog__header { flex-shrink: 0; }
.app-dialog__title { margin: 3px 0 0; font-size: 19px; font-weight: 600; line-height: 1.4; overflow-wrap: anywhere; }
.app-dialog__description { margin: 8px 0 0; color: var(--muted); font-size: 13px; line-height: 1.6; }
.app-dialog__body { min-height: 0; overflow-y: auto; overscroll-behavior: contain; scrollbar-gutter: stable; padding: 0 24px 24px; }
.app-dialog__body-content { display: flow-root; }
.app-dialog__footer { flex-shrink: 0; padding: 16px 24px 20px; border-top: 1px solid var(--border); }
.app-dialog[data-placement=right], .app-dialog[data-placement=left] { top: 0; translate: none; max-height: 100dvh; max-width: calc(100vw - 24px); border-radius: 0; }
.app-dialog[data-placement=right] { left: auto; right: 0; }
.app-dialog[data-placement=left] { left: 0; }
.app-dialog[data-placement=bottom] { top: auto; bottom: 0; left: 0; translate: none; max-width: 100vw; max-height: calc(100dvh - 24px); border-radius: 16px 16px 0 0; }
.app-dialog:not([data-placement=center]) .app-dialog__body { flex: 1; }
@media (max-width: 639px) {
  .app-dialog { max-width: calc(100vw - 24px); max-height: calc(100dvh - 24px); border-radius: 14px; }
  .app-dialog__header { padding: 20px 16px 16px; }
  .app-dialog__body { padding: 0 16px 20px; }
  .app-dialog__footer { padding: 16px; }
}
@media (forced-colors: active) { .app-dialog { border: 1px solid CanvasText; } }
</style>
