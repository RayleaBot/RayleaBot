import { watch } from 'vue'

import type { useAppAvailabilityStore } from '@/stores/app-availability'
import type { useSessionStore } from '@/stores/session'
import type { useSocketStore } from '@/stores/sockets'

const websocketFailureConfirmationDelayMs = 2500
const requestFailureConfirmationDelayMs = 800
const backendProbeTimeoutMs = 2500
const backendRecoveryProbeIntervalMs = 2500

// A failed request or both core sockets dropping only marks the backend unavailable after
// /healthz also fails; while interrupted, /healthz is probed until the backend returns.
export function installBackendAvailabilityMonitor(
  sessionStore: ReturnType<typeof useSessionStore>,
  socketStore: ReturnType<typeof useSocketStore>,
  availabilityStore: ReturnType<typeof useAppAvailabilityStore>,
) {
  let requestFailureTimer: number | null = null
  let websocketFailureTimer: number | null = null
  let backendRecoveryTimer: number | null = null
  let backendProbeInFlight = false
  let disposed = false
  const probes = new Set<AbortController>()

  function clearRequestFailureTimer() {
    if (requestFailureTimer !== null) {
      window.clearTimeout(requestFailureTimer)
      requestFailureTimer = null
    }
  }

  function clearWebsocketFailureTimer() {
    if (websocketFailureTimer !== null) {
      window.clearTimeout(websocketFailureTimer)
      websocketFailureTimer = null
    }
  }

  function clearFailureTimers() {
    clearRequestFailureTimer()
    clearWebsocketFailureTimer()
  }

  function clearBackendRecoveryTimer() {
    if (backendRecoveryTimer !== null) {
      window.clearInterval(backendRecoveryTimer)
      backendRecoveryTimer = null
    }
  }

  async function canReachBackend() {
    const controller = new AbortController()
    probes.add(controller)
    const timeoutId = window.setTimeout(() => controller.abort(), backendProbeTimeoutMs)
    try {
      const response = await fetch('/healthz', {
        cache: 'no-store',
        signal: controller.signal,
      })
      return response.ok
    } catch {
      return false
    } finally {
      window.clearTimeout(timeoutId)
      probes.delete(controller)
    }
  }

  function markConnected() {
    if (disposed) return
    clearFailureTimers()
    clearBackendRecoveryTimer()
    availabilityStore.markConnected()
  }

  async function probeBackendRecovery() {
    if (disposed || backendProbeInFlight || !availabilityStore.isConnectionInterrupted) {
      return
    }

    backendProbeInFlight = true
    try {
      if (!(await canReachBackend())) {
        return
      }

      if (disposed) return
      markConnected()
      if (!sessionStore.isBootstrapped) {
        await sessionStore.bootstrap(true).catch(() => undefined)
      }
      if (sessionStore.isAuthenticated) {
        socketStore.reconnectAll()
      }
    } finally {
      backendProbeInFlight = false
    }
  }

  function ensureBackendRecoveryTimer() {
    if (backendRecoveryTimer !== null) {
      return
    }

    backendRecoveryTimer = window.setInterval(() => {
      void probeBackendRecovery()
    }, backendRecoveryProbeIntervalMs)
  }

  function markConnectionInterrupted() {
    if (disposed) return
    clearFailureTimers()
    availabilityStore.markConnectionInterrupted()
    ensureBackendRecoveryTimer()
  }

  function scheduleConnectionFailureConfirmation(
    source: 'http' | 'websocket',
    delayMs: number,
  ) {
    if (disposed) return
    if (availabilityStore.isConnectionInterrupted) {
      ensureBackendRecoveryTimer()
      return
    }

    const pendingTimer = source === 'http' ? requestFailureTimer : websocketFailureTimer
    if (pendingTimer !== null) {
      return
    }

    const timer = window.setTimeout(async () => {
      if (source === 'http') {
        requestFailureTimer = null
      } else {
        websocketFailureTimer = null
      }

      if (await canReachBackend()) {
        markConnected()
        return
      }

      markConnectionInterrupted()
    }, delayMs)

    if (source === 'http') {
      requestFailureTimer = timer
    } else {
      websocketFailureTimer = timer
    }
  }

  const onOnline = () => { void probeBackendRecovery() }
  if (typeof window !== 'undefined') {
    window.addEventListener('offline', markConnectionInterrupted)
    window.addEventListener('online', onOnline)
  }

  const stopSocketsWatch = watch(
    () => [
      sessionStore.isAuthenticated,
      socketStore.snapshots.events.status,
      socketStore.snapshots.logs.status,
    ] as const,
    ([isAuthenticated, eventsStatus, logsStatus]) => {
      const coreSocketsUnavailable = [eventsStatus, logsStatus].every(
        (status) => status === 'disconnected' || status === 'reconnecting',
      )

      if (!isAuthenticated || !coreSocketsUnavailable) {
        clearWebsocketFailureTimer()
        return
      }

      scheduleConnectionFailureConfirmation('websocket', websocketFailureConfirmationDelayMs)
    },
    { immediate: true },
  )

  const stopAvailabilityWatch = watch(
    () => availabilityStore.isConnectionInterrupted,
    (isConnectionInterrupted) => {
      if (isConnectionInterrupted) {
        ensureBackendRecoveryTimer()
        return
      }

      clearBackendRecoveryTimer()
    },
    { immediate: true },
  )
  return {
    callbacks: {
      onNetworkUnavailable: () => scheduleConnectionFailureConfirmation('http', requestFailureConfirmationDelayMs),
      onReachable: markConnected,
    },
    dispose() {
      disposed = true
      clearFailureTimers()
      clearBackendRecoveryTimer()
      for (const controller of probes) controller.abort()
      probes.clear()
      stopSocketsWatch()
      stopAvailabilityWatch()
      window.removeEventListener('offline', markConnectionInterrupted)
      window.removeEventListener('online', onOnline)
    },
  }
}
