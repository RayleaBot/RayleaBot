<script setup lang="ts">
import { t } from '@/i18n'

defineProps<{ text: string | null }>()
</script>

<template>
  <!-- A line of text that reads the value back; it is neither a control nor a box. -->
  <p v-if="text" class="rate-limit-preview">
    <span class="rate-limit-preview__label">{{ t('config.hints.rateLimitPreview') }}</span>
    <strong class="rate-limit-preview__value">{{ text }}</strong>
  </p>
</template>

<style scoped>
.rate-limit-preview {
  display: inline-flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 6px;
  max-width: 100%;
  margin: 0;
  color: var(--muted);
  font-size: 13px;
  line-height: 1.5;
}
.rate-limit-preview__value { color: var(--text); font-weight: 600; }
</style>
