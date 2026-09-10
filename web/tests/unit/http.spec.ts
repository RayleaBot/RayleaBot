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
    vi.useRealTimers()
    vi.unstubAllGlobals()
    configureApiRuntime({
      getCSRFToken: () => null,
      onCSRFToken: () => undefined,
      onNetworkUnavailable: () => undefined,
      onReachable: () => undefined,
      onUnauthorized: () => undefined,
    })
  })

  describe.each([
    ['JSON', apiRequest],
    ['download', apiDownload],
  ] as const)('%s request lifecycle', (_name, request) => {
    it('does not dispatch a request whose caller has already cancelled', async () => {
      const fetch = vi.fn()
      const unavailable = vi.fn()
      vi.stubGlobal('fetch', fetch)
      configureApiRuntime({ onNetworkUnavailable: unavailable })
      const controller = new AbortController()
      controller.abort()
      await expect(request('/api/resource', { signal: controller.signal })).rejects.toMatchObject({ code: 'client.request_cancelled' })
      expect(fetch).not.toHaveBeenCalled()
      expect(unavailable).not.toHaveBeenCalled()
    })

    it.each(['success', 'HTTP failure', 'network failure'] as const)('releases caller abort listeners after %s', async outcome => {
      const controller = new AbortController()
      const add = vi.spyOn(controller.signal, 'addEventListener')
      const remove = vi.spyOn(controller.signal, 'removeEventListener')
      const fetch = outcome === 'network failure'
        ? vi.fn().mockRejectedValue(new TypeError('network failed'))
        : vi.fn().mockResolvedValue(new Response('{}', { status: outcome === 'success' ? 200 : 500, headers: { 'content-type': 'application/json' } }))
      vi.stubGlobal('fetch', fetch)
      await request('/api/resource', { signal: controller.signal }).catch(() => undefined)
      expect(add).toHaveBeenCalledTimes(1)
      expect(remove).toHaveBeenCalledWith('abort', add.mock.calls[0][1])
    })

    it.each(['cancelled', 'timeout'] as const)('classifies %s without marking the server unavailable', async cause => {
      vi.useFakeTimers()
      const unavailable = vi.fn()
      configureApiRuntime({ onNetworkUnavailable: unavailable })
      vi.stubGlobal('fetch', vi.fn((_path, init) => new Promise((_resolve, reject) => {
        init.signal.addEventListener('abort', () => reject(new DOMException('not relevant to classification', 'AbortError')), { once: true })
      })))
      const controller = new AbortController()
      const result = request('/api/resource', { signal: controller.signal, timeoutMs: 10 })
      const rejected = expect(result).rejects.toMatchObject({ code: `client.request_${cause}` })
      if (cause === 'cancelled') controller.abort()
      else await vi.advanceTimersByTimeAsync(10)
      await rejected
      expect(unavailable).not.toHaveBeenCalled()
    })

    it('keeps the timeout active while reading the response body', async () => {
      vi.useFakeTimers()
      const unavailable = vi.fn()
      configureApiRuntime({ onNetworkUnavailable: unavailable })
      vi.stubGlobal('fetch', vi.fn((_path, init) => {
        const body = new Promise<never>((_resolve, reject) => {
          init.signal.addEventListener('abort', () => reject(new DOMException('body aborted', 'AbortError')), { once: true })
        })
        return Promise.resolve({ ok: true, status: 200, headers: new Headers({ 'content-type': 'application/json' }), json: () => body, blob: () => body })
      }))
      const rejected = expect(request('/api/resource', { timeoutMs: 10 })).rejects.toMatchObject({ code: 'client.request_timeout' })
      await vi.advanceTimersByTimeAsync(10)
      await rejected
      expect(unavailable).not.toHaveBeenCalled()
    })

    it('treats accepted 503 responses as reachable and rotates CSRF on failures', async () => {
      const reachable = vi.fn()
      const unavailable = vi.fn()
      const rotate = vi.fn()
      configureApiRuntime({ onReachable: reachable, onNetworkUnavailable: unavailable, onCSRFToken: rotate })
      vi.stubGlobal('fetch', vi.fn().mockImplementation(() => Promise.resolve(new Response('{}', { status: 503, headers: { 'content-type': 'application/json', 'X-Raylea-CSRF': 'next-csrf' } }))))
      await request('/readyz', { acceptStatuses: [503] })
      await expect(request('/api/resource')).rejects.toMatchObject({ status: 503 })
      expect(reachable).toHaveBeenCalledTimes(2)
      expect(rotate).toHaveBeenCalledTimes(2)
      expect(unavailable).not.toHaveBeenCalled()
    })

    it('allows a caller to disable the timeout while retaining cancellation', async () => {
      vi.useFakeTimers()
      let signal: AbortSignal | undefined
      let resolve: ((value: Response) => void) | undefined
      vi.stubGlobal('fetch', vi.fn((_path, init) => {
        signal = init.signal
        return new Promise<Response>(accept => { resolve = accept })
      }))
      const result = request('/api/resource', { timeoutMs: 0 })
      await vi.advanceTimersByTimeAsync(60_000)
      expect(signal?.aborted).toBe(false)
      resolve?.(new Response(null, { status: 204 }))
      await result
    })
  })

  it('reports unauthorized cookie sessions without retaining bearer credentials', async () => {
    let resolveResponse: ((response: Response) => void) | null = null
    const onUnauthorized = vi.fn()

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
      onUnauthorized,
    })

    const request = apiRequest('/api/system/status')
    resolveResponse?.(unauthorizedResponse())

    await expect(request).rejects.toThrow('需要有效的管理会话')
    expect(onUnauthorized).toHaveBeenCalledOnce()
  })

  it('sends CSRF only for unsafe authenticated requests and rotates it from responses', async () => {
    const rotatedTokens: string[] = []
    const fetchMock = vi.fn().mockImplementation(() => Promise.resolve(new Response(JSON.stringify({ ok: true }), {
      status: 200,
      headers: {
        'Content-Type': 'application/json',
        'X-Raylea-CSRF': 'rotated-csrf-token',
      },
    })))
    vi.stubGlobal('fetch', fetchMock)

    configureApiRuntime({
      getCSRFToken: () => 'current-csrf-token',
      onCSRFToken: (token) => rotatedTokens.push(token),
    })

    await apiRequest('/api/system/status')
    await apiRequest('/api/config', { method: 'PUT', body: { fixture: true } })
    await apiRequest('/api/session/login', { method: 'POST', auth: false, body: { fixture: true } })

    const getHeaders = fetchMock.mock.calls[0]?.[1]?.headers as Headers
    const putHeaders = fetchMock.mock.calls[1]?.[1]?.headers as Headers
    const publicPostHeaders = fetchMock.mock.calls[2]?.[1]?.headers as Headers
    expect(getHeaders.has('X-Raylea-CSRF')).toBe(false)
    expect(putHeaders.get('X-Raylea-CSRF')).toBe('current-csrf-token')
    expect(publicPostHeaders.has('X-Raylea-CSRF')).toBe(false)
    expect(fetchMock.mock.calls.every(([, init]) => init?.credentials === 'same-origin')).toBe(true)
    expect(rotatedTokens).toEqual(['rotated-csrf-token', 'rotated-csrf-token', 'rotated-csrf-token'])
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
