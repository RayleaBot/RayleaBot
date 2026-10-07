import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import UpdateSettingsStatus from '@/components/config/UpdateSettingsStatus.vue'
import { apiRequest } from '@/lib/http'

vi.mock('@/lib/http', async importOriginal => ({ ...await importOriginal<object>(), apiRequest: vi.fn() }))

describe('update settings', () => {
  beforeEach(() => {
    vi.mocked(apiRequest).mockReset()
    vi.mocked(apiRequest).mockResolvedValue({ state: 'idle', current_version: '0.4.0', checked_at: null, update_mode: 'guided' })
  })

  it('prevents route checks and release refreshes from using unsaved settings', async () => {
    const wrapper = mount(UpdateSettingsStatus, { props: { dirty: true, disabled: false } })
    await flushPromises()
    for (const button of wrapper.findAll('button')) expect(button.element.disabled).toBe(true)
    expect(apiRequest).toHaveBeenCalledTimes(1)
    expect(apiRequest).toHaveBeenCalledWith('/api/update/status')
  })

  it('passes server release choices to the configuration editor', async () => {
    const wrapper = mount(UpdateSettingsStatus, { props: { dirty: false, disabled: false } })
    await flushPromises()
    const releases = [{ version: '0.5.0-beta.1', channel: 'beta', release_notes_ref: 'https://example.com/release' }]
    vi.mocked(apiRequest).mockResolvedValueOnce({ releases })
    await wrapper.findAll('button')[0]!.trigger('click')
    await flushPromises()
    expect(apiRequest).toHaveBeenLastCalledWith('/api/update/releases', { timeoutMs: 25000 })
    expect(wrapper.emitted('releases')?.[0]?.[0]).toEqual(releases)
  })

  it('renders route observations from the shared server check', async () => {
    const wrapper = mount(UpdateSettingsStatus, { props: { dirty: false, disabled: false } })
    await flushPromises()
    vi.mocked(apiRequest).mockResolvedValueOnce({ state: 'update_available', current_version: '0.4.0', available_version: '0.5.0', checked_at: null, update_mode: 'guided', routes: [{ url: 'https://proxy.example/release', available: true, selected: true, latency_ms: 120 }] })
    await wrapper.findAll('button')[1]!.trigger('click')
    await flushPromises()
    expect(wrapper.get('tbody').text()).toContain('https://proxy.example/release')
    expect(wrapper.get('tbody').text()).toContain('120 ms')
  })
})
