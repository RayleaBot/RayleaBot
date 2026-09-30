<script setup lang="ts">
import { t } from '@/i18n'
import AppButton from '@/components/AppButton.vue'
import { CircleAlertIcon, RotateCwIcon } from '@lucide/vue'

defineProps<{
  title: string
  description: string
  loading?: boolean
}>()

defineEmits<{
  retry: []
}>()
</script>

<template>
  <section class="retry-panel" role="alert">
    <div class="retry-panel__inline">
      <CircleAlertIcon class="retry-panel__icon" :size="28" aria-hidden="true" />
      <div class="retry-panel__copy">
        <strong>{{ title }}</strong>
        <!-- A generic failure often has no more to say than the title; the same sentence is not shown twice. -->
        <span v-if="description && description !== title">{{ description }}</span>
      </div>
      <AppButton :loading="loading" @click="$emit('retry')">
        <template #icon><RotateCwIcon :size="16" /></template>
        {{ t('ui.retry') }}
      </AppButton>
    </div>
  </section>
</template>
<style scoped lang="scss">
.retry-panel__inline {
  display: grid;
  justify-items: center;
  align-content: center;
  gap: 18px;
  min-height: 240px;
  padding: 32px 20px;
  text-align: center;
}

.retry-panel__icon { color: var(--muted); }

.retry-panel__copy {
  display: grid;
  gap: 4px;
  min-width: 0;
  max-width: 52ch;
}

.retry-panel__copy strong {
  color: var(--text);
  font-size: 16px;
  font-weight: 600;
  overflow-wrap: anywhere;
}

.retry-panel__copy span {
  color: var(--muted);
  font-size: 14px;
  overflow-wrap: anywhere;
}

</style>
