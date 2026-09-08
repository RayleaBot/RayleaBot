<script setup lang="ts">
import { LoaderCircleIcon } from '@lucide/vue'
import { Button, type ButtonVariants } from '@/components/ui/button'

withDefaults(defineProps<{
  variant?: ButtonVariants['variant']; size?: ButtonVariants['size']; loading?: boolean; disabled?: boolean
  type?: 'button' | 'submit' | 'reset'
}>(), { variant: 'outline', size: 'default', type: 'button' })
</script>

<template>
  <Button :variant="variant" :size="size" :type="type" :disabled="disabled || loading" :aria-busy="loading || undefined" class="app-button">
    <LoaderCircleIcon v-if="loading" class="app-spinner" aria-hidden="true" />
    <slot v-else name="icon" />
    <slot />
  </Button>
</template>

<style scoped>
.app-button { transition-property: background-color, color, border-color, opacity; transition-duration: var(--motion-fast, 160ms); }
.app-button[data-variant=default] { background: var(--brand-fill); color: var(--on-brand); }
.app-button[data-variant=default]:hover { background: var(--brand-fill-hover); }
.app-button[data-variant=default]:active { background: var(--brand-fill-pressed); }
.app-button[data-variant=destructive] { color: var(--text-danger); }
.app-button[data-variant=link] { color: var(--brand-foreground); }
.app-button:disabled { cursor: not-allowed; }
@media (max-width: 639px), (pointer: coarse) { .app-button { min-width: 44px; min-height: 44px; } }
.app-spinner { animation: app-spin 800ms linear infinite; }
@keyframes app-spin { to { transform: rotate(360deg); } }
@media (prefers-reduced-motion: reduce), (forced-colors: active) { .app-spinner { animation: none; } .app-button { transition: none; } }
</style>
