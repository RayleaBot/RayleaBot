import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import GovernancePage from '@/views/operations/GovernanceView.vue'
import { useGovernanceStore } from '@/stores/governance'

function createRouterForPage() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/governance', name: 'governance', component: GovernancePage },
      { path: '/commands', name: 'commands', component: { template: '<div>commands</div>' } },
      { path: '/config', name: 'config', component: { template: '<div>config</div>' } },
    ],
  })
}

describe('GovernancePage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    document.body.innerHTML = ''
  })

  it('renders governance summary and keeps local errors scoped to the current card', async () => {
    const router = createRouterForPage()
    await router.push('/governance')
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
    store.commandPolicy = {
      default_level: 'everyone',
      cooldown: {
        user_command_rate_limit: '10/60s',
        group_command_rate_limit: '30/60s',
        cooldown_reply: true,
      },
      commands: [],
    }
    store.blacklistError = '读取黑名单失败'

    vi.spyOn(store, 'refresh').mockResolvedValue({
      blacklist: store.blacklist,
      whitelist: store.whitelist,
      commandPolicy: store.commandPolicy,
    })

    const wrapper = mount(GovernancePage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.text()).toContain('权限策略')
    expect(wrapper.text()).toContain('治理总览')
    expect(wrapper.text()).toContain('所有成员')
    expect(wrapper.text()).toContain('10/60s')
    expect(wrapper.text()).toContain('30/60s')
    expect(wrapper.text()).toContain('前往配置')
    expect(wrapper.text()).toContain('查看指令中心')
    expect(wrapper.get('[data-testid="governance-blacklist-card"]').text()).toContain('读取黑名单失败')
    expect(wrapper.get('[data-testid="governance-whitelist-card"]').text()).not.toContain('读取黑名单失败')

    await wrapper.get('[data-testid="governance-open-config"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.name).toBe('config')

    await router.push('/governance')
    await flushPromises()
    await wrapper.get('[data-testid="governance-open-commands"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.name).toBe('commands')
  }, 15000)

  it('adds and removes blacklist and whitelist entries', async () => {
    const router = createRouterForPage()
    await router.push('/governance')
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

    vi.spyOn(store, 'refresh').mockResolvedValue({
      blacklist: store.blacklist,
      whitelist: store.whitelist,
      commandPolicy: store.commandPolicy,
    })
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
      store.blacklist = {
        user_entries: [],
        group_entries: [],
      }
      return store.blacklist
    })
    vi.spyOn(store, 'addWhitelistEntry').mockImplementation(async (payload) => {
      store.whitelist = {
        enabled: false,
        user_entries: [{
          ...payload,
          created_at: '2026-04-19T10:00:00Z',
        }],
        group_entries: [],
      }
      return store.whitelist
    })
    vi.spyOn(store, 'removeWhitelistEntry').mockImplementation(async () => {
      store.whitelist = {
        enabled: false,
        user_entries: [],
        group_entries: [],
      }
      return store.whitelist
    })

    const wrapper = mount(GovernancePage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    const blacklistForm = wrapper.get('[data-testid="governance-blacklist-user-form"]')
    await blacklistForm.findAll('input')[0]!.setValue('30003')
    await blacklistForm.findAll('input')[1]!.setValue('临时封禁')
    await wrapper.get('[data-testid="governance-blacklist-add-user"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="governance-blacklist-card"]').text()).toContain('30003')
    expect(wrapper.get('[data-testid="governance-blacklist-card"]').text()).toContain('临时封禁')

    await wrapper.get('[data-testid="governance-blacklist-card"]').get('button.ant-btn-link').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="governance-blacklist-card"]').text()).not.toContain('30003')

    const whitelistForm = wrapper.get('[data-testid="governance-whitelist-user-form"]')
    await whitelistForm.findAll('input')[0]!.setValue('30003')
    await whitelistForm.findAll('input')[1]!.setValue('临时放行')
    await wrapper.get('[data-testid="governance-whitelist-add-user"]').trigger('click')
    await flushPromises()

    expect(wrapper.get('[data-testid="governance-whitelist-card"]').text()).toContain('30003')
    expect(wrapper.get('[data-testid="governance-whitelist-card"]').text()).toContain('临时放行')

    await wrapper.get('[data-testid="governance-whitelist-card"]').get('button.ant-btn-link').trigger('click')
    await flushPromises()
    expect(wrapper.get('[data-testid="governance-whitelist-card"]').text()).not.toContain('30003')
  }, 15000)

  it('confirms empty whitelist enable and keeps the warning visible after enabling', async () => {
    const router = createRouterForPage()
    await router.push('/governance')
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

    vi.spyOn(store, 'refresh').mockResolvedValue({
      blacklist: store.blacklist,
      whitelist: store.whitelist,
      commandPolicy: store.commandPolicy,
    })
    vi.spyOn(store, 'setWhitelistEnabled').mockImplementation(async (enabled: boolean) => {
      store.whitelist = {
        enabled,
        user_entries: [],
        group_entries: [],
      }
      return store.whitelist
    })

    const wrapper = mount(GovernancePage, {
      attachTo: document.body,
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    const switchComponent = wrapper.findComponent({ name: 'ASwitch' })
    await switchComponent.vm.$emit('change', true)
    await flushPromises()

    expect(document.body.textContent ?? '').toContain('确认启用空白名单')
    expect(document.body.textContent ?? '').toContain('当前没有任何白名单条目')

    const modal = wrapper.findComponent({ name: 'AModal' })
    await modal.vm.$emit('ok')
    await flushPromises()

    expect(wrapper.text()).toContain('白名单已启用且当前为空')
    expect(wrapper.text()).toContain('除超级管理员外，所有命令都会被挡下')
  }, 15000)
})
