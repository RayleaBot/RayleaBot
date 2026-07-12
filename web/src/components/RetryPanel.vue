<script setup lang="ts">
import { computed, getCurrentInstance } from 'vue'
import type { Router } from 'vue-router'

import VbenFallback from '@/components/fallback/VbenFallback.vue'
import { t } from '@/i18n'
import { resolveExceptionStatusFromText, type ExceptionStatus } from '@/lib/exception-status'

const props = defineProps<{
  title: string
  description: string
  loading?: boolean
  retryLabel?: string
  status?: ExceptionStatus
  variant?: 'compact' | 'page'
}>()

defineEmits<{
  retry: []
}>()

const instance = getCurrentInstance()
const router = instance?.appContext.config.globalProperties.$router as Router | undefined
const isPageVariant = computed(() => props.variant === 'page')
const fallbackStatus = computed(() => props.status ?? resolveExceptionStatusFromText(props.description))
const usesNativeFallbackCopy = computed(() => {
  const genericCopy = new Set([
    t('errors.common.actionFailed'),
    t('errors.common.loadFailed'),
    t('errors.permission.denied'),
    t('errors.platform.notFound'),
  ])

  return genericCopy.has(props.title) || genericCopy.has(props.description)
})
const fallbackTitle = computed(() => (usesNativeFallbackCopy.value ? undefined : props.title))
const fallbackDescription = computed(() => (usesNativeFallbackCopy.value ? undefined : props.description))

function goHome() {
  void router?.push({ name: 'status' })
}
</script>

<template>
  <section class="retry-panel" role="alert">
    <VbenFallback
      v-if="isPageVariant"
      :status="fallbackStatus"
      :title="fallbackTitle"
      :description="fallbackDescription"
      :retry-label="retryLabel"
      :retry-loading="loading"
      @home="goHome"
      @retry="$emit('retry')"
    />
    <div v-else class="retry-panel__inline">
      <div class="retry-panel__copy">
        <strong>{{ title }}</strong>
        <span>{{ description }}</span>
      </div>
      <a-button type="primary" :loading="loading" @click="$emit('retry')">
        {{ retryLabel ?? '重试' }}
      </a-button>
    </div>
  </section>
</template>

<style scoped lang="scss">
.retry-panel__inline {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  padding: 16px;
  border: 1px solid color-mix(in srgb, var(--danger) 28%, var(--border));
  border-radius: var(--radius-md);
  background: var(--surface-danger);
}

.retry-panel__copy {
  display: grid;
  gap: 4px;
  min-width: 0;
}

.retry-panel__copy strong {
  color: var(--text);
  font-size: 14px;
}

.retry-panel__copy span {
  color: var(--muted);
  font-size: 13px;
  overflow-wrap: anywhere;
}

@media (max-width: 639px) {
  .retry-panel__inline {
    align-items: stretch;
    flex-direction: column;
  }
}
</style>
