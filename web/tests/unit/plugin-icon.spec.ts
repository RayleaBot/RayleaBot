import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import PluginIcon from '@/components/plugins/PluginIcon.vue'

describe('shared plugin icons', () => {
  it('retries changed resources, sources, and explicit refreshes after an image failure', async () => {
    const wrapper = mount(PluginIcon, { props: { pluginId: 'echo', remoteUrl: 'https://example.test/old.svg', sourceKey: 'official' } })
    await wrapper.get('.plugin-icon > img').trigger('error')
    expect(wrapper.find('.plugin-icon > img').exists()).toBe(false)
    expect(wrapper.find('.raylea-mark').exists()).toBe(true)
    await wrapper.setProps({ remoteUrl: 'https://example.test/new.svg' })
    expect(wrapper.get('.plugin-icon > img').attributes('src')).toBe('https://example.test/new.svg')
    await wrapper.get('.plugin-icon > img').trigger('error')
    await wrapper.setProps({ sourceKey: 'custom' })
    expect(wrapper.find('.plugin-icon > img').exists()).toBe(true)
    await wrapper.get('.plugin-icon > img').trigger('error')
    await wrapper.setProps({ refreshKey: 1 })
    expect(wrapper.find('.plugin-icon > img').exists()).toBe(true)
  })

  it('loads only the protected icon URL and restores the logo after an image error', async () => {
    const wrapper = mount(PluginIcon, { props: { pluginId: 'weather', icon: 'assets/weather.svg', version: '1.0.0' } })
    expect(wrapper.get('.plugin-icon > img').attributes('src')).toBe('/api/plugins/weather/icon')
    await wrapper.get('.plugin-icon > img').trigger('error')
    expect(wrapper.find('.plugin-icon > img').exists()).toBe(false)
    expect(wrapper.find('.raylea-mark').exists()).toBe(true)
    await wrapper.setProps({ icon: 'assets/replacement.svg' })
    expect(wrapper.find('.plugin-icon > img').exists()).toBe(true)
    await wrapper.setProps({ icon: undefined })
    expect(wrapper.find('.plugin-icon > img').exists()).toBe(false)
    expect(wrapper.find('.raylea-mark').exists()).toBe(true)
  })
})
