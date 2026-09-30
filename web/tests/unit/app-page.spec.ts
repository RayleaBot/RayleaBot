import { createPinia, setActivePinia } from 'pinia'
import { mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it } from 'vitest'
import { nextTick } from 'vue'

import AppPage from '@/components/page/AppPage.vue'
import { useUiShellStore } from '@/stores/ui-shell'

describe('AppPage content width', () => {
  beforeEach(() => {
    window.localStorage.clear()
  })

  function mountPage(width: 'detail' | 'form') {
    const pinia = createPinia()
    setActivePinia(pinia)

    const wrapper = mount(AppPage, {
      global: {
        plugins: [pinia],
      },
      props: {
        title: '工作区',
        width,
      },
    })
    return { store: useUiShellStore(), wrapper }
  }

  it('applies the detail width limit only when fixed content width is selected', async () => {
    const { store, wrapper } = mountPage('detail')

    expect(wrapper.classes()).not.toContain('app-page--fixed-width')
    expect(wrapper.classes()).not.toContain('app-page--detail')

    store.patchPreferences({ contentWidth: 'fixed' })
    await nextTick()

    expect(wrapper.classes()).toContain('app-page--fixed-width')
    expect(wrapper.classes()).toContain('app-page--detail')

    store.patchPreferences({ contentWidth: 'wide' })
    await nextTick()

    expect(wrapper.classes()).not.toContain('app-page--fixed-width')
    expect(wrapper.classes()).not.toContain('app-page--detail')
  })

  it('keeps the form width limit under both content width preferences', async () => {
    const { store, wrapper } = mountPage('form')

    expect(wrapper.classes()).toContain('app-page--form')

    store.patchPreferences({ contentWidth: 'fixed' })
    await nextTick()

    expect(wrapper.classes()).toContain('app-page--form')
  })
})
