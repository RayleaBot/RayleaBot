import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { t } from '@/i18n'
import { useGovernanceStore } from '@/stores/governance'

function jsonResponse(body: unknown, status = 200) {
  if (body && typeof body === 'object') {
    const value = body as Record<string, unknown>
    if (Array.isArray(value.items)) body = { total: value.items.length, ...value }
    if (Array.isArray(value.user_entries) && Array.isArray(value.group_entries)) body = { total: value.user_entries.length + value.group_entries.length, entry_count: value.user_entries.length + value.group_entries.length, ...value }
  }
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('governance store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('loads blacklist, whitelist and command policy together', async () => {
    vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/governance/blacklist')) {
        return Promise.resolve(jsonResponse({
          user_entries: [{ scope: {"kind":"global","source_protocol":"onebot11","source_adapter":"","bot_id":""}, entry_type: 'user', target_id: '10001', reason: 'spam', created_at: '2026-04-17T09:00:00Z' }],
          group_entries: [{ scope: {"kind":"global","source_protocol":"onebot11","source_adapter":"","bot_id":""}, entry_type: 'group', target_id: '20002', reason: 'risk', created_at: '2026-04-16T06:30:00Z' }],
        }))
      }

      if (url.includes('/api/governance/whitelist')) {
        return Promise.resolve(jsonResponse({
          enabled: true,
          user_entries: [{ scope: {"kind":"global","source_protocol":"onebot11","source_adapter":"","bot_id":""}, entry_type: 'user', target_id: '10003', reason: 'ops', created_at: '2026-04-18T09:00:00Z' }],
          group_entries: [],
        }))
      }

      return Promise.resolve(jsonResponse({
        default_level: 'everyone',
        cooldown: {
          user_command_rate_limit: '10/60s',
          group_command_rate_limit: '30/60s',
          cooldown_reply: true,
        },
        commands: [{
          plugin_id: 'weather',
          plugin_name: 'Weather',
          command: 'weather',
          aliases: ['tq'],
          declared_permission: null,
          effective_permission: 'everyone',
          permission_source: 'default_level',
        }],
      }))
    }))

    const store = useGovernanceStore()
    const result = await store.refresh()

    expect(result.blacklist?.user_entries).toHaveLength(1)
    expect(result.whitelist?.enabled).toBe(true)
    expect(result.commandPolicy?.commands[0]?.declared_permission).toBeNull()
    expect(store.blacklist?.group_entries[0]?.target_id).toBe('20002')
    expect(store.whitelist?.user_entries[0]?.target_id).toBe('10003')
    expect(store.commandPolicy?.commands[0]?.effective_permission).toBe('everyone')
    expect(store.error).toBeNull()
  })

  it('keeps successful data when one governance request fails', async () => {
    vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/governance/blacklist')) {
        return Promise.resolve(jsonResponse({
          error: {
            code: 'platform.internal_error',
            message: '读取黑名单失败',
            request_id: 'req_governance_blacklist_failed',
          },
        }, 500))
      }

      if (url.includes('/api/governance/whitelist')) {
        return Promise.resolve(jsonResponse({
          enabled: false,
          user_entries: [],
          group_entries: [],
        }))
      }

      return Promise.resolve(jsonResponse({
        default_level: 'everyone',
        cooldown: {
          user_command_rate_limit: '10/60s',
          group_command_rate_limit: '30/60s',
          cooldown_reply: true,
        },
        commands: [],
      }))
    }))

    const store = useGovernanceStore()
    const result = await store.refresh()

    expect(result.blacklist).toBeNull()
    expect(result.whitelist?.enabled).toBe(false)
    expect(result.commandPolicy?.default_level).toBe('everyone')
    expect(store.blacklistError).toBe(t('errors.platform.internal_error'))
    expect(store.blacklistError).not.toBe('读取黑名单失败')
    expect(store.whitelistError).toBeNull()
    expect(store.commandPolicyError).toBeNull()
    expect(store.error).toBeNull()
  })

  it('writes governance entries and refreshes the latest snapshot', async () => {
    const state = {
      blacklist: {
        user_entries: [] as Array<Record<string, string>>,
        group_entries: [] as Array<Record<string, string>>,
      },
      whitelist: {
        enabled: false,
        user_entries: [] as Array<Record<string, string>>,
        group_entries: [] as Array<Record<string, string>>,
      },
    }

    vi.stubGlobal('fetch', vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
      const url = String(input)
      const method = init?.method ?? 'GET'

      if (url.includes('/api/governance/blacklist/entries') && method === 'POST') {
        state.blacklist.user_entries = [{ scope: {"kind":"global","source_protocol":"onebot11","source_adapter":"","bot_id":""},
          entry_type: 'user',
          target_id: '10001',
          reason: 'spam',
          created_at: '2026-04-17T09:00:00Z',
        }]
        return jsonResponse(state.blacklist.user_entries[0])
      }

      if (url.includes('/api/governance/blacklist/entries/user/10001?kind=global&source_protocol=onebot11&source_adapter=&bot_id=') && method === 'DELETE') {
        state.blacklist.user_entries = []
        return new Response(null, { status: 204 })
      }

      if (url.includes('/api/governance/whitelist/state') && method === 'PUT') {
        state.whitelist.enabled = true
        return jsonResponse({ enabled: true })
      }

      if (url.includes('/api/governance/whitelist/entries') && method === 'POST') {
        state.whitelist.group_entries = [{ scope: {"kind":"global","source_protocol":"onebot11","source_adapter":"","bot_id":""},
          entry_type: 'group',
          target_id: '20002',
          reason: 'ops',
          created_at: '2026-04-18T09:00:00Z',
        }]
        return jsonResponse(state.whitelist.group_entries[0])
      }

      if (url.includes('/api/governance/whitelist/entries/group/20002?kind=global&source_protocol=onebot11&source_adapter=&bot_id=') && method === 'DELETE') {
        state.whitelist.group_entries = []
        return new Response(null, { status: 204 })
      }

      if (url.includes('/api/governance/blacklist')) {
        return jsonResponse(state.blacklist)
      }

      if (url.includes('/api/governance/whitelist')) {
        return jsonResponse(state.whitelist)
      }

      return jsonResponse({
        default_level: 'everyone',
        cooldown: {
          user_command_rate_limit: '10/60s',
          group_command_rate_limit: '30/60s',
          cooldown_reply: true,
        },
        commands: [],
      })
    }))

    const store = useGovernanceStore()
    await store.refresh()

    await store.addBlacklistEntry({ scope: {"kind":"global","source_protocol":"onebot11","source_adapter":"","bot_id":""},
      entry_type: 'user',
      target_id: '10001',
      reason: 'spam',
    })
    expect(store.blacklist?.user_entries[0]?.target_id).toBe('10001')

    await store.setWhitelistEnabled(true)
    expect(store.whitelist?.enabled).toBe(true)

    await store.addWhitelistEntry({ scope: {"kind":"global","source_protocol":"onebot11","source_adapter":"","bot_id":""},
      entry_type: 'group',
      target_id: '20002',
      reason: 'ops',
    })
    expect(store.whitelist?.group_entries[0]?.target_id).toBe('20002')

    await store.removeBlacklistEntry('user', '10001', {"kind":"global","source_protocol":"onebot11","source_adapter":"","bot_id":""})
    expect(store.blacklist?.user_entries).toEqual([])

    await store.removeWhitelistEntry('group', '20002', {"kind":"global","source_protocol":"onebot11","source_adapter":"","bot_id":""})
    expect(store.whitelist?.group_entries).toEqual([])
  })
})
