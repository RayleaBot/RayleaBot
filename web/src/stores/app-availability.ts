import { computed, ref } from 'vue'
import { defineStore } from 'pinia'

export type OfflineSource = 'browser' | 'http' | 'websocket'

const exceptionPaths = new Set(['/403', '/404', '/500', '/offline', '/login', '/setup'])

function canRememberReturnPath(path: string | null | undefined) {
  if (!path || !path.startsWith('/')) {
    return false
  }

  const normalizedPath = path.split('?')[0]?.split('#')[0] || path
  return !exceptionPaths.has(normalizedPath)
}

export const useAppAvailabilityStore = defineStore('app-availability', () => {
  const offlineSource = ref<OfflineSource | null>(null)
  const returnPath = ref<string | null>(null)
  const lastOfflineAt = ref<string | null>(null)

  const isOffline = computed(() => offlineSource.value !== null)

  function markOffline(source: OfflineSource, currentPath?: string | null) {
    if (canRememberReturnPath(currentPath)) {
      returnPath.value = currentPath ?? null
    }

    offlineSource.value = source
    lastOfflineAt.value = new Date().toISOString()
  }

  function markOnline() {
    offlineSource.value = null
    lastOfflineAt.value = null
  }

  function consumeReturnPath() {
    const path = returnPath.value
    returnPath.value = null
    return path
  }

  function clearReturnPath() {
    returnPath.value = null
  }

  return {
    isOffline,
    lastOfflineAt,
    offlineSource,
    returnPath,
    clearReturnPath,
    consumeReturnPath,
    markOffline,
    markOnline,
  }
})
