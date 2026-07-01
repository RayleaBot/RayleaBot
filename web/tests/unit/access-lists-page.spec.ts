import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { notifySuccess, useToastFeedback } from '@/adapter/feedback'
import AccessListsPage from '@/views/operations/AccessListsView.vue'
import { useGovernanceStore } from '@/stores/governance'

vi.mock('@/adapter/feedback', () => ({
  notifySuccess: vi.fn(),
  useToastFeedback: vi.fn(),
}))

function createRouterForPage() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/access-lists', name: 'access-lists', component: AccessListsPage },
      { path: '/commands', name: 'commands', component: { template: '<div>commands</div>' } },
      { path: '/config', name: 'config', component: { template: '<div>config</div>' } },
    ],
  })
}

function buildEntries(
  count: number,
  entryType: 'user' | 'group',
  startId: number,
  reasonPrefix: string,
) {
  return Array.from({ length: count }, (_, index) => ({
    entry_type: entryType,
    target_id: String(startId + index),
    reason: `${reasonPrefix}${index + 1}`,
    created_at: new Date(Date.UTC(2026, 3, 18, 8, index, 0)).toISOString(),
  }))
}

function mockAccessListFetches(store: ReturnType<typeof useGovernanceStore>) {
  vi.spyOn(store, 'fetchBlacklist').mockResolvedValue(store.blacklist!)
  vi.spyOn(store, 'fetchWhitelist').mockResolvedValue(store.whitelist!)
}

function toastMessages() {
  return vi.mocked(useToastFeedback).mock.calls
    .map(([source]) => {
      if (typeof source === 'function') {
        return source()?.message
      }
      return source.value?.message
    })
    .filter((message): message is string => Boolean(message))
}

describe('AccessListsPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    document.body.innerHTML = ''
    vi.clearAllMocks()
  })

  it('renders access lists and keeps local errors scoped to the current card', async () => {
    const router = createRouterForPage()
    await router.push('/access-lists')
    await router.isReady()

    const store = useGovernanceStore()
    store.blacklist = {
      user_entries: [
        {
          entry_type: 'user',
          target_id: '10001',
          reason: '反复刷屏',
          created_at: '2026-04-17T09:00:00Z',
        },
      ],
      group_entries: [],
    }
    store.whitelist = {
      enabled: true,
      user_entries: [],
      group_entries: [
        {
          entry_type: 'group',
          target_id: '20002',
          reason: '核心值守群',
          created_at: '2026-04-18T09:00:00Z',
        },
      ],
    }
    store.blacklistError = '读取黑名单失败'

    mockAccessListFetches(store)

    const wrapper = mount(AccessListsPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('黑白名单')
    expect(wrapper.get('[data-testid="access-lists-whitelist-card"]').text()).toContain('白名单')
    expect(wrapper.get('[data-testid="access-lists-whitelist-card"]').text()).not.toContain('命中白名单的用户或群')
    expect(wrapper.text()).toContain('查看指令中心')
    expect(wrapper.get('[data-testid="access-lists-whitelist-card"]').text()).not.toContain('读取黑名单失败')

    expect(wrapper.get('[data-testid="access-lists-blacklist-card"]').text()).toContain('黑名单')
    expect(wrapper.get('[data-testid="access-lists-blacklist-card"]').text()).not.toContain('命中黑名单的用户或群')
    expect(wrapper.get('[data-testid="access-lists-blacklist-card"]').text()).not.toContain('读取黑名单失败')
    expect(toastMessages()).toContain('读取黑名单失败')

    await wrapper.get('[data-testid="access-lists-open-commands"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.name).toBe('commands')
  }, 15000)

  it('adds and removes whitelist and blacklist entries through modal and popconfirm', async () => {
    const router = createRouterForPage()
    await router.push('/access-lists')
    await router.isReady()

    const store = useGovernanceStore()
    store.blacklist = {
      user_entries: [],
      group_entries: [],
    }
    store.whitelist = {
      enabled: false,
      user_entries: [],
      group_entries: [],
    }
    store.commandPolicy = {
      default_level: 'everyone',
      cooldown: {
        user_command_rate_limit: '10/60s',
        group_command_rate_limit: '30/60s',
        cooldown_reply: true,
      },
      commands: [],
    }

    mockAccessListFetches(store)
    vi.spyOn(store, 'addBlacklistEntry').mockImplementation(async (payload) => {
      const entry = { ...payload, created_at: '2026-04-19T09:00:00Z' }
      if (payload.entry_type === 'group') {
        store.blacklist = {
          user_entries: store.blacklist.user_entries,
          group_entries: [...store.blacklist.group_entries, entry],
        }
      } else {
        store.blacklist = {
          user_entries: [...store.blacklist.user_entries, entry],
          group_entries: store.blacklist.group_entries,
        }
      }
      return store.blacklist
    })
    vi.spyOn(store, 'removeBlacklistEntry').mockImplementation(async () => {
      store.blacklist = {
        user_entries: [],
        group_entries: [],
      }
      return store.blacklist
    })
    vi.spyOn(store, 'addWhitelistEntry').mockImplementation(async (payload) => {
      const entry = { ...payload, created_at: '2026-04-19T10:00:00Z' }
      if (payload.entry_type === 'group') {
        store.whitelist = {
          enabled: store.whitelist.enabled,
          user_entries: store.whitelist.user_entries,
          group_entries: [...store.whitelist.group_entries, entry],
        }
      } else {
        store.whitelist = {
          enabled: store.whitelist.enabled,
          user_entries: [...store.whitelist.user_entries, entry],
          group_entries: store.whitelist.group_entries,
        }
      }
      return store.whitelist
    })
    vi.spyOn(store, 'removeWhitelistEntry').mockImplementation(async () => {
      store.whitelist = {
        enabled: store.whitelist.enabled,
        user_entries: [],
        group_entries: [],
      }
      return store.whitelist
    })

    const wrapper = mount(AccessListsPage, {
      attachTo: document.body,
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    // --- Whitelist: add via inline table edit ---
    // Default entry_type follows scopeFilter ('all' -> 'user')
    await wrapper.get('[data-testid="access-lists-whitelist-add-btn"]').trigger('click')
    await flushPromises()

    expect(wrapper.vm.isAddingWhitelist).toBe(true)
    expect(wrapper.vm.whitelistDraft.entry_type).toBe('user')

    await wrapper.get('[data-testid="whitelist-draft-target-id"]').setValue('30003')
    await wrapper.get('[data-testid="whitelist-draft-reason"]').setValue('临时放行')
    await flushPromises()

    await wrapper.get('[data-testid="whitelist-draft-save"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="access-lists-whitelist-card"]').text()).toContain('30003')
    expect(wrapper.get('[data-testid="access-lists-whitelist-card"]').text()).toContain('临时放行')

    // Remove via popconfirm confirm
    const whitelistPopconfirm = wrapper.get('[data-testid="access-lists-whitelist-card"]').findComponent({ name: 'APopconfirm' })
    await whitelistPopconfirm.vm.$emit('confirm')
    await flushPromises()

    expect(wrapper.get('[data-testid="access-lists-whitelist-card"]').text()).not.toContain('30003')

    // --- Blacklist (always visible): add via inline table edit ---
    wrapper.vm.blacklistScopeFilter = 'group'
    await flushPromises()

    await wrapper.get('[data-testid="access-lists-blacklist-add-btn"]').trigger('click')
    await flushPromises()

    expect(wrapper.vm.isAddingBlacklist).toBe(true)
    expect(wrapper.vm.blacklistDraft.entry_type).toBe('group')

    await wrapper.get('[data-testid="blacklist-draft-target-id"]').setValue('30003')
    await wrapper.get('[data-testid="blacklist-draft-reason"]').setValue('临时封禁')
    await flushPromises()

    await wrapper.get('[data-testid="blacklist-draft-save"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="access-lists-blacklist-card"]').text()).toContain('30003')
    expect(wrapper.get('[data-testid="access-lists-blacklist-card"]').text()).toContain('临时封禁')

    // Remove via popconfirm confirm
    const blacklistPopconfirm = wrapper.get('[data-testid="access-lists-blacklist-card"]').findComponent({ name: 'APopconfirm' })
    await blacklistPopconfirm.vm.$emit('confirm')
    await flushPromises()

    expect(wrapper.get('[data-testid="access-lists-blacklist-card"]').text()).not.toContain('30003')
  }, 15000)

  it('shows complete filtered lists without pagination controls', async () => {
    const router = createRouterForPage()
    await router.push('/access-lists')
    await router.isReady()

    const store = useGovernanceStore()
    store.blacklist = {
      user_entries: buildEntries(11, 'user', 50001, '黑名单用户'),
      group_entries: [
        {
          entry_type: 'group',
          target_id: '60001',
          reason: '黑名单群组',
          created_at: '2026-04-18T20:00:00Z',
        },
      ],
    }
    store.whitelist = {
      enabled: false,
      user_entries: buildEntries(12, 'user', 10001, '白名单用户'),
      group_entries: [
        {
          entry_type: 'group',
          target_id: '20002',
          reason: '核心服务群',
          created_at: '2026-04-18T21:00:00Z',
        },
      ],
    }
    store.commandPolicy = {
      default_level: 'everyone',
      cooldown: {
        user_command_rate_limit: '10/60s',
        group_command_rate_limit: '30/60s',
        cooldown_reply: true,
      },
      commands: [],
    }

    mockAccessListFetches(store)

    const wrapper = mount(AccessListsPage, {
      attachTo: document.body,
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    const whitelistCard = wrapper.get('[data-testid="access-lists-whitelist-card"]')
    expect(whitelistCard.text()).toContain('10001')
    expect(whitelistCard.text()).toContain('10012')
    expect(whitelistCard.text()).toContain('20002')
    expect(whitelistCard.findAll('.ant-table-tbody > tr')).toHaveLength(13)
    expect(whitelistCard.find('.ant-pagination').exists()).toBe(false)

    const blacklistCard = wrapper.get('[data-testid="access-lists-blacklist-card"]')
    expect(blacklistCard.text()).toContain('50001')
    expect(blacklistCard.text()).toContain('50011')
    expect(blacklistCard.text()).toContain('60001')
    expect(blacklistCard.findAll('.ant-table-tbody > tr')).toHaveLength(12)
    expect(blacklistCard.find('.ant-pagination').exists()).toBe(false)
  }, 15000)

  it('clears region error after a successful add', async () => {
    const router = createRouterForPage()
    await router.push('/access-lists')
    await router.isReady()

    const store = useGovernanceStore()
    store.blacklist = {
      user_entries: [],
      group_entries: [],
    }
    store.whitelist = {
      enabled: false,
      user_entries: [],
      group_entries: [],
    }
    store.commandPolicy = {
      default_level: 'everyone',
      cooldown: {
        user_command_rate_limit: '10/60s',
        group_command_rate_limit: '30/60s',
        cooldown_reply: true,
      },
      commands: [],
    }

    mockAccessListFetches(store)
    vi.spyOn(store, 'addBlacklistEntry').mockImplementation(async (payload) => {
      store.blacklist = {
        user_entries: [{
          ...payload,
          created_at: '2026-04-19T09:00:00Z',
        }],
        group_entries: [],
      }
      return store.blacklist
    })
    vi.spyOn(store, 'removeBlacklistEntry').mockImplementation(async () => {
      throw new Error('删除失败')
    })

    const wrapper = mount(AccessListsPage, {
      attachTo: document.body,
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    // Trigger a remove error
    const blacklistCard = wrapper.get('[data-testid="access-lists-blacklist-card"]')
    expect(blacklistCard.text()).not.toContain('删除失败')

    // We need an entry to remove; add one first
    await wrapper.get('[data-testid="access-lists-blacklist-add-btn"]').trigger('click')
    await flushPromises()

    await wrapper.get('[data-testid="blacklist-draft-target-id"]').setValue('10001')
    await wrapper.get('[data-testid="blacklist-draft-reason"]').setValue('测试')
    await flushPromises()

    await wrapper.get('[data-testid="blacklist-draft-save"]').trigger('click')
    await flushPromises()

    expect(blacklistCard.text()).toContain('10001')

    // Now remove it; it should fail and leave an error
    const popconfirm = blacklistCard.findComponent({ name: 'APopconfirm' })
    await popconfirm.vm.$emit('confirm')
    await flushPromises()

    expect(wrapper.vm.blacklistActionError).toBe('删除失败')

    // Add another entry successfully; the old error should be cleared
    await wrapper.get('[data-testid="access-lists-blacklist-add-btn"]').trigger('click')
    await flushPromises()

    await wrapper.get('[data-testid="blacklist-draft-target-id"]').setValue('10002')
    await wrapper.get('[data-testid="blacklist-draft-reason"]').setValue('测试2')
    await flushPromises()

    await wrapper.get('[data-testid="blacklist-draft-save"]').trigger('click')
    await flushPromises()

    expect(wrapper.vm.blacklistActionError).toBeNull()
    expect(blacklistCard.text()).toContain('10002')
  }, 15000)

  it('confirms empty whitelist enable and keeps the warning visible after enabling', async () => {
    const router = createRouterForPage()
    await router.push('/access-lists')
    await router.isReady()

    const store = useGovernanceStore()
    store.blacklist = {
      user_entries: [],
      group_entries: [],
    }
    store.whitelist = {
      enabled: false,
      user_entries: [],
      group_entries: [],
    }
    store.commandPolicy = {
      default_level: 'everyone',
      cooldown: {
        user_command_rate_limit: '10/60s',
        group_command_rate_limit: '30/60s',
        cooldown_reply: true,
      },
      commands: [],
    }

    mockAccessListFetches(store)
    vi.spyOn(store, 'setWhitelistEnabled').mockImplementation(async (enabled: boolean) => {
      store.whitelist = {
        enabled,
        user_entries: [],
        group_entries: [],
      }
      return store.whitelist
    })

    const wrapper = mount(AccessListsPage, {
      attachTo: document.body,
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    // Click the whitelist enable switch
    await wrapper.get('[data-testid="access-lists-whitelist-enabled"]').trigger('click')
    await flushPromises()

    expect(document.body.textContent ?? '').toContain('确认启用空白名单')
    expect(document.body.textContent ?? '').toContain('当前没有任何白名单条目')

    // Emit ok on the confirm modal
    const confirmModal = wrapper.findAllComponents({ name: 'AModal' }).find(m => m.props('open') === true)
    expect(confirmModal).toBeDefined()
    await confirmModal!.vm.$emit('ok')
    await flushPromises()

    expect(wrapper.text()).not.toContain('白名单已启用且当前为空')
    expect(wrapper.text()).not.toContain('除超级管理员外，所有命令都会被挡下')
    expect(toastMessages()).toContain('白名单已启用且当前为空：除超级管理员外，所有命令都会被挡下。请尽快补充条目，或先关闭白名单。')
  }, 15000)

  it('copies the target id and keeps the existing success feedback', async () => {
    const router = createRouterForPage()
    await router.push('/access-lists')
    await router.isReady()

    const store = useGovernanceStore()
    store.blacklist = {
      user_entries: [],
      group_entries: [],
    }
    store.whitelist = {
      enabled: true,
      user_entries: [
        {
          entry_type: 'user',
          target_id: '91001',
          reason: '值班账号',
          created_at: '2026-04-18T08:30:00Z',
        },
      ],
      group_entries: [],
    }
    store.commandPolicy = {
      default_level: 'everyone',
      cooldown: {
        user_command_rate_limit: '10/60s',
        group_command_rate_limit: '30/60s',
        cooldown_reply: true,
      },
      commands: [],
    }

    mockAccessListFetches(store)

    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.defineProperty(navigator, 'clipboard', {
      configurable: true,
      value: { writeText },
    })

    const wrapper = mount(AccessListsPage, {
      attachTo: document.body,
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    await wrapper.get('[data-testid="access-lists-whitelist-card"] .copyable-text').trigger('click')
    await flushPromises()

    expect(writeText).toHaveBeenCalledWith('91001')
    expect(notifySuccess).toHaveBeenCalledTimes(1)
  }, 15000)
})
