import { afterEach, describe, expect, it, vi } from 'vitest'

import { parseBotAccountKey, PluginUIClient } from '../src/client'

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

  it('reads the account key verbatim after the first colon and rejects malformed keys', () => {
    expect(parseBotAccountKey('qqofficial:bot:id')).toEqual({ source_protocol: 'qqofficial', bot_id: 'bot:id' })
    expect(parseBotAccountKey('onebot11:10001')).toEqual({ source_protocol: 'onebot11', bot_id: '10001' })
    for (const value of ['onebot11:', ':10001', 'telegram:10001', '10001']) {
      expect(parseBotAccountKey(value)).toBeNull()
    }
    expect(parseBotAccountKey(['onebot11:10001', 'onebot11:10002'])).toBeNull()
  })

  it('sends the page and account with actions of a bot_account page only', async () => {
    const fetch = vi.fn().mockImplementation(async (path: string) => {
      if (path === '/api/plugins/example-config-panel') {
        return jsonResponse({ plugin: { id: 'example-config-panel', management_ui: { pages: [
          { id: 'records', label: '记录', scope: 'bot_account' },
          { id: 'config', label: '配置', scope: 'global' },
        ] } } })
      }
      return path.endsWith('/secrets')
        ? jsonResponse({ configured: {} }, { headers: { 'content-type': 'application/json', 'X-Raylea-CSRF': 'csrf-fixture' } })
        : jsonResponse({ plugin_id: 'example-config-panel', action: 'records.list', result: {} })
    })
    const bot = { source_protocol: 'onebot11' as const, bot_id: '10001' }
    const accountPage = new PluginUIClient({ pluginId: 'example-config-panel', pageId: 'records', bot, fetch })
    const globalPage = new PluginUIClient({ pluginId: 'example-config-panel', pageId: 'config', bot, fetch })

    await accountPage.invokeAction('records.list', { query: 'x' })
    await globalPage.invokeAction('config.check')

    const sent = fetch.mock.calls.filter(([path]) => String(path).endsWith('/management/actions')).map(([, init]) => JSON.parse(String((init as RequestInit).body)))
    expect(sent).toEqual([
      { action: 'records.list', payload: { query: 'x' }, page_id: 'records', bot },
      { action: 'config.check', payload: {} },
    ])
  })

  it('refuses a bot_account page that was opened without a valid account', async () => {
    const fetch = vi.fn().mockImplementation(async (path: string) => {
      if (path === '/api/plugins/example-config-panel') {
        return jsonResponse({ plugin: { id: 'example-config-panel', management_ui: { pages: [{ id: 'records', label: '记录', scope: 'bot_account' }] } } })
      }
      return path.endsWith('/secrets') ? jsonResponse({ configured: {} }) : jsonResponse({ values: {} })
    })
    const client = new PluginUIClient({ pluginId: 'example-config-panel', pageId: 'records', bot: null, fetch })

    await expect(client.loadContext()).rejects.toMatchObject({ code: 'plugin.ui_bot_account_missing' })
    expect(fetch.mock.calls.some(([path]) => String(path).startsWith('/api/bot-accounts/'))).toBe(false)
  })

  it('identifies the plugin and page from the plugin UI address', () => {
    vi.stubGlobal('location', { pathname: '/plugin-ui/example-config-panel/index.html', search: '?page=config' })
    const client = new PluginUIClient({ fetch: vi.fn() })

    expect(client.pluginId).toBe('example-config-panel')
    expect(client.pageId).toBe('config')
  })
})
