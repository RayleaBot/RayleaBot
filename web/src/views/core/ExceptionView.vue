<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import VbenFallback from '@/components/fallback/VbenFallback.vue'
import { t } from '@/i18n'
import type { ExceptionStatus } from '@/lib/exception-status'
import { useAppAvailabilityStore } from '@/stores/app-availability'
import { useSessionStore } from '@/stores/session'

const props = defineProps<{
  status?: ExceptionStatus
}>()

const route = useRoute()
const router = useRouter()
const availabilityStore = useAppAvailabilityStore()
const sessionStore = useSessionStore()
const retrying = ref(false)
const status = computed<ExceptionStatus>(() => props.status ?? route.meta.exceptionStatus ?? '500')

const retryLabel = computed(() => (
  status.value === 'offline'
    ? t('fallback.actions.recheck')
    : t('fallback.actions.retry')
))

function resolveHomeRoute() {
  if (sessionStore.requiresSetup) {
    return { name: 'setup' }
  }

  return sessionStore.isAuthenticated ? { name: 'status' } : { name: 'login' }
}

async function goHome() {
  availabilityStore.clearReturnPath()
  await router.push(resolveHomeRoute())
}

async function retry() {
  if (status.value !== 'offline') {
    window.location.reload()
    return
  }

  retrying.value = true
  try {
    await sessionStore.bootstrap(true)
    availabilityStore.markOnline()

    const returnPath = availabilityStore.consumeReturnPath()
    if (sessionStore.requiresSetup) {
      await router.replace({ name: 'setup' })
    } else if (sessionStore.isAuthenticated) {
      await router.replace(returnPath || { name: 'status' })
    } else {
      await router.replace({ name: 'login' })
    }
  } finally {
    retrying.value = false
  }
}
</script>

<template>
  <VbenFallback
    :status="status"
    :retry-label="retryLabel"
    :retry-loading="retrying"
    @home="goHome"
    @retry="retry"
  />
</template>
