import { flushPromises, mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { afterEach, describe, expect, it } from 'vitest'

import PreferencesDrawer from '@/components/shell/PreferencesDrawer.vue'
import { useUiShellStore } from '@/stores/ui-shell'

const mounted: Array<ReturnType<typeof mount>> = []

afterEach(() => {
  mounted.splice(0).forEach(wrapper => wrapper.unmount())
})

describe('preferences drawer', () => {
  // The HarmonyOS Sans agreement requires a notice in the software that the fonts are used.
  it('states the bundled HarmonyOS Sans font', async () => {
    const pinia = createPinia()
    setActivePinia(pinia)
    useUiShellStore().openSettings()
    mounted.push(mount(PreferencesDrawer, { attachTo: document.body, global: { plugins: [pinia] } }))
    await flushPromises()

    const notice = document.querySelector('[data-testid=preferences-font-notice]')
    expect(notice?.textContent).toContain('HarmonyOS Sans SC')
  })
})
