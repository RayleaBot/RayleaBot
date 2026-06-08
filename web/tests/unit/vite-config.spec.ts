import { resolve as resolvePath } from 'node:path'

import { createBackendProxyOptions, createRayleaBotDevStatus, resolveClientWebSocketBaseUrl, resolveDevWebSocketBaseUrl, resolveServerFsAllow } from '../../vite.config'

describe('vite config', () => {
  it('uses the backend target when the dev WebSocket base URL is empty', () => {
    expect(resolveDevWebSocketBaseUrl(undefined, 'http://127.0.0.1:8080')).toBe('http://127.0.0.1:8080')
    expect(resolveDevWebSocketBaseUrl('   ', 'http://127.0.0.1:8080')).toBe('http://127.0.0.1:8080')
  })

  it('keeps an explicit dev WebSocket base URL', () => {
    expect(resolveDevWebSocketBaseUrl('ws://127.0.0.1:4010', 'http://127.0.0.1:8080')).toBe('ws://127.0.0.1:4010')
  })

  it('does not pin built assets to a development WebSocket base URL', () => {
    expect(resolveClientWebSocketBaseUrl('build', 'ws://127.0.0.1:4010', 'http://127.0.0.1:8080')).toBe('')
  })

  it('does not route WebSocket traffic through the Vite backend proxy', () => {
    expect(createBackendProxyOptions('http://127.0.0.1:8080')).toMatchObject({
      target: 'http://127.0.0.1:8080',
      changeOrigin: true,
      ws: false,
    })
  })

  it('allows both the web app root and shared templates in dev server fs access', () => {
    expect(resolveServerFsAllow('C:/repo/web')).toEqual([
      resolvePath('C:/repo/web'),
      resolvePath('C:/repo/templates'),
    ])
  })

  it('exposes the dev backend target for start script reuse checks', () => {
    const middlewares: Array<(request: { url?: string }, response: {
      body?: string
      headers?: Record<string, string>
      status?: number
      end: (body: string) => void
      writeHead: (status: number, headers: Record<string, string>) => void
    }, next: () => void) => void> = []
    const plugin = createRayleaBotDevStatus('http://127.0.0.1:8080')

    plugin.configureServer?.({
      middlewares: {
        use(handler) {
          middlewares.push(handler)
        },
      },
    } as never)

    let nextCalled = false
    const response = {
      end(body: string) {
        this.body = body
      },
      writeHead(status: number, headers: Record<string, string>) {
        this.status = status
        this.headers = headers
      },
    }

    middlewares[0]?.({ url: '/__rayleabot-dev/status' }, response, () => {
      nextCalled = true
    })

    expect(nextCalled).toBe(false)
    expect(response.status).toBe(200)
    expect(response.headers?.['Content-Type']).toBe('application/json; charset=utf-8')
    expect(JSON.parse(response.body ?? '{}')).toEqual({
      app: 'RayleaBot Web',
      backendTarget: 'http://127.0.0.1:8080',
    })
  })
})
