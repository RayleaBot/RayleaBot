import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useProtocolLogsStore } from '@/stores/protocol-logs'

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('protocol logs store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('always queries the OneBot11 protocol surface and keeps auxiliary filters', async () => {
    const fetchMock = vi.fn().mockResolvedValue(jsonResponse({
      items: [
        {
          timestamp: '2026-04-05T08:00:00Z',
          level: 'warn',
          protocol: 'onebot11',
          source: 'adapter',
          request_id: 'req_adapter_1',
          message: 'authentication failed for reverse websocket',
        },
      ],
    }))
    vi.stubGlobal('fetch', fetchMock)

    const store = useProtocolLogsStore()
    store.filters = {
      protocol: 'onebot11',
      level: 'warn',
      source: 'adapter',
      requestId: 'req_adapter_1',
      limit: 10,
    }

    await store.fetchList()

    expect(fetchMock).toHaveBeenCalledWith(
      '/api/logs?level=warn&source=adapter&protocol=onebot11&request_id=req_adapter_1&limit=10',
      expect.any(Object),
    )
    expect(store.items[0].protocol).toBe('onebot11')
  })
})
