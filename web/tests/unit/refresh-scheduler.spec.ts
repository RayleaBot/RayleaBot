import { afterEach, describe, expect, it, vi } from 'vitest'
import { createRefreshScheduler } from '@/lib/refresh-scheduler'

describe('refresh scheduling', () => {
  afterEach(() => vi.useRealTimers())

  it('publishes the last change after a burst that arrives during a pending refresh', async () => {
    vi.useFakeTimers()
    let source = 1
    let visible = 0
    let finish: (() => void) | undefined
    const refresh = vi.fn(async () => {
      const value = source
      await new Promise<void>(resolve => { finish = resolve })
      visible = value
    })
    const scheduler = createRefreshScheduler(refresh)
    scheduler.schedule()
    scheduler.schedule()
    await vi.advanceTimersByTimeAsync(120)
    source = 2
    scheduler.schedule()
    source = 3
    scheduler.schedule()
    finish?.()
    await vi.advanceTimersByTimeAsync(120)
    expect(refresh).toHaveBeenCalledTimes(2)
    finish?.()
    await vi.advanceTimersByTimeAsync(0)
    expect(visible).toBe(3)
    scheduler.cancel()
  })

  it('cancels pending and running work without reviving a queued refresh after logout', async () => {
    vi.useFakeTimers()
    const signals: AbortSignal[] = []
    let finish: (() => void) | undefined
    const refresh = vi.fn(async signal => {
      signals.push(signal)
      await new Promise<void>(resolve => { finish = resolve })
    })
    const scheduler = createRefreshScheduler(refresh)
    scheduler.schedule()
    scheduler.cancel()
    await vi.advanceTimersByTimeAsync(120)
    expect(refresh).not.toHaveBeenCalled()
    scheduler.schedule()
    await vi.advanceTimersByTimeAsync(120)
    scheduler.schedule()
    scheduler.cancel()
    expect(signals[0].aborted).toBe(true)
    finish?.()
    await vi.advanceTimersByTimeAsync(500)
    expect(refresh).toHaveBeenCalledTimes(1)
    scheduler.schedule()
    await vi.advanceTimersByTimeAsync(120)
    expect(refresh).toHaveBeenCalledTimes(2)
    scheduler.cancel()
    finish?.()
  })
})
