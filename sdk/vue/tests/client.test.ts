import { afterEach, describe, expect, it, vi } from 'vitest'

import { PluginUIClient } from '../src/client'

function jsonResponse(body: unknown, init: ResponseInit = {}) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'content-type': 'application/json' },
    ...init,
  })
}

describe('PluginUIClient', () => {
  afterEach(() => {
    vi.unstubAllGlobals()
  })

  it('calls the plugin-scoped management API and reuses the session CSRF token', async () => {
    const fetch = vi.fn()
      .mockResolvedValueOnce(jsonResponse(
        { plugin_id: 'example-config-panel', values: { default_city: '上海' } },
        { headers: { 'content-type': 'application/json', 'X-Raylea-CSRF': 'csrf-fixture' } },
      ))
      .mockResolvedValueOnce(jsonResponse({ plugin_id: 'example-config-panel', values: { default_city: '广州' }, changed_keys: ['default_city'] }))
    const client = new PluginUIClient({ pluginId: 'example-config-panel', pageId: 'config', fetch })

    await expect(client.reloadSettings()).resolves.toEqual({ config: { default_city: '上海' } })
    await expect(client.saveSettings({ default_city: '广州' })).resolves.toEqual({ config: { default_city: '广州' } })

    const [path, init] = fetch.mock.calls[1] as [string, RequestInit]
    expect(path).toBe('/api/plugins/example-config-panel/settings')
    expect(init.method).toBe('PUT')
    expect(new Headers(init.headers).get('X-Raylea-CSRF')).toBe('csrf-fixture')
    expect(JSON.parse(String(init.body))).toEqual({ values: { default_city: '广州' } })
  })

  it('reports the stable code from the management error envelope', async () => {
    const fetch = vi.fn().mockResolvedValue(jsonResponse(
      { error: { code: 'plugin.management_action_failed', message: '插件管理动作失败' } },
      { status: 502, headers: { 'content-type': 'application/json' } },
    ))
    const client = new PluginUIClient({ pluginId: 'example-config-panel', fetch })

    await expect(client.apiRequest('GET', '/api/plugins/example-config-panel/secrets')).rejects.toMatchObject({
      code: 'plugin.management_action_failed',
      status: 502,
    })
  })

  it('identifies the plugin and page from the plugin UI address', () => {
    vi.stubGlobal('location', { pathname: '/plugin-ui/example-config-panel/index.html', search: '?page=config' })
    const client = new PluginUIClient({ fetch: vi.fn() })

    expect(client.pluginId).toBe('example-config-panel')
    expect(client.pageId).toBe('config')
  })
})
