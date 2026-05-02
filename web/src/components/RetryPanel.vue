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
const isPageVariant = computed(() => props.variant !== 'compact')
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
    <a-result v-else status="warning" :title="title" :sub-title="description">
      <template #extra>
        <a-button type="primary" :loading="loading" @click="$emit('retry')">
          {{ retryLabel ?? '重试' }}
        </a-button>
      </template>
    </a-result>
  </section>
</template>
