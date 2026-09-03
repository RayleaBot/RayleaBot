<script setup lang="ts">
import { computed } from 'vue'
import { useRoute, useRouter } from 'vue-router'

import VbenFallback from '@/components/fallback/VbenFallback.vue'
import type { ExceptionStatus } from '@/lib/exception-status'
import { useSessionStore } from '@/stores/session'

const props = defineProps<{
  status?: ExceptionStatus
}>()

const route = useRoute()
const router = useRouter()
const sessionStore = useSessionStore()
const status = computed<ExceptionStatus>(() => props.status ?? route.meta.exceptionStatus ?? '500')

function resolveHomeRoute() {
  if (sessionStore.requiresSetup) {
    return { name: 'setup' }
  }

  return sessionStore.isAuthenticated ? { name: 'status' } : { name: 'login' }
}

async function goHome() {
  await router.push(resolveHomeRoute())
}
</script>

<template>
  <VbenFallback
    :status="status"
    :show-retry="false"
    @home="goHome"
  />
</template>
