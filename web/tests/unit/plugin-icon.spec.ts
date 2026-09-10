import { mount } from '@vue/test-utils'
import { describe, expect, it } from 'vitest'
import PluginIcon from '@/components/plugins/PluginIcon.vue'

describe('shared plugin icons', () => {
  it('retries changed resources, sources, and explicit refreshes after an image failure', async () => {
    const wrapper = mount(PluginIcon, { props: { pluginId: 'echo', remoteUrl: 'https://example.test/old.svg', sourceKey: 'official' } })
    await wrapper.get('img').trigger('error')
    expect(wrapper.find('img').exists()).toBe(false)
    expect(wrapper.find('.raylea-mark').exists()).toBe(true)
    await wrapper.setProps({ remoteUrl: 'https://example.test/new.svg' })
    expect(wrapper.get('img').attributes('src')).toBe('https://example.test/new.svg')
    await wrapper.get('img').trigger('error')
    await wrapper.setProps({ sourceKey: 'custom' })
    expect(wrapper.find('img').exists()).toBe(true)
    await wrapper.get('img').trigger('error')
    await wrapper.setProps({ refreshKey: 1 })
    expect(wrapper.find('img').exists()).toBe(true)
  })
})
