import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { notifySuccess } from '@/adapter/feedback'
import RateLimitInput from '@/components/config/RateLimitInput.vue'
import RateLimitsPage from '@/views/operations/RateLimitsView.vue'
import { useConfigStore } from '@/stores/config'
import { createConfigDocumentFixture } from './config-document.fixture'

vi.mock('@/adapter/feedback', () => ({
  notifySuccess: vi.fn(),
  useToastFeedback: vi.fn(),
}))

function getRateLimitInput(wrapper: ReturnType<typeof mount>, value: string) {
  const input = wrapper.findAllComponents(RateLimitInput).find(candidate => candidate.props('value') === value)
  expect(input).toBeDefined()
  return input!
}

describe('RateLimitsPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('submits rate limit fields', async () => {
    const store = useConfigStore()
    store.document = createConfigDocumentFixture()

    vi.spyOn(store, 'fetchConfig').mockResolvedValue(undefined)
    const saveSpy = vi.spyOn(store, 'saveConfig').mockImplementation(async (submittedConfig) => {
      store.document = submittedConfig
      return {
        config: submittedConfig,
        redacted_fields: [],
        restart_required: false,
        apply_effects: {
          applied_now: [
            'user.command_rate_limit',
            'group.command_rate_limit',
            'user.cooldown_reply',
            'message.rate_limit_per_plugin',
            'message.rate_limit_per_target',
          ],
          reloaded_now: [],
          restart_required_fields: [],
        },
      }
    })

    const wrapper = mount(RateLimitsPage, {
      global: {
        plugins: [Antd],
      },
    })

    await flushPromises()

    expect(wrapper.find('[data-testid="rate-limits-unsaved-status"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="rate-limits-save"]').attributes('disabled')).toBeDefined()

    await getRateLimitInput(wrapper, '10/60s').vm.$emit('update:value', '20/60s')
    await getRateLimitInput(wrapper, '30/60s').vm.$emit('update:value', '60/60s')
    await wrapper.getComponent({ name: 'ASwitch' }).vm.$emit('update:checked', false)
    await getRateLimitInput(wrapper, '20/10s').vm.$emit('update:value', '30/10s')
    await getRateLimitInput(wrapper, '5/5s').vm.$emit('update:value', '12/1m')
    await flushPromises()

    expect(wrapper.find('[data-testid="rate-limits-unsaved-status"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="rate-limits-save"]').attributes('disabled')).toBeUndefined()

    await wrapper.get('[data-testid="rate-limits-save"]').trigger('click')
    await flushPromises()

    expect(saveSpy).toHaveBeenCalledTimes(1)
    const submitted = saveSpy.mock.calls[0][0]
    expect(submitted.user.command_rate_limit).toBe('20/60s')
    expect(submitted.group.command_rate_limit).toBe('60/60s')
    expect(submitted.user.cooldown_reply).toBe(false)
    expect(submitted.message.rate_limit_per_plugin).toBe('30/10s')
    expect(submitted.message.rate_limit_per_target).toBe('12/1m')
    expect(wrapper.find('[data-testid="rate-limits-unsaved-status"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="rate-limits-save-status"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="rate-limits-save"]').attributes('disabled')).toBeDefined()
    expect(notifySuccess).toHaveBeenCalledTimes(1)
  })
})
