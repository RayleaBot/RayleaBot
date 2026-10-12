import { createPinia, disposePinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import PluginBotAccountBar from '@/components/plugins/PluginBotAccountBar.vue'
import { buildPluginDetailLocation, readPluginBotAccount } from '@/lib/management-links'
import type { BotAccount } from '@/types/api'

const json = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json' } })
const online: BotAccount = { source_protocol: 'onebot11', bot_id: '10001', nickname: '小雷', state: 'online', adapter_id: 'napcat-home', adapter_display_name: 'OneBot11（napcat-home）', last_seen_at: '2026-10-12T08:00:00Z' }
const detached: BotAccount = { source_protocol: 'onebot11', bot_id: '10003', state: 'no_connection', last_adapter_id: 'napcat-old', last_adapter_display_name: 'OneBot11（napcat-old）', last_seen_at: '2026-10-10T08:00:00Z' }

function stubDirectory(listed: BotAccount[], single: Record<string, BotAccount> = {}) {
  vi.stubGlobal('fetch', vi.fn(async (path: string) => {
    const url = new URL(path, 'http://local.test')
    if (url.pathname === '/api/adapters') return json({ adapters: [], available_protocols: [] })
    if (url.pathname === '/api/bot-accounts') return json({ items: listed, total: listed.length })
    const key = decodeURIComponent(url.pathname.slice('/api/bot-accounts/'.length)).replace('/', ':')
    return single[key] ? json(single[key]) : json({ error: { code: 'platform.resource_not_found', message: '请求的资源不存在' } }, 404)
  }))
}

function mountBar(requested: { present: boolean; account: BotAccount | { source_protocol: 'onebot11'; bot_id: string } | null }) {
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/', component: { template: '<div />' } }, { path: '/protocols', name: 'protocols', component: { template: '<div />' } }] })
  return mount(PluginBotAccountBar, { props: { requested }, global: { plugins: [router] } })
}

describe('plugin bot account bar', () => {
  let pinia: ReturnType<typeof createPinia>
  beforeEach(() => { pinia = createPinia(); setActivePinia(pinia) })
  afterEach(() => { disposePinia(pinia); vi.unstubAllGlobals() })

  it('fills in the first directory account only when the address names none', async () => {
    stubDirectory([online, detached])
    const wrapper = mountBar({ present: false, account: null })
    await flushPromises()

    expect(wrapper.emitted('select')).toEqual([[online, { replace: true }]])
    expect(wrapper.emitted('resolved')?.at(-1)).toEqual([null])
  })

  it('reports a malformed address account instead of switching to another account', async () => {
    stubDirectory([online])
    const wrapper = mountBar({ present: true, account: null })
    await flushPromises()

    expect(wrapper.emitted('select')).toBeUndefined()
    expect(wrapper.emitted('resolved')?.at(-1)).toEqual([null])
    expect(wrapper.find('[data-testid="plugin-bot-account-problem"]').exists()).toBe(true)
  })

  it('reads an account beyond the loaded page and reports one the directory no longer holds', async () => {
    stubDirectory([online], { 'onebot11:10003': detached })
    const found = mountBar({ present: true, account: { source_protocol: 'onebot11', bot_id: '10003' } })
    await flushPromises()
    expect(found.emitted('resolved')?.at(-1)).toEqual([detached])

    const missing = mountBar({ present: true, account: { source_protocol: 'onebot11', bot_id: '10009' } })
    await flushPromises()
    expect(missing.emitted('resolved')?.at(-1)).toEqual([null])
    expect(missing.find('[data-testid="plugin-bot-account-problem"]').exists()).toBe(true)
  })

  it('keeps the account in plugin page links only for management pages', () => {
    const bot = { source_protocol: 'qqofficial' as const, bot_id: 'bot:id' }
    expect(buildPluginDetailLocation('demo', { panel: 'management-ui', managementPage: 'records', bot }).query).toEqual({ panel: 'management-ui', management_page: 'records', bot: 'qqofficial:bot:id' })
    expect(buildPluginDetailLocation('demo', { panel: 'overview', bot }).query).toEqual({ panel: 'overview' })
    expect(readPluginBotAccount({ bot: 'qqofficial:bot:id' })).toEqual({ present: true, account: bot })
    expect(readPluginBotAccount({ bot: ['onebot11:1', 'onebot11:2'] })).toEqual({ present: true, account: null })
    expect(readPluginBotAccount({})).toEqual({ present: false, account: null })
  })
})
