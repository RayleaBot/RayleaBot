import { createPinia, getActivePinia, setActivePinia } from 'pinia'
import { flushPromises, mount } from '@vue/test-utils'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import NativeTemplatePreviewFrame, { calculateNativePreviewLayout } from '@/components/NativeTemplatePreviewFrame.vue'
import MenuCenterView from '@/views/builtin/MenuCenterView.vue'
import { useConfigStore } from '@/stores/config'
import { usePluginsStore } from '@/stores/plugins'
import type { ConfigDocument, PluginSummary } from '@/types/api'

vi.mock('@/adapter/feedback', () => ({ notifySuccess: vi.fn(), useToastFeedback: vi.fn() }))

function config(): ConfigDocument {
  return {
    schema_version: '2', command: { prefixes: ['/', '*'] },
    builtin_features: { menu: { commands: ['help', '帮助'], prefixes: [] } },
    permission: { default_level: 'everyone' },
  } as ConfigDocument
}

function plugin(): PluginSummary {
  return {
    id: 'subscription-hub', name: '订阅与解析', version: '1.0.0', role: 'community', state: 'running',
    commands: [
      {
        id: 'status', name: '订阅状态', effective_names: ['订阅状态'], description: '查看订阅状态', usage: '/订阅状态',
        permission: 'everyone', trigger: { type: 'exact', names: ['订阅状态'] },
      },
      {
        id: 'resolver-help', name: '解析帮助', effective_names: ['解析帮助'], description: '查看解析帮助', usage: '/解析帮助',
        permission: 'everyone', trigger: { type: 'exact', names: ['解析帮助'] },
      },
      {
        id: 'guide', name: '角色攻略', effective_names: [], description: '查询角色攻略', usage: '*<角色名>攻略',
        permission: 'everyone', trigger: { type: 'pattern', pattern: '^(.+)攻略$' },
      },
    ],
    command_groups: [
      { id: 'subscription', title: '订阅操作', commands: ['status'] },
      { id: 'resolver', title: '解析操作', commands: ['resolver-help', 'guide'] },
    ],
    help: { title: '订阅与解析', summary: '订阅推送与链接解析' },
    command_conflicts: [],
  }
}

async function mountPage(item: PluginSummary = plugin()) {
  const configStore = useConfigStore()
  const pluginsStore = usePluginsStore()
  configStore.document = config()
  pluginsStore.items = [item]
  vi.spyOn(configStore, 'fetchConfig').mockResolvedValue(undefined)
  vi.spyOn(pluginsStore, 'fetchList').mockResolvedValue(undefined)
  const wrapper = mount(MenuCenterView, { global: { plugins: [getActivePinia()!] } })
  await flushPromises()
  return wrapper
}

function pluginPreviewData(wrapper: Awaited<ReturnType<typeof mountPage>>) {
  const previews = wrapper.findAllComponents(NativeTemplatePreviewFrame)
  return previews[1].props('data') as { groups: Array<{ title: string, items: Array<Record<string, unknown>> }> }
}

function rootPreviewData(wrapper: Awaited<ReturnType<typeof mountPage>>) {
  const previews = wrapper.findAllComponents(NativeTemplatePreviewFrame)
  return previews[0].props('data') as Record<string, unknown>
}

describe('MenuCenterView', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    vi.restoreAllMocks()
  })

  it('builds menu groups only from real command ids', async () => {
    const data = pluginPreviewData(await mountPage())
    expect(data.groups.map((group) => group.title)).toEqual(['订阅操作', '解析操作'])
    expect(data.groups.flatMap((group) => group.items).map((item) => item.name))
      .toEqual(['订阅状态', '解析帮助', '角色攻略'])
    expect(JSON.stringify(data)).not.toContain('无对应指令')
  })

  it('projects unified trigger types into native preview data', async () => {
    const data = pluginPreviewData(await mountPage())
    const items = data.groups.flatMap((group) => group.items)
    expect(items[0]).toMatchObject({ trigger_type: 'exact', command_prefixes: ['/', '*'] })
    expect(items[2]).toMatchObject({
      trigger_type: 'pattern', usage: '<角色名>攻略',
      usage_parts: [{ kind: 'required', text: '角色名' }, { kind: 'literal', text: '攻略' }],
    })
    expect(items[0]).not.toHaveProperty('command_source')
  })

  it('projects exact, setting and pattern commands without legacy command fields', async () => {
    const item = plugin()
    item.commands.push({
      id: 'resolver-toggle',
      name: '解析开关',
      effective_names: ['解析开关'],
      description: '调整解析开关',
      usage: '/解析开关 <平台>',
      permission: 'super_admin',
      trigger: { type: 'setting', settings_key: 'resolver.enabled' },
    })
    item.command_groups[1].commands.push('resolver-toggle')

    const data = pluginPreviewData(await mountPage(item))
    const items = data.groups.flatMap((group) => group.items)
    expect(items.find((command) => command.name === '订阅状态')).toMatchObject({ trigger_type: 'exact' })
    expect(items.find((command) => command.name === '解析开关')).toMatchObject({
      trigger_type: 'setting',
      usage_args: '<平台>',
      usage_parts: [{ kind: 'required', text: '平台' }],
    })
    expect(items.find((command) => command.name === '角色攻略')).toMatchObject({
      trigger_type: 'pattern',
      usage: '<角色名>攻略',
    })
    expect(JSON.stringify(items)).not.toContain('command_source')
  })

  it('updates the root preview and saves only the builtin menu draft', async () => {
    const wrapper = await mountPage()
    const configStore = useConfigStore()
    const saveSpy = vi.spyOn(configStore, 'saveConfig').mockImplementation(async (nextConfig) => ({
      config: nextConfig,
      apply_effects: { applied_now: [], reloaded_now: [], restart_required_fields: [] },
      redacted_fields: [],
      restart_required: false,
    }))

    expect(rootPreviewData(wrapper)).toMatchObject({
      command_prefixes: ['/', '*'],
      items: [expect.objectContaining({ name: '订阅与解析' })],
    })

    await wrapper.getComponent('[data-testid="menu-center-commands"]').vm.$emit('update:modelValue', ['menu', '菜单'])
    await wrapper.getComponent('[data-testid="menu-center-prefixes"]').vm.$emit('update:modelValue', ['#', '*'])
    await flushPromises()
    expect(rootPreviewData(wrapper)).toMatchObject({ command_prefixes: ['#', '*'] })
    expect(wrapper.get('[data-testid="menu-center-save"]').attributes('disabled')).toBeUndefined()

    await wrapper.get('[data-testid="menu-center-save"]').trigger('click')
    await flushPromises()
    expect(saveSpy).toHaveBeenCalledTimes(1)
    expect(saveSpy.mock.calls[0][0]).toMatchObject({
      builtin_features: { menu: { commands: ['menu', '菜单'], prefixes: ['#', '*'] } },
    })
  })

  it('keeps native preview scaling bounded', () => {
    expect(calculateNativePreviewLayout({ containerWidth: 1180, contentHeight: 760, viewportHeight: 1280, containerTop: 120 }))
      .toMatchObject({ frameWidth: 960, scale: 1 })
    expect(calculateNativePreviewLayout({ containerWidth: 480, contentHeight: 760, viewportHeight: 1280, containerTop: 120 }).scale).toBeLessThan(1)
  })
})
