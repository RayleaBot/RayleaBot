import Antd from 'ant-design-vue'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { notifySuccess } from '@/adapter/feedback'
import RateLimitInput from '@/components/config/RateLimitInput.vue'
import { t } from '@/i18n'
import PluginSettingsPage from '@/views/plugins/PluginSettingsView.vue'
import { useConfigStore } from '@/stores/config'
import { createConfigDocumentFixture } from './config-document.fixture'

vi.mock('@/adapter/feedback', () => ({
  notifySuccess: vi.fn(),
  useToastFeedback: vi.fn(),
}))

function getCommandPrefixSelect(wrapper: ReturnType<typeof mount>) {
  const select = wrapper.findAllComponents({ name: 'ASelect' }).find(
    candidate => candidate.attributes('data-testid') === 'plugin-settings-command-prefixes',
  )
  expect(select).toBeDefined()
  return select!
}

describe('PluginSettingsPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.clearAllMocks()
  })

  it('submits plugin-facing config fields', async () => {
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
            'command.prefixes',
            'log.rate_limit_per_plugin',
            'render.footer_template',
            'storage.plugin_workdir_soft_limit_mb',
          ],
          reloaded_now: [],
          restart_required_fields: [],
        },
      }
    })

    const wrapper = mount(PluginSettingsPage, {
      global: {
        plugins: [Antd],
      },
    })

    await flushPromises()

    expect(wrapper.find('[data-testid="plugin-settings-unsaved-status"]').exists()).toBe(false)
    expect(wrapper.get('[data-testid="plugin-settings-save"]').attributes('disabled')).toBeDefined()

    await getCommandPrefixSelect(wrapper).vm.$emit(
      'update:value',
      ['/', '!'],
    )
    await wrapper.getComponent(RateLimitInput).vm.$emit('update:value', '300/10s')
    await wrapper.get('textarea').setValue('Footer {{plugin_name}}')
    await wrapper.get(`[aria-label="${t('config.fields.storagePluginWorkdirSoftLimitMb')}"]`).setValue('512')
    await flushPromises()

    store.document = JSON.parse(JSON.stringify(store.document))
    await flushPromises()

    expect(wrapper.find('[data-testid="plugin-settings-unsaved-status"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="plugin-settings-save"]').attributes('disabled')).toBeUndefined()

    await wrapper.get('[data-testid="plugin-settings-save"]').trigger('click')
    await flushPromises()

    expect(saveSpy).toHaveBeenCalledTimes(1)
    const submitted = saveSpy.mock.calls[0][0]
    expect(submitted.command.prefixes).toEqual(['/', '!'])
    expect(submitted.log.rate_limit_per_plugin).toBe('300/10s')
    expect(submitted.render.footer_template).toBe('Footer {{plugin_name}}')
    expect(submitted.message.rate_limit_per_plugin).toBe('20/10s')
    expect(submitted.storage.plugin_workdir_soft_limit_mb).toBe(512)
    expect(submitted.server.host).toBe('127.0.0.1')
    expect(wrapper.find('[data-testid="plugin-settings-unsaved-status"]').exists()).toBe(false)
    expect(wrapper.find('[data-testid="plugin-settings-save-status"]').exists()).toBe(true)
    expect(wrapper.get('[data-testid="plugin-settings-save"]').attributes('disabled')).toBeDefined()
    expect(notifySuccess).toHaveBeenCalledTimes(1)
  })

  it('restores render footer template to the default value', async () => {
    const store = useConfigStore()
    const fixture = createConfigDocumentFixture()
    fixture.render.footer_template = 'Custom footer'
    store.document = fixture

    vi.spyOn(store, 'fetchConfig').mockResolvedValue(undefined)

    const wrapper = mount(PluginSettingsPage, {
      global: {
        plugins: [Antd],
      },
    })

    await flushPromises()

    const footerInput = wrapper.get('textarea')
    expect((footerInput.element as HTMLTextAreaElement).value).toBe('Custom footer')
    await wrapper.get('[data-testid="plugin-settings-reset-default"]').trigger('click')
    await flushPromises()

    expect(wrapper.find('[data-testid="plugin-settings-unsaved-status"]').exists()).toBe(true)
    expect((footerInput.element as HTMLTextAreaElement).value).toBe(
      'Created By RayleaBot {{rayleabot_version}} & Plugin {{plugin_name}} {{plugin_version}}',
    )
  })
})
