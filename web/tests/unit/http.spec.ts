import { afterEach, describe, expect, it, vi } from 'vitest'

import { apiDownload, apiRequest, configureApiRuntime } from '@/lib/http'

function unauthorizedResponse() {
  return new Response(
    JSON.stringify({
      error: {
        code: 'permission.denied',
        message: '需要有效的管理会话',
        message_key: 'errors.permission.denied',
        request_id: 'req_fixture_unauthorized',
      },
    }),
    {
      status: 401,
      headers: { 'Content-Type': 'application/json' },
    },
  )
}

describe('api runtime', () => {
  afterEach(() => {
    configureApiRuntime({
      getToken: () => null,
      onNetworkUnavailable: () => undefined,
      onReachable: () => undefined,
      onUnauthorized: () => undefined,
    })
  })

  it('reports the request token snapshot on unauthorized responses', async () => {
    let currentToken = 'stale-token'
    let resolveResponse: ((response: Response) => void) | null = null
    const unauthorizedTokens: Array<string | null> = []

    vi.stubGlobal(
      'fetch',
      vi.fn().mockImplementation(
        () =>
          new Promise<Response>((resolve) => {
            resolveResponse = resolve
          }),
      ),
    )

    configureApiRuntime({
      getToken: () => currentToken,
      onUnauthorized: (tokenSnapshot) => {
        unauthorizedTokens.push(tokenSnapshot)
      },
    })

    const request = apiRequest('/api/system/status')
    currentToken = 'fresh-token'
    resolveResponse?.(unauthorizedResponse())

    await expect(request).rejects.toThrow('需要有效的管理会话')
    expect(unauthorizedTokens).toEqual(['stale-token'])
  })

  it('reports network failures and reachable responses to the runtime', async () => {
    const networkUnavailable = vi.fn()
    const reachable = vi.fn()

    vi.stubGlobal('fetch', vi.fn()
      .mockRejectedValueOnce(new TypeError('Failed to fetch'))
      .mockResolvedValueOnce(new Response(JSON.stringify({ ok: true }), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })))

    configureApiRuntime({
      onNetworkUnavailable: networkUnavailable,
      onReachable: reachable,
    })

    await expect(apiRequest('/api/system/status')).rejects.toThrow('Failed to fetch')
    await expect(apiRequest('/api/system/status')).resolves.toEqual({ ok: true })

    expect(networkUnavailable).toHaveBeenCalledWith('/api/system/status', expect.any(Error))
    expect(reachable).toHaveBeenCalledWith('/api/system/status', 200)
  })

  it('reports dev proxy backend unavailable responses as network failures', async () => {
    const networkUnavailable = vi.fn()
    const reachable = vi.fn()

    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response(JSON.stringify({
      error: {
        message: '管理服务暂不可用。',
      },
    }), {
      status: 503,
      headers: {
        'Content-Type': 'application/json',
        'x-rayleabot-backend-unavailable': '1',
      },
    })))

    configureApiRuntime({
      onNetworkUnavailable: networkUnavailable,
      onReachable: reachable,
    })

    await expect(apiRequest('/api/system/status')).rejects.toThrow('管理服务暂不可用。')

    expect(networkUnavailable).toHaveBeenCalledWith('/api/system/status', expect.any(Error))
    expect(reachable).not.toHaveBeenCalled()
  })

  it('parses plain content-disposition filenames without quotes', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('fixture', {
      status: 200,
      headers: {
        'Content-Disposition': 'attachment; filename=report.zip',
      },
    })))

    const response = await apiDownload('/api/files/report.zip')

    expect(response.filename).toBe('report.zip')
  })

  it('decodes percent-encoded plain content-disposition filenames', async () => {
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(new Response('fixture', {
      status: 200,
      headers: {
        'Content-Disposition': 'attachment; filename=report%20final.zip',
      },
    })))

    const response = await apiDownload('/api/files/report-final.zip')

    expect(response.filename).toBe('report final.zip')
  })
})
