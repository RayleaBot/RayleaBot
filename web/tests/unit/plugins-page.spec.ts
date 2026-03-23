import ElementPlus from 'element-plus'
import { createPinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { createRouter, createMemoryHistory } from 'vue-router'

import PluginsPage from '@/pages/PluginsPage.vue'
import { usePluginsStore } from '@/stores/plugins'

describe('PluginsPage', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('calls enable action when the enable button is pressed', async () => {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [{ path: '/', component: { template: '<div />' } }],
    })
    const store = usePluginsStore()
    store.items = [{
      id: 'weather',
      registration_state: 'installed',
      desired_state: 'disabled',
      runtime_state: 'stopped',
      display_state: 'disabled',
    }]

    vi.spyOn(store, 'fetchList').mockResolvedValue(undefined)
    const executeSpy = vi.spyOn(store, 'executeAction').mockResolvedValue(store.items[0])

    const wrapper = mount(PluginsPage, {
      global: {
        plugins: [ElementPlus, router],
      },
    })

    await flushPromises()
    const button = wrapper.findAll('button').find((candidate) => candidate.text().includes('Enable'))
    expect(button).toBeTruthy()
    await button!.trigger('click')

    expect(executeSpy).toHaveBeenCalledWith('weather', 'enable')
  })
})
