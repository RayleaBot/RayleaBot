<script setup lang="ts">
import { t } from '@/i18n'

defineProps<{ text: string | null; inline?: boolean }>()
</script>

<template>
  <div v-if="text" class="rate-limit-preview" :class="{ 'rate-limit-preview--inline': inline }">
    <span class="rate-limit-preview__label">{{ t('config.hints.rateLimitPreview') }}</span>
    <strong class="rate-limit-preview__value">{{ text }}</strong>
  </div>
</template>

<style scoped>
.rate-limit-preview {
  display: grid;
  align-content: center;
  gap: 3px;
  min-height: 36px;
  padding: 7px 10px;
  border-radius: var(--radius-md);
  background: var(--surface-accent);
  border: 1px solid var(--border-accent);
}
.rate-limit-preview__label { font-size: 13px; letter-spacing: 0; color: var(--accent); }
.rate-limit-preview__value { color: var(--text); font-size: 0.9rem; line-height: 1.4; }
.rate-limit-preview--inline {
  display: inline-flex;
  align-items: baseline;
  gap: 6px;
  min-height: auto;
  padding: 6px 10px;
  background: var(--surface-soft);
  border-color: var(--border);
  color: var(--muted);
  font-size: 13px;
  line-height: 1.4;
  width: max-content;
  max-width: 100%;
}
.rate-limit-preview--inline .rate-limit-preview__label { color: inherit; letter-spacing: 0.04em; }
.rate-limit-preview--inline .rate-limit-preview__value { font-size: inherit; font-weight: 600; font-family: var(--font-mono); }
</style>
