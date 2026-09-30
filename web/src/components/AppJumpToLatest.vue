<script setup lang="ts">
import { ChevronDownIcon } from '@lucide/vue'
import AppButton from './AppButton.vue'
import AppTooltip from './AppTooltip.vue'

// Floats over a live list whose following is paused and returns it to the newest entries.
// count: entries that arrived since the pause; pendingLabel names them in the tooltip.
defineProps<{ label: string; pendingLabel?: string; count?: number }>()
defineEmits<{ jump: [] }>()
</script>

<template>
  <div class="app-jump-latest">
    <AppTooltip :title="count && pendingLabel ? pendingLabel : label">
      <AppButton variant="default" size="icon" class="app-jump-latest__button" :aria-label="label" @click="$emit('jump')">
        <template #icon>
          <ChevronDownIcon />
        </template>
        <span v-if="count" class="app-jump-latest__count">{{ count }}</span>
      </AppButton>
    </AppTooltip>
  </div>
</template>

<style scoped>
.app-jump-latest { position: absolute; right: 18px; bottom: 18px; z-index: 2; }
.app-jump-latest__button { box-shadow: var(--shadow-floating); }
/* The count of new entries is information, not an error: a white badge on the blue button. */
.app-jump-latest__count { position: absolute; top: -8px; right: -8px; min-width: 22px; padding: 2px 5px; border-radius: 12px; background: var(--surface-raised); color: var(--brand-foreground); box-shadow: var(--shadow-xs); font-size: 12px; font-weight: 600; font-variant-numeric: tabular-nums; }
</style>
