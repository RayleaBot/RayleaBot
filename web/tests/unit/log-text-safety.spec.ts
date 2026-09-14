import { createPinia } from 'pinia'
import { mount } from '@vue/test-utils'
import { expect, it } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'
import ManagementLogDetailContent from '@/components/logs/ManagementLogDetailContent.vue'
import ManagementLogRow from '@/components/logs/ManagementLogRow.vue'
import type { LogSummary } from '@/types/api'

const unsafeMessage = '<img src=x onerror="window.__logXss=1"><script>window.__logXss=1</script>'

it('renders OneBot message markup in log rows and details as inert text', () => {
  const summary = { timestamp: '2026-09-14T09:00:00Z', level: 'info', source: 'onebot11', message: unsafeMessage } as LogSummary
  const router = createRouter({ history: createMemoryHistory(), routes: [{ path: '/:pathMatch(.*)*', component: { template: '<div />' } }] })
  const global = { plugins: [createPinia(), router] }
  for (const wrapper of [
    mount(ManagementLogRow, { props: { item: summary, selected: false }, global }),
    mount(ManagementLogDetailContent, { props: { loading: false, error: null, summary, detail: null, scope: 'current_session' }, global }),
  ]) {
    expect(wrapper.find('img').exists()).toBe(false)
    expect(wrapper.find('script').exists()).toBe(false)
    expect(wrapper.text()).toContain(unsafeMessage)
    wrapper.unmount()
  }
})
