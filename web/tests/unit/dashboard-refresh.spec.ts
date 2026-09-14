import { mount } from '@vue/test-utils'
import { describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'

import { useDashboardRefresh } from '@/views/dashboard/useDashboardRefresh'

describe('dashboard state sync', () => {
  it('loads the dashboard snapshots when the page mounts', async () => {
    const refreshAll = vi.fn().mockResolvedValue(undefined)
    const refreshAdapters = vi.fn().mockResolvedValue(undefined)

    const Harness = defineComponent({
      setup() {
        useDashboardRefresh({
          adaptersStore: {
            refresh: refreshAdapters,
          },
          systemStore: {
            refreshAll,
          },
        })

        return () => h('div')
      },
    })

    mount(Harness)
    await Promise.resolve()

    expect(refreshAll).toHaveBeenCalledTimes(1)
    expect(refreshAdapters).toHaveBeenCalledTimes(1)
  })
})
