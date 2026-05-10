import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useAppAvailabilityStore } from '@/stores/app-availability'

describe('app availability store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.useFakeTimers()
    vi.setSystemTime(new Date('2026-05-02T09:00:00Z'))
  })

  it('marks the app offline and remembers the current workspace path', () => {
    const store = useAppAvailabilityStore()

    store.markOffline('http', '/commands?plugin_id=raylea.echo')

    expect(store.isOffline).toBe(true)
    expect(store.offlineSource).toBe('http')
    expect(store.returnPath).toBe('/commands?plugin_id=raylea.echo')
    expect(store.lastOfflineAt).toBe('2026-05-02T09:00:00.000Z')
  })

  it('does not use exception routes as return paths', () => {
    const store = useAppAvailabilityStore()

    store.markOffline('websocket', '/offline')

    expect(store.returnPath).toBeNull()
  })

  it('clears offline state and consumes the remembered return path', () => {
    const store = useAppAvailabilityStore()

    store.markOffline('browser', '/plugins')
    store.markOnline()

    expect(store.isOffline).toBe(false)
    expect(store.consumeReturnPath()).toBe('/plugins')
    expect(store.returnPath).toBeNull()
  })
})
