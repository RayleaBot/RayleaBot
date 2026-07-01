import { defineComponent, h, onMounted } from 'vue'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import RouteView from '@/layouts/RouteView.vue'

describe('RouteView', () => {
  beforeEach(() => {
    document.body.innerHTML = ''
  })

  it('does not keep an extra nested cache layer for grouped pages', async () => {
    const mountCounts = {
      currentLogs: 0,
      historyLogs: 0,
    }

    const LogsView = defineComponent({
      name: 'LogsView',
      setup() {
        onMounted(() => {
          mountCounts.currentLogs += 1
        })

        return () => h('div', '实时日志')
      },
    })

    const LogsHistoryView = defineComponent({
      name: 'LogsHistoryView',
      setup() {
        onMounted(() => {
          mountCounts.historyLogs += 1
        })

        return () => h('div', '历史日志')
      },
    })

    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        {
          path: '/',
          component: RouteView,
          children: [
            {
              path: 'logs',
              component: LogsView,
            },
            {
              path: 'logs/history',
              component: LogsHistoryView,
            },
          ],
        },
      ],
    })

    await router.push('/logs')
    await router.isReady()

    mount(RouteView, {
      attachTo: document.body,
      global: {
        plugins: [router],
      },
    })

    await flushPromises()
    expect(mountCounts.currentLogs).toBe(1)

    await router.push('/logs/history')
    await flushPromises()
    expect(mountCounts.historyLogs).toBe(1)

    await router.push('/logs')
    await flushPromises()
    expect(mountCounts.currentLogs).toBe(2)
  })
})
