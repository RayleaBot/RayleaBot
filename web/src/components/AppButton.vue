<script setup lang="ts">
import { LoaderCircleIcon } from '@lucide/vue'
import { Button, type ButtonVariants } from '@/components/ui/button'

// href renders the button as a link, e.g. to an external repository.
withDefaults(defineProps<{
  variant?: ButtonVariants['variant']; size?: ButtonVariants['size']; loading?: boolean; disabled?: boolean
  type?: 'button' | 'submit' | 'reset'; href?: string
}>(), { variant: 'outline', size: 'default', type: 'button' })
</script>

<template>
  <Button v-if="href" as="a" :href="href" :variant="variant" :size="size" class="app-button">
    <slot name="icon" />
    <slot />
  </Button>
  <Button v-else :variant="variant" :size="size" :type="type" :disabled="disabled || loading" :aria-busy="loading || undefined" class="app-button">
    <LoaderCircleIcon v-if="loading" class="app-spinner" aria-hidden="true" />
    <slot v-else name="icon" />
    <slot />
  </Button>
</template>

<style scoped lang="scss">
@use '@/styles/breakpoints.generated' as bp;
// Pill buttons: the orange primary carries dark ink and a lit top edge; outline and ghost stay quiet.
.app-button { transition-property: background-color, color, border-color, box-shadow, opacity; transition-duration: var(--motion-fast, 160ms); }
.app-button[data-variant=default] { background: var(--brand-fill); color: var(--on-brand); font-weight: 600; box-shadow: inset 0 1px 0 rgb(255 255 255 / 45%), 0 6px 16px -10px rgb(0 0 0 / 45%); }
.app-button[data-variant=default]:hover { background: var(--brand-fill-hover); }
.app-button[data-variant=default]:active { background: var(--brand-fill-pressed); }
.app-button[data-variant=outline] { border-color: var(--border); background: var(--surface-strong); color: var(--text); }
.app-button[data-variant=outline]:hover, .app-button[data-variant=outline][aria-expanded=true] { border-color: color-mix(in srgb, var(--border-strong) 45%, var(--border)); background: var(--surface-soft); }
.app-button[data-variant=ghost]:hover, .app-button[data-variant=ghost][aria-expanded=true] { background: color-mix(in srgb, var(--text) 6%, transparent); }
.app-button[data-variant=destructive] { color: var(--text-danger); }
.app-button[data-variant=link] { color: var(--brand-foreground); }
.app-button:disabled { cursor: not-allowed; box-shadow: none; }
@media (max-width: #{bp.$phone - 1px}), (pointer: coarse) { .app-button { min-width: 44px; min-height: 44px; } }
.app-spinner { animation: app-spin 800ms linear infinite; }
@keyframes app-spin { to { transform: rotate(360deg); } }
@media (prefers-reduced-motion: reduce), (forced-colors: active) { .app-spinner { animation: none; } .app-button { transition: none; } }
</style>
