import { mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { computed, defineComponent, h, ref } from 'vue'

import type { ConnectionStatus } from '@/types/api'
import {
  shouldUseDashboardHttpAutoRefresh,
  useDashboardRefresh,
} from '@/views/dashboard/useDashboardRefresh'

describe('dashboard refresh', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('uses http auto refresh only when the events socket is not healthy', async () => {
    const socketStatus = ref<ConnectionStatus>('authenticated')
    const refreshAll = vi.fn().mockResolvedValue(undefined)
    const refreshProtocols = vi.fn().mockResolvedValue(undefined)

    const Harness = defineComponent({
      setup(_, { expose }) {
        const state = useDashboardRefresh({
          eventsSocketStatus: computed(() => socketStatus.value),
          protocolsStore: {
            refresh: refreshProtocols,
          },
          recoveryConfirmNote: ref(''),
          recoverySummary: computed(() => null),
          selectedRecoveryReviewIds: ref<string[]>([]),
          systemStore: {
            refreshAll,
          },
        })

        expose(state)
        return () => h('div')
      },
    })

    const wrapper = mount(Harness)
    await Promise.resolve()
    expect(refreshAll).toHaveBeenCalledTimes(1)
    expect(refreshProtocols).toHaveBeenCalledTimes(1)

    refreshAll.mockClear()
    refreshProtocols.mockClear()

    ;(wrapper.vm as { toggleAutoRefresh: (value: boolean) => void }).toggleAutoRefresh(true)
    await Promise.resolve()
    refreshAll.mockClear()
    refreshProtocols.mockClear()

    await vi.advanceTimersByTimeAsync(10_000)
    expect(refreshAll).not.toHaveBeenCalled()
    expect(refreshProtocols).not.toHaveBeenCalled()

    socketStatus.value = 'disconnected'
    await vi.advanceTimersByTimeAsync(10_000)
    expect(refreshAll).toHaveBeenCalledTimes(1)
    expect(refreshProtocols).toHaveBeenCalledTimes(1)

    wrapper.unmount()
  })

  it('treats authenticated and connected sockets as healthy auto-refresh sources', () => {
    expect(shouldUseDashboardHttpAutoRefresh('authenticated')).toBe(false)
    expect(shouldUseDashboardHttpAutoRefresh('connected')).toBe(false)
    expect(shouldUseDashboardHttpAutoRefresh('disconnected')).toBe(true)
    expect(shouldUseDashboardHttpAutoRefresh('reconnecting')).toBe(true)
  })
})
