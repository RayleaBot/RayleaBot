<script setup lang="ts">
import { computed, inject } from 'vue'
import { ContextMenuContent, ContextMenuPortal, ContextMenuRoot, ContextMenuTrigger, DropdownMenuContent, DropdownMenuPortal, DropdownMenuRoot, DropdownMenuTrigger } from 'reka-ui'
import { overlayLayerKey } from './overlay-layer'
defineOptions({ inheritAttrs: false })
const props = withDefaults(defineProps<{ context?: boolean; side?: 'top' | 'right' | 'bottom' | 'left'; align?: 'start' | 'center' | 'end'; label?: string }>(), { side: 'bottom', align: 'end' })
const layer = inject(overlayLayerKey, undefined)
const parts = computed(() => props.context
  ? { root: ContextMenuRoot, trigger: ContextMenuTrigger, portal: ContextMenuPortal, content: ContextMenuContent }
  : { root: DropdownMenuRoot, trigger: DropdownMenuTrigger, portal: DropdownMenuPortal, content: DropdownMenuContent })
</script>
<template>
  <component :is="parts.root">
    <component :is="parts.trigger" as-child><slot /></component>
    <component :is="parts.portal">
      <component :is="parts.content" v-bind="$attrs" class="app-menu-popup" :side="side" :align="align" :side-offset="6" :collision-padding="8" :aria-label="label" :style="{ zIndex: (layer || 1100) + 5 }">
        <slot name="content" />
      </component>
    </component>
  </component>
</template>
<style>
.app-menu-popup { min-width: 180px; max-width: min(360px, calc(100vw - 16px)); max-height: var(--reka-popper-available-height); overflow-y: auto; padding: 6px; border: 1px solid var(--border); border-radius: 12px; background: var(--surface-strong); color: var(--text); box-shadow: var(--shadow-floating); outline: none; transform-origin: var(--reka-dropdown-menu-content-transform-origin, var(--reka-context-menu-content-transform-origin)); }
.app-menu-popup[data-state=open] { animation: app-popup-enter var(--motion-fast, 160ms) cubic-bezier(.16,1,.3,1); }
.app-menu-popup[data-state=closed] { animation: app-popup-exit var(--motion-fast, 160ms) ease; }
.app-menu-separator { height: 1px; margin: 5px 2px; background: var(--border); }
.app-menu-label { padding: 8px 10px 4px; color: var(--muted); font-size: 12px; }
@keyframes app-popup-enter { from { opacity: 0; transform: scale(.97); } to { opacity: 1; transform: scale(1); } }
@keyframes app-popup-exit { to { opacity: 0; transform: scale(.97); } }
@media (prefers-reduced-motion: reduce), (forced-colors: active) { .app-menu-popup[data-state] { animation: none; } }
@media (forced-colors: active) { .app-menu-popup { border: 1px solid CanvasText; } }
</style>
