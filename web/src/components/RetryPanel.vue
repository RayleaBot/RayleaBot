<script setup lang="ts">
import AppButton from '@/components/AppButton.vue'
import { CircleAlertIcon, RotateCwIcon } from '@lucide/vue'
import { computed, getCurrentInstance } from 'vue'
import type { Router } from 'vue-router'

import AppFallback from '@/components/fallback/AppFallback.vue'
import { resolveExceptionStatus, type ExceptionStatus } from '@/lib/exception-status'

const props = defineProps<{
  title: string
  description: string
  loading?: boolean
  retryLabel?: string
  status?: ExceptionStatus
  error?: unknown
  variant?: 'compact' | 'page'
}>()

defineEmits<{
  retry: []
}>()

const instance = getCurrentInstance()
const router = instance?.appContext.config.globalProperties.$router as Router | undefined
const isPageVariant = computed(() => props.variant === 'page')
const fallbackStatus = computed(() => props.status ?? resolveExceptionStatus(props.error))

function goHome() {
  void router?.push({ name: 'status' })
}
</script>

<template>
  <section class="retry-panel" role="alert">
    <AppFallback
      v-if="isPageVariant"
      :status="fallbackStatus"
      :title="title"
      :description="description"
      :retry-label="retryLabel"
      :retry-loading="loading"
      @home="goHome"
      @retry="$emit('retry')"
    />
    <div v-else class="retry-panel__inline">
      <CircleAlertIcon class="retry-panel__icon" :size="28" aria-hidden="true" />
      <div class="retry-panel__copy">
        <strong>{{ title }}</strong>
        <span>{{ description }}</span>
      </div>
      <AppButton :loading="loading" @click="$emit('retry')">
        <template #icon><RotateCwIcon :size="16" /></template>
        {{ retryLabel ?? '重试' }}
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
