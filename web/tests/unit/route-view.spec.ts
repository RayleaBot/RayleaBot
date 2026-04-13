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
      logs: 0,
      protocols: 0,
    }

    const ProtocolsView = defineComponent({
      name: 'ProtocolsView',
      setup() {
        onMounted(() => {
          mountCounts.protocols += 1
        })

        return () => h('div', '协议中心')
      },
    })

    const ProtocolLogsView = defineComponent({
      name: 'ProtocolLogsView',
      setup() {
        onMounted(() => {
          mountCounts.logs += 1
        })

        return () => h('div', '协议日志')
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
              path: 'protocols',
              component: ProtocolsView,
            },
            {
              path: 'protocols/logs',
              component: ProtocolLogsView,
            },
          ],
        },
      ],
    })

    await router.push('/protocols')
    await router.isReady()

    mount(RouteView, {
      attachTo: document.body,
      global: {
        plugins: [router],
      },
    })

    await flushPromises()
    expect(mountCounts.protocols).toBe(1)
    expect(document.body.textContent).toContain('协议中心')

    await router.push('/protocols/logs')
    await flushPromises()
    expect(mountCounts.logs).toBe(1)
    expect(document.body.textContent).toContain('协议日志')

    await router.push('/protocols')
    await flushPromises()
    expect(mountCounts.protocols).toBe(2)
    expect(document.body.textContent).toContain('协议中心')
  })
})
