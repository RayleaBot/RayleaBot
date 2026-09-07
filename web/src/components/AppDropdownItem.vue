<script setup lang="ts">
import { DropdownMenuItem } from 'reka-ui'
defineProps<{ disabled?: boolean; danger?: boolean }>()
defineEmits<{ select: [event: Event] }>()
</script>
<template>
  <DropdownMenuItem :disabled="disabled" class="app-menu-item" :data-danger="danger || undefined" @select="$emit('select', $event)"><slot /></DropdownMenuItem>
</template>
<style>
.app-menu-item { display: flex; align-items: center; gap: 10px; min-height: 38px; padding: 8px 10px; border-radius: 7px; font-size: 14px; cursor: default; outline: none; user-select: none; }
.app-menu-item > svg { flex-shrink: 0; width: 17px; height: 17px; }
.app-menu-item[data-highlighted] { background: var(--surface-accent); color: var(--text); }
.app-menu-item[data-disabled] { opacity: .5; pointer-events: none; }
.app-menu-item[data-danger] { color: var(--text-danger); }
.app-menu-item[data-danger][data-highlighted] { background: var(--surface-danger); }
@media (pointer: coarse) { .app-menu-item { min-height: 44px; } }
@media (forced-colors: active) { .app-menu-item[data-highlighted] { outline: 2px solid Highlight; outline-offset: -2px; } }
</style>
