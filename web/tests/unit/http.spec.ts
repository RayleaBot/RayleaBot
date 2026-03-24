import { afterEach, describe, expect, it, vi } from 'vitest'

import { apiRequest, configureApiRuntime } from '@/lib/http'

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
})
