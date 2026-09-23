import { computed, defineComponent } from 'vue'
import { createPinia, disposePinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { createMemoryHistory, createRouter } from 'vue-router'

import { notifySuccess } from '@/adapter/feedback'
import { useConfigStore } from '@/stores/config'
import { usePluginConsoleStore } from '@/stores/plugin-console'
import { usePluginsStore } from '@/stores/plugins'
import { useSocketStore } from '@/stores/sockets'
import { usePluginDetail } from '@/views/plugins/usePluginDetail'
import type { PluginDetail, TaskAcceptedResponse } from '@/types/api'

vi.mock('@/adapter/feedback', () => ({ notifySuccess: vi.fn(), useToastFeedback: vi.fn() }))

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (error: unknown) => void
  const promise = new Promise<T>((res, rej) => { resolve = res; reject = rej })
  return { promise, resolve, reject }
}

function plugin(id: string): PluginDetail {
  return { id, name: id, role: 'community', state: 'disabled', commands: [], command_groups: [], help: {}, webhooks: [] }
}

describe('plugin detail lifecycle', () => {
  let pinia: ReturnType<typeof createPinia>
  beforeEach(() => {
    pinia = createPinia()
    setActivePinia(pinia)
    vi.mocked(notifySuccess).mockClear()
    vi.spyOn(useConfigStore(), 'fetchConfig').mockResolvedValue(undefined)
    vi.spyOn(usePluginConsoleStore(), 'fetchOutboundConsoleHistory').mockResolvedValue([])
    vi.spyOn(useSocketStore(), 'setConsolePlugin').mockImplementation(() => undefined)
    vi.spyOn(usePluginsStore(), 'fetchDetail').mockImplementation(async (id) => {
      const detail = plugin(id)
      usePluginsStore().current = detail
      return detail
    })
  })
  afterEach(() => { disposePinia(pinia) })

  async function setup() {
    const router = createRouter({
      history: createMemoryHistory(),
      routes: [
        { path: '/plugins', component: { template: '<div />' } },
        { path: '/plugins/:id', component: { template: '<div />' } },
      ],
    })
    await router.push('/plugins/first')
    const id = computed(() => String(router.currentRoute.value.params.id))
    let state!: ReturnType<typeof usePluginDetail>
    const wrapper = mount(defineComponent({
      setup() { state = usePluginDetail(id); return () => null },
    }), { global: { plugins: [pinia, router] } })
    await flushPromises()
    return { router, state, wrapper }
  }

  it('keeps the selected plugin page when an earlier uninstall completes', async () => {
    const { state, router } = await setup()
    const pending = deferred<TaskAcceptedResponse>()
    vi.spyOn(usePluginsStore(), 'uninstallPlugin').mockReturnValue(pending.promise)
    state.uninstallDialogVisible.value = true
    const operation = state.uninstallPlugin()
    await router.push('/plugins/second')
    await flushPromises()
    expect(state.uninstallDialogVisible.value).toBe(false)

    pending.resolve({ task_id: 'uninstall-first' } as TaskAcceptedResponse)
    await operation
    expect(router.currentRoute.value.path).toBe('/plugins/second')
    expect(notifySuccess).not.toHaveBeenCalled()
  })

  it('ignores an old operation error even after returning to the same plugin', async () => {
    const { state, router } = await setup()
    const pending = deferred<PluginDetail>()
    vi.spyOn(usePluginsStore(), 'executeAction').mockReturnValue(pending.promise)
    const operation = state.runAction('enable')
    await router.push('/plugins/second')
    await router.push('/plugins/first')
    await flushPromises()
    pending.reject(new Error('old action failure'))
    await operation
    expect(state.operationError.value).toBeNull()
  })

  it('preserves successful action feedback and uninstall navigation for the current plugin', async () => {
    const { state, router } = await setup()
    const store = usePluginsStore()
    vi.spyOn(store, 'executeAction').mockResolvedValue(plugin('first'))
    vi.spyOn(store, 'uninstallPlugin').mockImplementation(async (_id, onAccepted) => {
      onAccepted?.()
      return { task_id: 'uninstall-first' } as TaskAcceptedResponse
    })
    await state.runAction('enable')
    expect(store.executeAction).toHaveBeenCalledWith('first', 'enable')
    state.uninstallDialogVisible.value = true
    await state.uninstallPlugin()
    expect(state.uninstallDialogVisible.value).toBe(false)
    expect(router.currentRoute.value.path).toBe('/plugins')
    expect(notifySuccess).toHaveBeenCalledTimes(2)
  })

  it('does not enable a different plugin using stale detail state', async () => {
    const { state } = await setup()
    const store = usePluginsStore()
    const action = vi.spyOn(store, 'executeAction')
    store.current = plugin('another')
    expect(state.currentPlugin.value).toBeNull()
    await state.runAction('enable')
    expect(action).not.toHaveBeenCalled()
  })

  it('ignores completion after the page is unmounted', async () => {
    const { state, wrapper } = await setup()
    const pending = deferred<PluginDetail>()
    vi.spyOn(usePluginsStore(), 'executeAction').mockReturnValue(pending.promise)
    const operation = state.runAction('enable')
    wrapper.unmount()
    pending.resolve(plugin('first'))
    await operation
    expect(notifySuccess).not.toHaveBeenCalled()
  })
})
