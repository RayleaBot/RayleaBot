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

  it.each(['detail', 'form'] as const)(
    'applies the %s width limit only when fixed content width is selected',
    async (width) => {
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
      const store = useUiShellStore()

      expect(wrapper.classes()).not.toContain('app-page--fixed-width')
      expect(wrapper.classes()).not.toContain(`app-page--${width}`)

      store.patchPreferences({ contentWidth: 'fixed' })
      await nextTick()

      expect(wrapper.classes()).toContain('app-page--fixed-width')
      expect(wrapper.classes()).toContain(`app-page--${width}`)

      store.patchPreferences({ contentWidth: 'wide' })
      await nextTick()

      expect(wrapper.classes()).not.toContain('app-page--fixed-width')
      expect(wrapper.classes()).not.toContain(`app-page--${width}`)
    },
  )
})
