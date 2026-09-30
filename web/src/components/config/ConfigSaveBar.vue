<script setup lang="ts">
import { CircleAlertIcon, CircleCheckIcon, SaveIcon } from '@lucide/vue'
import AppButton from '@/components/AppButton.vue'
import { t } from '@/i18n'

// The on-demand save bar of a sectioned settings form. It sticks to the bottom of the workspace; the draft
// state and the save result are announced from the same place, left of the single save action.
defineProps<{
  testIdPrefix: string
  dirty: boolean
  savedLabel?: string
  canSave: boolean
  saving: boolean
}>()
defineEmits<{ save: [] }>()
</script>

<template>
  <footer class="config-save-bar">
    <div class="config-save-bar__status" aria-live="polite">
      <span v-if="dirty" class="config-save-bar__pill config-save-bar__pill--dirty" :data-testid="`${testIdPrefix}-unsaved-status`">
        <CircleAlertIcon aria-hidden="true" />{{ t('config.unsaved') }}
      </span>
      <span v-else-if="savedLabel" class="config-save-bar__pill config-save-bar__pill--saved" :data-testid="`${testIdPrefix}-save-status`">
        <CircleCheckIcon aria-hidden="true" />{{ savedLabel }}
      </span>
    </div>
    <AppButton variant="default" :disabled="!canSave" :loading="saving" :data-testid="`${testIdPrefix}-save`" @click="$emit('save')">
      <template #icon><SaveIcon /></template>
      {{ t('config.save') }}
    </AppButton>
  </footer>
</template>

<style scoped lang="scss">
.config-save-bar {
  position: sticky;
  bottom: 0;
  z-index: 2;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  min-height: 64px;
  padding: 12px 20px;
  border-top: 1px solid var(--border);
  border-radius: 0 0 var(--app-card-radius) var(--app-card-radius);
  background: var(--surface);
}

.config-save-bar .app-button {
  flex: 0 0 auto;
}

.config-save-bar__status {
  display: flex;
  align-items: center;
  min-width: 0;
  min-height: 28px;
}

.config-save-bar__pill {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  min-height: 28px;
  padding: 4px 10px;
  border-radius: var(--radius-sm);
  font-size: 13px;
  font-weight: 500;
  line-height: 1;

  svg {
    width: 14px;
    height: 14px;
  }
}

.config-save-bar__pill--dirty {
  color: var(--text-attention);
  background: var(--surface-attention);
  border: 1px solid var(--border-attention);
}

.config-save-bar__pill--saved {
  color: var(--success);
  background: var(--surface-success);
  border: 1px solid color-mix(in srgb, var(--success) 32%, var(--border));
}
</style>
