import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it } from 'vitest'

import PluginManagementUIHost from '@/components/plugins/PluginManagementUIHost.vue'
import type { PluginDetail } from '@/types/api'

const page = { id: 'config', label: '配置页面' }

function buildPlugin(overrides: Record<string, unknown> = {}): PluginDetail {
  return {
    id: 'example-config-panel',
    name: 'Example Config Panel',
    role: 'community',
    state: 'disabled',
    version: '0.2.0',
    description: 'Go example plugin with a Vue management page.',
    source: {
      root: 'examples/plugins',
      package_source_type: 'local_zip',
      package_source_ref: 'examples/plugins/example-config-panel.zip',
      verified: true,
    },
    trust: { level: 'third_party' },
    management_ui: { entry: 'ui/index.html', pages: [page] },
    commands: [],
    command_groups: [],
    help: {},
    command_conflicts: [],
    webhooks: [],
    ...overrides,
  } as unknown as PluginDetail
}

function mountHost(plugin = buildPlugin()) {
  return mount(PluginManagementUIHost, { props: { plugin, title: '配置页面', page } })
}

function frameSource(wrapper: ReturnType<typeof mountHost>) {
  return wrapper.get('[data-testid="plugin-management-ui-frame"]').attributes('src') ?? ''
}

describe('PluginManagementUIHost', () => {
  beforeEach(() => {
    localStorage.clear()
  })

  it('loads the plugin page from the same-origin plugin UI path', async () => {
    const wrapper = mountHost()
    await flushPromises()

    const source = new URL(frameSource(wrapper), 'http://127.0.0.1:8080')
    expect(source.pathname).toBe('/plugin-ui/example-config-panel/index.html')
    expect(source.searchParams.get('page')).toBe('config')
    wrapper.unmount()
  })

  it('requires an explicit confirmation before loading an unverified plugin page', async () => {
    const wrapper = mountHost(buildPlugin({ trust: { level: 'unverified' } }))
    await flushPromises()
    expect(wrapper.find('[data-testid="plugin-management-ui-frame"]').exists()).toBe(false)

    await wrapper.get('[data-testid="plugin-management-ui-confirm"] button').trigger('click')
    await flushPromises()
    expect(wrapper.find('[data-testid="plugin-management-ui-frame"]').exists()).toBe(true)
    wrapper.unmount()
  })

  it('restarts the plugin page after a runtime reload reaches running', async () => {
    const wrapper = mountHost(buildPlugin({ state: 'running' }))
    await flushPromises()
    const first = frameSource(wrapper)

    await wrapper.setProps({ plugin: buildPlugin({ state: 'starting' }) })
    await wrapper.setProps({ plugin: buildPlugin({ state: 'running' }) })
    await flushPromises()
    expect(frameSource(wrapper)).not.toBe(first)
    wrapper.unmount()
  })
})
