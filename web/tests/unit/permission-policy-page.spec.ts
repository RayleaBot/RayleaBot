import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { notifySuccess } from '@/adapter/feedback'
import { t } from '@/i18n'
import PermissionPolicyPage from '@/views/operations/PermissionPolicyView.vue'
import { useConfigStore } from '@/stores/config'
import { useGovernanceStore } from '@/stores/governance'
import { createConfigDocumentFixture } from './config-document.fixture'

vi.mock('@/adapter/feedback', () => ({
  notifySuccess: vi.fn(),
  useToastFeedback: vi.fn(),
}))

function createFixtureConfig() {
  return createConfigDocumentFixture((config) => {
    config.admin.super_admins = ['10001']
  })
}

function createRouterForPage() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [
      { path: '/permission-policy', name: 'permission-policy', component: PermissionPolicyPage },
      { path: '/access-lists', name: 'access-lists', component: { template: '<div>access lists</div>' } },
    ],
  })
}

function getDefaultLevelSelect(wrapper: ReturnType<typeof mount>) {
  const select = wrapper.findAllComponents({ name: 'ASelect' }).find((candidate) => {
    const options = candidate.props('options') as Array<{ value?: unknown }> | undefined
    return options?.some(option => option.value === 'group_admin')
  })
  expect(select).toBeDefined()
  return select!
}

function getSuperAdminSelect(wrapper: ReturnType<typeof mount>) {
  const select = wrapper.findAllComponents({ name: 'ASelect' }).find(
    candidate => candidate.attributes('data-testid') === 'permission-policy-super-admins',
  )
  expect(select).toBeDefined()
  return select!
}

describe('PermissionPolicyPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    document.body.innerHTML = ''
    vi.clearAllMocks()
  })

  it('loads policy settings with concise field guidance', async () => {
    const router = createRouterForPage()
    await router.push('/permission-policy')
    await router.isReady()

    const configStore = useConfigStore()
    const governanceStore = useGovernanceStore()
    configStore.document = createFixtureConfig()
    governanceStore.commandPolicy = {
      default_level: 'everyone',
      cooldown: {
        user_command_rate_limit: '10/60s',
        group_command_rate_limit: '30/60s',
        cooldown_reply: true,
      },
      commands: [],
    }

    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined)
    vi.spyOn(governanceStore, 'fetchCommandPolicy').mockResolvedValue(governanceStore.commandPolicy)

    const wrapper = mount(PermissionPolicyPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.find(`[aria-label="${t('rateLimits.fields.cooldownReply')}"]`).exists()).toBe(false)
    expect(wrapper.find('[data-testid="permission-policy-summary-card"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="permission-policy-super-admins"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="permission-policy-save"]').attributes('disabled')).toBeDefined()

    await wrapper.get('[data-testid="permission-policy-open-access-lists"]').trigger('click')
    await flushPromises()
    expect(router.currentRoute.value.name).toBe('access-lists')
  }, 15000)

  it('shows the retry state when config is unavailable even if the policy summary loaded', async () => {
    const router = createRouterForPage()
    await router.push('/permission-policy')
    await router.isReady()

    const configStore = useConfigStore()
    const governanceStore = useGovernanceStore()
    configStore.error = '无法读取权限配置'
    governanceStore.commandPolicy = {
      default_level: 'everyone',
      cooldown: {
        user_command_rate_limit: '10/60s',
        group_command_rate_limit: '30/60s',
        cooldown_reply: true,
      },
      commands: [],
    }

    vi.spyOn(configStore, 'fetchConfig').mockRejectedValue(new Error('fixture config failure'))
    vi.spyOn(governanceStore, 'fetchCommandPolicy').mockResolvedValue(governanceStore.commandPolicy)

    const wrapper = mount(PermissionPolicyPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    expect(wrapper.find('.retry-panel').exists()).toBe(true)
    expect(wrapper.find('.permission-policy-settings-section').exists()).toBe(false)
  }, 15000)

  it('submits policy fields and refreshes command policy after saving', async () => {
    const router = createRouterForPage()
    await router.push('/permission-policy')
    await router.isReady()

    const configStore = useConfigStore()
    const governanceStore = useGovernanceStore()
    const config = createFixtureConfig()
    configStore.document = config
    governanceStore.commandPolicy = {
      default_level: 'everyone',
      cooldown: {
        user_command_rate_limit: '10/60s',
        group_command_rate_limit: '30/60s',
        cooldown_reply: true,
      },
      commands: [],
    }

    vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined)
    const fetchPolicySpy = vi.spyOn(governanceStore, 'fetchCommandPolicy').mockResolvedValue(governanceStore.commandPolicy)
    const saveSpy = vi.spyOn(configStore, 'saveConfig').mockImplementation(async (submittedConfig) => {
      configStore.document = submittedConfig
      return {
        config: submittedConfig,
        redacted_fields: [],
        restart_required: false,
        apply_effects: {
          applied_now: ['permission.default_level'],
          reloaded_now: [],
          restart_required_fields: [],
        },
      }
    })

    const wrapper = mount(PermissionPolicyPage, {
      global: {
        plugins: [Antd, router],
      },
    })

    await flushPromises()

    await getSuperAdminSelect(wrapper).vm.$emit(
      'update:value',
      ['10001', '10002'],
    )
    await getDefaultLevelSelect(wrapper).vm.$emit('update:value', 'group_admin')
    await flushPromises()

    expect(wrapper.find('[data-testid="permission-policy-unsaved-status"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="permission-policy-save"]').attributes('disabled')).toBeUndefined()

    await wrapper.get('[data-testid="permission-policy-save"]').trigger('click')
    await flushPromises()

    expect(saveSpy).toHaveBeenCalledTimes(1)
    const submitted = saveSpy.mock.calls[0][0]
    expect(submitted.admin.super_admins).toEqual(['10001', '10002'])
    expect(submitted.permission.default_level).toBe('group_admin')
    expect(submitted.user.command_rate_limit).toBe('10/60s')
    expect(submitted.group.command_rate_limit).toBe('30/60s')
    expect(submitted.user.cooldown_reply).toBe(true)
    expect(wrapper.find('[data-testid="permission-policy-unsaved-status"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="permission-policy-save-status"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="permission-policy-save"]').attributes('disabled')).toBeDefined()
    expect(fetchPolicySpy).toHaveBeenCalledTimes(2)
    expect(notifySuccess).toHaveBeenCalledTimes(1)
  }, 15000)
})
