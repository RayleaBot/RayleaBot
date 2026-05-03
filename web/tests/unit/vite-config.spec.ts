import { resolveClientWebSocketBaseUrl, resolveDevWebSocketBaseUrl } from '../../vite.config'

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
})
