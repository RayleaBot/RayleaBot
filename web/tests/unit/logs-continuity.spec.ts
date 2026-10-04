import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, disposePinia, setActivePinia } from 'pinia'
import { defineComponent, h, nextTick } from 'vue'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { useLogHistoryStore } from '@/stores/log-history'
import { useLogsStore } from '@/stores/logs'
import type { LogSummary } from '@/types/api'

function row(index: number, boot = 'a'): LogSummary {
  return { log_id: `${boot}-${index}`, timestamp: new Date(Date.UTC(2026, 3, 5) + index * 1000).toISOString(), level: 'info', source: 'runtime', message: `${boot} ${index}` }
}

function response(items: LogSummary[], cursor: string | null = null) {
  return new Response(JSON.stringify({ items, page: { has_older: Boolean(cursor), older_cursor: cursor } }), { headers: { 'Content-Type': 'application/json' } })
}

function deferred<T>() {
  let resolve!: (value: T) => void
  const promise = new Promise<T>(r => { resolve = r })
  return { promise, resolve }
}

describe('log window continuity', () => {
  let pinia: ReturnType<typeof createPinia>
  beforeEach(() => {
    vi.useFakeTimers()
    pinia = createPinia()
    setActivePinia(pinia)
  })
  afterEach(() => {
    disposePinia(pinia)
    vi.restoreAllMocks()
    vi.useRealTimers()
  })

  it('loads HTTP after WS replay and excludes replay outside its contiguous window', async () => {
    const fetch = vi.fn().mockResolvedValue(response([row(99), row(100)], 'opaque-old'))
    vi.stubGlobal('fetch', fetch)
    const store = useLogsStore()
    store.setViewportActive(true)
    store.appendBatch([row(1), row(100)])
    expect(store.initialized).toBe(false)
    await store.ensureLoaded()
    expect(fetch).toHaveBeenCalledTimes(1)
    expect(store.items.map(log => log.log_id)).toEqual(['a-99', 'a-100'])
    expect(store.hasOlder).toBe(true)
    fetch.mockResolvedValueOnce(response([row(1)]))
    await store.loadOlder()
    expect(fetch.mock.calls[1]![0]).toContain('cursor=opaque-old')
    expect(store.items[0]!.log_id).toBe('a-1')
  })

  it('replaces filters without losing matching frames received during the request', async () => {
    const pending = deferred<Response>()
    vi.stubGlobal('fetch', vi.fn().mockReturnValue(pending.promise))
    const store = useLogsStore()
    store.setViewportActive(true)
    store.filters = { levels: ['info'] }
    const loading = store.applyFilters()
    store.append(row(3))
    pending.resolve(response([row(2)]))
    await loading
    expect(store.items.map(log => log.log_id)).toEqual(['a-2', 'a-3'])
  })

  it('keeps late replay outside the hydrated window accessible through its original older cursor', async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce(response([row(99), row(100)], 'opaque-older'))
      .mockResolvedValueOnce(response([row(99), row(100), row(101)], 'newer-page-cursor'))
      .mockResolvedValueOnce(response(Array.from({ length: 98 }, (_, i) => row(i + 1))))
    vi.stubGlobal('fetch', fetch)
    const store = useLogsStore()
    store.setViewportActive(true)
    await store.ensureLoaded()
    store.append(row(1))
    expect(store.items.map(log => log.log_id)).toEqual(['a-99', 'a-100'])
    expect(store.hasOlder).toBe(true)
    await vi.advanceTimersByTimeAsync(2000)
    expect(fetch).toHaveBeenCalledTimes(2)
    expect(store.items.map(log => log.log_id)).toEqual(['a-99', 'a-100', 'a-101'])
    await store.loadOlder()
    expect(fetch.mock.calls[2]![0]).toContain('cursor=opaque-older')
    expect(store.items.map(log => log.log_id)).toEqual(Array.from({ length: 101 }, (_, i) => `a-${i + 1}`))
    expect(store.hasOlder).toBe(false)
  })

  it('accepts late same-instant frames and existing-ID updates at the hydrated boundary', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(response([row(2), row(3)], 'older')))
    const store = useLogsStore()
    store.setViewportActive(true)
    await store.ensureLoaded()
    store.appendBatch([{ ...row(2), log_id: '0-same-instant' }, { ...row(1), log_id: 'a-3', message: 'updated' }])
    expect(store.items.map(log => log.log_id)).toEqual(['a-3', '0-same-instant', 'a-2'])
    expect(store.items[0]!.message).toBe('updated')
  })

  it('buffers new-boot frames below the old window boundary while reconnect hydration runs', async () => {
    const pending = deferred<Response>()
    vi.stubGlobal('fetch', vi.fn()
      .mockResolvedValueOnce(response([row(99), row(100)], 'old-boot-cursor'))
      .mockReturnValueOnce(pending.promise))
    const store = useLogsStore()
    store.setViewportActive(true)
    await store.ensureLoaded()
    store.onStreamConnected()
    store.append(row(3, 'b'))
    pending.resolve(response([row(2, 'b')], 'new-boot-cursor'))
    await flushPromises()
    expect(store.items.map(log => log.log_id)).toEqual(['b-2', 'b-3'])
    expect(store.hasOlder).toBe(true)
  })

  it('keeps same-nanosecond frames and same-ID updates independently of ID ordering at the HTTP boundary', async () => {
    const pending = deferred<Response>()
    vi.stubGlobal('fetch', vi.fn().mockReturnValue(pending.promise))
    const store = useLogsStore()
    store.setViewportActive(true)
    const loading = store.ensureLoaded()
    const oldest = { ...row(2), log_id: 'z-oldest', timestamp: '2026-04-05T00:00:02.000000001Z' }
    const sameInstant = { ...oldest, log_id: 'a-same-instant' }
    const updated = { ...row(1), log_id: 'updated', message: 'latest WS content' }
    store.appendBatch([sameInstant, updated, { ...row(0), log_id: 'older-replay' }])
    pending.resolve(response([oldest, { ...row(3), log_id: 'updated' }], 'older'))
    await loading
    expect(store.items.map(log => log.log_id)).toEqual(['updated', 'a-same-instant', 'z-oldest'])
    expect(store.items[0]!.message).toBe('latest WS content')
    expect(store.hasOlder).toBe(true)
  })

  it('discards unconfirmed rows from the old connection when its first HTTP load never completed', async () => {
    const first = deferred<Response>()
    const second = deferred<Response>()
    vi.stubGlobal('fetch', vi.fn().mockReturnValueOnce(first.promise).mockReturnValueOnce(second.promise))
    const store = useLogsStore()
    store.setViewportActive(true)
    const oldLoading = store.ensureLoaded()
    store.append(row(1, 'a'))
    expect(store.initialized).toBe(false)
    store.onStreamConnected()
    store.append(row(3, 'b'))
    second.resolve(response([row(2, 'b')]))
    await flushPromises()
    first.resolve(response([row(1, 'a')]))
    await oldLoading
    expect(store.items.map(log => log.log_id)).toEqual(['b-2', 'b-3'])
    expect(store.initialized).toBe(true)
    expect(store.hasOlder).toBe(false)
  })

  it.each(['live', 'history'] as const)('isolates an old %s pagination response after a new filter query', async kind => {
    const pending = deferred<Response>()
    let oldSignal: AbortSignal | undefined
    const fetch = vi.fn()
      .mockResolvedValueOnce(response([row(3)], 'old-page'))
      .mockImplementationOnce((_url, options: RequestInit) => { oldSignal = options.signal as AbortSignal; return pending.promise })
      .mockResolvedValueOnce(response([row(9)]))
    vi.stubGlobal('fetch', fetch)
    const store = kind === 'live' ? useLogsStore() : useLogHistoryStore()
    if (kind === 'live') await useLogsStore().ensureLoaded()
    else await useLogHistoryStore().refreshAnchor()
    const older = store.loadOlder()
    await flushPromises()
    store.filters = { levels: ['warn'] }
    await store.applyFilters()
    expect(oldSignal?.aborted).toBe(true)
    pending.resolve(response([row(1)], 'stale-cursor'))
    await older
    expect(store.items.map(log => log.log_id)).toEqual(['a-9'])
    expect(store.hasOlder).toBe(false)
    expect(store.loadingOlder).toBe(false)
    expect(store.error).toBeNull()
  })

  it('fills dropped live frames through HTTP pages up to the last HTTP boundary', async () => {
    let history = [row(1)]
    const cursors = new Map<string, number>()
    let sequence = 0
    const requests: string[] = []
    vi.stubGlobal('fetch', vi.fn(async (url: string) => {
      requests.push(url)
      const cursor = new URL(url, 'http://localhost').searchParams.get('cursor')
      const end = cursor ? cursors.get(cursor)! : history.length
      const start = Math.max(0, end - 100)
      const next = start ? `opaque-${++sequence}` : null
      if (next) cursors.set(next, start)
      return response(history.slice(start, end), next)
    }))
    const store = useLogsStore()
    store.setViewportActive(true)
    await store.ensureLoaded()
    history = Array.from({ length: 251 }, (_, index) => row(index + 1))
    store.append(row(251))
    await vi.advanceTimersByTimeAsync(2010)
    expect(requests).toHaveLength(4)
    expect(requests.slice(1).filter(url => url.includes('cursor='))).toHaveLength(2)
    expect(store.items.map(log => log.log_id)).toEqual(history.map(log => log.log_id))
    expect(store.hasOlder).toBe(false)
    await vi.advanceTimersByTimeAsync(10_000)
    expect(requests).toHaveLength(4)
  })

  it('preserves WS frames arriving during gap repair and counts duplicates once', async () => {
    const pending = deferred<Response>()
    vi.stubGlobal('fetch', vi.fn().mockResolvedValueOnce(response([row(1)])).mockReturnValueOnce(pending.promise))
    const store = useLogsStore()
    store.setViewportActive(true)
    store.setViewportAtBottom(false)
    await store.ensureLoaded()
    store.appendBatch([row(3), row(3)])
    await vi.advanceTimersByTimeAsync(2000)
    store.append(row(4))
    pending.resolve(response([row(1), row(2), row(3)]))
    await flushPromises()
    expect(store.items.map(log => log.log_id)).toEqual(['a-1', 'a-2', 'a-3', 'a-4'])
    expect(store.pendingNewCount).toBe(3)
  })

  it('waits for an older page already in flight when the reconciliation timer fires', async () => {
    const pending = deferred<Response>()
    const fetch = vi.fn()
      .mockResolvedValueOnce(response([row(3)], 'older'))
      .mockReturnValueOnce(pending.promise)
      .mockResolvedValueOnce(response([row(3), row(4), row(5)]))
    vi.stubGlobal('fetch', fetch)
    const store = useLogsStore()
    store.setViewportActive(true)
    await store.ensureLoaded()
    store.append(row(5))
    const older = store.loadOlder()
    await vi.advanceTimersByTimeAsync(2000)
    expect(fetch).toHaveBeenCalledTimes(2)
    pending.resolve(response([row(1), row(2)]))
    await older
    await vi.advanceTimersByTimeAsync(2000)
    expect(fetch).toHaveBeenCalledTimes(3)
    expect(store.items.map(log => log.log_id)).toEqual(['a-1', 'a-2', 'a-3', 'a-4', 'a-5'])
    expect(store.hasOlder).toBe(false)
  })

  it('cancels live reads on leaving and rebuilds the retained page count after reconnect', async () => {
    const pending = deferred<Response>()
    let signal: AbortSignal | undefined
    const fetch = vi.fn()
      .mockResolvedValueOnce(response(Array.from({ length: 100 }, (_, i) => row(i + 101)), 'a-old'))
      .mockResolvedValueOnce(response(Array.from({ length: 100 }, (_, i) => row(i + 1))))
      .mockImplementationOnce((_url, options: RequestInit) => { signal = options.signal as AbortSignal; return pending.promise })
      .mockResolvedValueOnce(response(Array.from({ length: 100 }, (_, i) => row(i + 101, 'b')), 'b-old'))
      .mockResolvedValueOnce(response(Array.from({ length: 100 }, (_, i) => row(i + 1, 'b'))))
    vi.stubGlobal('fetch', fetch)
    const store = useLogsStore()
    store.setViewportActive(true)
    await store.ensureLoaded()
    await store.loadOlder()
    store.append(row(201))
    await vi.advanceTimersByTimeAsync(2000)
    store.setViewportActive(false)
    expect(signal?.aborted).toBe(true)
    for (let i = 202; i < 6000; i++) store.append(row(i))
    expect(store.items).toHaveLength(201)
    store.onStreamConnected()
    store.setViewportActive(true)
    const loading = store.ensureLoaded()
    await vi.advanceTimersByTimeAsync(1)
    await loading
    pending.resolve(response([row(1)]))
    await flushPromises()
    expect(store.items).toHaveLength(200)
    expect(store.items.every(log => log.log_id.startsWith('b-'))).toBe(true)
    expect(fetch).toHaveBeenCalledTimes(5)
  })

  it('updates an ordinary Vue child when ordered rows are appended in place', async () => {
    const store = useLogsStore()
    store.setViewportActive(true)
    const Child = defineComponent({ props: { rows: { type: Array<LogSummary>, required: true } }, setup: props => () => h('span', `${props.rows.length}:${props.rows.at(-1)?.log_id}`) })
    const wrapper = mount(defineComponent({ setup: () => () => h(Child, { rows: store.items }) }))
    store.append(row(1))
    await nextTick()
    expect(wrapper.text()).toBe('1:a-1')
    store.appendBatch([row(2), row(3)])
    await nextTick()
    expect(wrapper.text()).toBe('3:a-3')
    wrapper.unmount()
  })

  it('stops accumulating and polling in a hidden tab, then refreshes its window when visible', async () => {
    const visibility = vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible')
    const fetch = vi.fn()
      .mockResolvedValueOnce(response([row(1)]))
      .mockResolvedValueOnce(response([row(1), row(2), row(3)]))
    vi.stubGlobal('fetch', fetch)
    const store = useLogsStore()
    store.setViewportActive(true)
    await store.ensureLoaded()
    visibility.mockReturnValue('hidden')
    document.dispatchEvent(new Event('visibilitychange'))
    store.appendBatch([row(2), row(3)])
    await vi.advanceTimersByTimeAsync(10_000)
    expect(store.items.map(log => log.log_id)).toEqual(['a-1'])
    expect(fetch).toHaveBeenCalledTimes(1)
    visibility.mockReturnValue('visible')
    document.dispatchEvent(new Event('visibilitychange'))
    await flushPromises()
    expect(fetch).toHaveBeenCalledTimes(2)
    expect(store.items.map(log => log.log_id)).toEqual(['a-1', 'a-2', 'a-3'])
  })
})
