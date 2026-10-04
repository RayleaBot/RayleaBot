import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useLogDetailController } from '@/components/logs/useLogDetailController'
import { apiRequest } from '@/lib/http'
import type { LogDetailResponse, LogSummary } from '@/types/api'

vi.mock('@/lib/http', () => ({ apiRequest: vi.fn() }))

const summary = (id: string): LogSummary => ({
  log_id: id, timestamp: '2026-04-02T00:00:00Z', level: 'info', source: 'runtime', message: id,
})
const response = (id: string): LogDetailResponse => ({ ...summary(id), details: { id } })

describe('log detail selection requests', () => {
  beforeEach(() => vi.mocked(apiRequest).mockReset())

  it.each(['resolve', 'reject'] as const)('clears loading on a cache hit and ignores a late %s', async (outcome) => {
    const controller = useLogDetailController()
    vi.mocked(apiRequest).mockResolvedValueOnce(response('cached'))
    await controller.openDetail(summary('cached'))
    let resolve!: (value: LogDetailResponse) => void
    let reject!: (reason: Error) => void
    vi.mocked(apiRequest).mockImplementationOnce(() => new Promise((yes, no) => { resolve = yes; reject = no }))
    const pending = controller.openDetail(summary('pending')).catch(() => undefined)
    const oldSignal = vi.mocked(apiRequest).mock.calls.at(-1)?.[1]?.signal
    expect(controller.loading.value).toBe(true)
    await controller.openDetail(summary('cached'))
    expect(controller.loading.value).toBe(false)
    expect(oldSignal?.aborted).toBe(true)
    expect(controller.currentDetail.value).toEqual(response('cached'))
    if (outcome === 'resolve') resolve(response('pending'))
    else reject(new Error('late error'))
    await pending
    expect(controller.currentDetail.value).toEqual(response('cached'))
    expect(controller.error.value).toBeNull()
    expect(controller.loading.value).toBe(false)
    expect(apiRequest).toHaveBeenCalledTimes(2)
  })

  it('ignores an old request after closing and reopening the same entry', async () => {
    const controller = useLogDetailController()
    let resolveOld!: (value: LogDetailResponse) => void
    let resolveNew!: (value: LogDetailResponse) => void
    vi.mocked(apiRequest)
      .mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve }))
      .mockImplementationOnce(() => new Promise(resolve => { resolveNew = resolve }))
    const oldRequest = controller.openDetail(summary('entry'))
    const oldSignal = vi.mocked(apiRequest).mock.calls.at(-1)?.[1]?.signal
    controller.closeDetail()
    expect(oldSignal?.aborted).toBe(true)
    expect(controller.open.value).toBe(false)
    expect(controller.selectedLogId.value).toBeNull()
    const newRequest = controller.openDetail(summary('entry'))
    resolveOld({ ...response('entry'), details: { version: 'old' } })
    await oldRequest
    expect(controller.loading.value).toBe(true)
    expect(controller.currentDetail.value).toBeNull()
    resolveNew({ ...response('entry'), details: { version: 'new' } })
    await newRequest
    expect(controller.loading.value).toBe(false)
    expect(controller.currentDetail.value?.details).toEqual({ version: 'new' })
  })
})
