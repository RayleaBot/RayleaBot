import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import UpdateSettingsStatus from '@/components/config/UpdateSettingsStatus.vue'
import { apiRequest } from '@/lib/http'
import AppButton from '@/components/AppButton.vue'
import { t } from '@/i18n'

vi.mock('@/lib/http', async importOriginal => ({ ...await importOriginal<object>(), apiRequest: vi.fn() }))

function action(wrapper: ReturnType<typeof mount>, key: 'refresh' | 'check') {
  return wrapper.findAllComponents(AppButton).find(button => button.text() === t(`config.update.${key}`))!
}

describe('update settings', () => {
  beforeEach(() => {
    vi.mocked(apiRequest).mockReset()
    vi.mocked(apiRequest).mockResolvedValue({ state: 'idle', current_version: '0.4.0', checked_at: null, update_mode: 'guided' })
  })

  it('prevents route checks and release refreshes from using unsaved settings', async () => {
    const wrapper = mount(UpdateSettingsStatus, { props: { dirty: true, disabled: false } })
    await flushPromises()
    for (const key of ['refresh', 'check'] as const) {
      expect(action(wrapper, key).attributes('disabled')).toBeDefined()
    }
  })

  it('passes server release choices to the configuration editor', async () => {
    const wrapper = mount(UpdateSettingsStatus, { props: { dirty: false, disabled: false } })
    await flushPromises()
    const releases = [{ version: '0.5.0-beta.1', channel: 'beta', release_notes_ref: 'https://example.com/release' }]
    vi.mocked(apiRequest).mockResolvedValueOnce({ releases })
    await action(wrapper, 'refresh').trigger('click')
    await flushPromises()
    expect(vi.mocked(apiRequest).mock.lastCall?.[0]).toBe('/api/update/releases')
    expect(wrapper.emitted('releases')?.[0]?.[0]).toEqual(releases)
  })

  it('renders route observations from the shared server check', async () => {
    const wrapper = mount(UpdateSettingsStatus, { props: { dirty: false, disabled: false } })
    await flushPromises()
    vi.mocked(apiRequest).mockResolvedValueOnce({ state: 'update_available', current_version: '0.4.0', available_version: '0.5.0', checked_at: null, update_mode: 'guided', routes: [{ url: 'https://proxy.example/release', available: true, selected: true, latency_ms: 120 }] })
    await action(wrapper, 'check').trigger('click')
    await flushPromises()
    expect(wrapper.get('[title="https://proxy.example/release"]').text()).toBe('proxy.example')
    expect(wrapper.get('tbody').text()).toContain('120 ms')
  })
})
