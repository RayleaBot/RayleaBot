import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useAppAvailabilityStore } from '@/stores/app-availability'

describe('app availability store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-05-02T09:00:00Z'))
  })

  it('records a connection interruption without changing workspace state', () => {
    const store = useAppAvailabilityStore()

    store.markConnectionInterrupted('http')

    expect(store.isConnectionInterrupted).toBe(true)
    expect(store.connectionIssueSource).toBe('http')
    expect(store.lastConnectionIssueAt).toBe('2026-05-02T09:00:00.000Z')
  })

  it('clears the interruption state after the connection recovers', () => {
    const store = useAppAvailabilityStore()

    store.markConnectionInterrupted('browser')
    store.markConnected()

    expect(store.isConnectionInterrupted).toBe(false)
    expect(store.connectionIssueSource).toBeNull()
    expect(store.lastConnectionIssueAt).toBeNull()
  })
})
