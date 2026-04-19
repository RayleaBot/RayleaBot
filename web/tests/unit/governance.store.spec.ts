import { createPinia, setActivePinia } from 'pinia'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useGovernanceStore } from '@/stores/governance'

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

describe('governance store', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('loads blacklist and command policy together', async () => {
    vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/governance/blacklist')) {
        return Promise.resolve(jsonResponse({
          user_entries: [{ entry_type: 'user', target_id: '10001', reason: 'spam', created_at: '2026-04-17T09:00:00Z' }],
          group_entries: [{ entry_type: 'group', target_id: '20002', reason: 'risk', created_at: '2026-04-16T06:30:00Z' }],
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
    expect(result.commandPolicy?.commands[0]?.declared_permission).toBeNull()
    expect(store.blacklist?.group_entries[0]?.target_id).toBe('20002')
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
    expect(result.commandPolicy?.default_level).toBe('everyone')
    expect(store.blacklistError).toBe('读取黑名单失败')
    expect(store.commandPolicyError).toBeNull()
    expect(store.error).toBeNull()
  })

  it('keeps empty governance lists as valid data', async () => {
    vi.stubGlobal('fetch', vi.fn((input: RequestInfo | URL) => {
      const url = String(input)
      if (url.includes('/api/governance/blacklist')) {
        return Promise.resolve(jsonResponse({
          user_entries: [],
          group_entries: [],
        }))
      }

      return Promise.resolve(jsonResponse({
        default_level: 'everyone',
        cooldown: {
          user_command_rate_limit: '10/60s',
          group_command_rate_limit: '30/60s',
          cooldown_reply: false,
        },
        commands: [],
      }))
    }))

    const store = useGovernanceStore()
    await store.refresh()

    expect(store.blacklist?.user_entries).toEqual([])
    expect(store.blacklist?.group_entries).toEqual([])
    expect(store.commandPolicy?.commands).toEqual([])
    expect(store.hasData).toBe(true)
  })
})
