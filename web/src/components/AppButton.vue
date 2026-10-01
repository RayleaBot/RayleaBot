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
// Pill buttons: the blue primary is the only filled action. Secondary buttons take the opposite lightness of
// their container (--control-fill) and float on a soft shadow; ghost buttons stay flat.
.app-button { transition-property: background-color, color, border-color, box-shadow, opacity; transition-duration: var(--motion-fast, 160ms); }
.app-button[data-variant=default] { background: var(--brand-fill); color: var(--on-brand); font-weight: 700; box-shadow: var(--shadow-xs); }
.app-button[data-variant=default]:hover { background: var(--brand-fill-hover); }
.app-button[data-variant=default]:active { background: var(--brand-fill-pressed); }
.app-button[data-variant=outline] { border-color: transparent; background: var(--control-fill); color: var(--text); box-shadow: var(--shadow-xs); }
.app-button[data-variant=outline]:hover, .app-button[data-variant=outline][aria-expanded=true] { background: var(--control-fill-hover); }
.app-button[data-variant=ghost]:hover, .app-button[data-variant=ghost][aria-expanded=true] { background: var(--nav-hover); }
/* Destructive buttons tint their container's control fill instead of letting the surface show through. */
.app-button[data-variant=destructive] { --destructive-tint: 10%; color: var(--text-danger); background: color-mix(in srgb, var(--text-danger) var(--destructive-tint), var(--control-fill)); }
.app-button[data-variant=destructive]:hover { --destructive-tint: 16%; }
:global([data-theme=dark]) .app-button[data-variant=destructive] { --destructive-tint: 18%; }
:global([data-theme=dark]) .app-button[data-variant=destructive]:hover { --destructive-tint: 26%; }
.app-button[data-variant=link] { color: var(--brand-foreground); }
.app-button:disabled { cursor: not-allowed; box-shadow: none; }
@media (pointer: coarse) { .app-button { min-width: 44px; min-height: 44px; } }
.app-spinner { animation: app-spin 800ms linear infinite; }
@keyframes app-spin { to { transform: rotate(360deg); } }
@media (prefers-reduced-motion: reduce), (forced-colors: active) { .app-spinner { animation: none; } .app-button { transition: none; } }
</style>
